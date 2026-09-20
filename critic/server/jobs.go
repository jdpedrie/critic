package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// A full-manuscript review runs for minutes. An MCP tool call cannot stay open
// that long: clients cap tool calls, and the cap is shorter than the work. So
// an invocation runs as a Job on a context that outlives the tool call that
// started it, and the caller collects the result later by id.
//
// Short calls still answer inline. A tool call waits up to its budget before
// handing back a handle, so probes and quick questions behave exactly as they
// did when the handlers were synchronous.

// defaultWaitSeconds is how long a tool call blocks before returning a handle.
// Low enough to return before any plausible client timeout, high enough that
// ordinary calls never become two-step.
const defaultWaitSeconds = 60

// maxWaitSeconds caps what a caller may ask for. Past this the two-step path
// is the right answer, whatever the caller requested.
const maxWaitSeconds = 300

// heartbeatInterval is how often a waiting call emits notifications/progress.
// Clients that enforce an idle timeout reset it on each notification.
const heartbeatInterval = 10 * time.Second

// jobRetention is how long a finished job stays collectable.
const jobRetention = 6 * time.Hour

type JobState string

const (
	JobRunning JobState = "running"
	JobDone    JobState = "done"
	JobFailed  JobState = "failed"
)

// Job is one external-harness invocation. Its result is kept in memory and
// mirrored to disk, so a review survives the session that asked for it.
type Job struct {
	id      string
	kind    string
	started time.Time
	done    chan struct{}

	mu        sync.Mutex
	state     JobState
	finished  time.Time
	response  string
	sessionID string
	errMsg    string
	output    string
}

// JobStatus is a consistent read of a job.
type JobStatus struct {
	ID        string
	Kind      string
	State     JobState
	Started   time.Time
	Elapsed   time.Duration
	Response  string
	SessionID string
	Err       string
	Output    string
}

func (j *Job) ID() string { return j.id }

func (j *Job) Status() JobStatus {
	j.mu.Lock()
	defer j.mu.Unlock()
	elapsed := time.Since(j.started)
	if j.state != JobRunning {
		elapsed = j.finished.Sub(j.started)
	}
	return JobStatus{
		ID:        j.id,
		Kind:      j.kind,
		State:     j.state,
		Started:   j.started,
		Elapsed:   elapsed,
		Response:  j.response,
		SessionID: j.sessionID,
		Err:       j.errMsg,
		Output:    j.output,
	}
}

func (j *Job) finish(dir, response, sessionID string, err error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.finished = time.Now()
	body := response
	if err != nil {
		j.state = JobFailed
		j.errMsg = err.Error()
		body = err.Error()
	} else {
		j.state = JobDone
		j.response = response
		j.sessionID = sessionID
	}
	// Mirror to disk before returning. A result that never makes it back
	// through a tool call is still recoverable from here.
	if dir == "" || body == "" {
		return
	}
	path := filepath.Join(dir, j.id+".txt")
	if os.WriteFile(path, []byte(body), 0o600) == nil {
		j.output = path
	}
}

// Jobs owns every running invocation. Its base context is the server's, not
// any request's, which is the whole point: work must not die when the tool
// call that started it returns.
type Jobs struct {
	base context.Context
	dir  string

	// run distinguishes this server process from any other. Without it the
	// per-kind counter restarts at 1 on every launch and a new job overwrites
	// an older one's mirrored output.
	run string

	// beatEvery is the heartbeat period. A field rather than a constant so
	// tests can drive the waiting path without sleeping for real.
	beatEvery time.Duration

	mu   sync.Mutex
	m    map[string]*Job
	next atomic.Int64
}

func NewJobs(ctx context.Context, dir string) *Jobs {
	if dir != "" {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			dir = ""
		}
	}
	return &Jobs{
		base:      ctx,
		dir:       dir,
		run:       strconv.FormatInt(time.Now().UnixNano()/int64(time.Millisecond), 36),
		beatEvery: heartbeatInterval,
		m:         make(map[string]*Job),
	}
}

// Dir is where completed job output is mirrored. Empty if unavailable.
func (js *Jobs) Dir() string { return js.dir }

// Start launches fn and returns immediately. fn receives the manager's base
// context, so it survives the tool call that started it.
func (js *Jobs) Start(kind string, fn func(context.Context) (response, sessionID string, err error)) *Job {
	js.prune()

	job := &Job{
		id:      fmt.Sprintf("%s-%s-%d", kind, js.run, js.next.Add(1)),
		kind:    kind,
		started: time.Now(),
		done:    make(chan struct{}),
		state:   JobRunning,
	}

	js.mu.Lock()
	js.m[job.id] = job
	js.mu.Unlock()

	go func() {
		defer close(job.done)
		response, sessionID, err := fn(js.base)
		job.finish(js.dir, response, sessionID, err)
	}()

	return job
}

func (js *Jobs) Get(id string) (*Job, bool) {
	js.mu.Lock()
	defer js.mu.Unlock()
	job, ok := js.m[id]
	return job, ok
}

// List returns every known job, newest first.
func (js *Jobs) List() []JobStatus {
	js.mu.Lock()
	jobs := make([]*Job, 0, len(js.m))
	for _, job := range js.m {
		jobs = append(jobs, job)
	}
	js.mu.Unlock()

	out := make([]JobStatus, 0, len(jobs))
	for _, job := range jobs {
		out = append(out, job.Status())
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Started.After(out[b].Started) })
	return out
}

// Wait blocks until the job finishes, the budget expires, or the calling tool
// call is cancelled. It reports whether the job finished. A false return is
// not a failure: the job keeps running and stays collectable by id.
func (js *Jobs) Wait(ctx context.Context, job *Job, budget time.Duration, beat func(time.Duration)) bool {
	select {
	case <-job.done:
		return true
	default:
	}
	if budget <= 0 {
		return false
	}

	deadline := time.NewTimer(budget)
	defer deadline.Stop()
	ticker := time.NewTicker(js.beatEvery)
	defer ticker.Stop()

	for {
		select {
		case <-job.done:
			return true
		case <-ctx.Done():
			return false
		case <-deadline.C:
			return false
		case <-ticker.C:
			if beat != nil {
				beat(job.Status().Elapsed)
			}
		}
	}
}

// prune drops jobs that finished longer ago than jobRetention.
func (js *Jobs) prune() {
	js.mu.Lock()
	defer js.mu.Unlock()
	for id, job := range js.m {
		job.mu.Lock()
		expired := job.state != JobRunning && time.Since(job.finished) > jobRetention
		job.mu.Unlock()
		if expired {
			delete(js.m, id)
		}
	}
}

// heartbeatFor returns a progress emitter bound to this request's progress
// token. It returns nil when the client did not ask for progress: the MCP spec
// only permits notifications/progress against a token the client supplied, so
// staying quiet is correct, not a degraded mode.
func heartbeatFor(ctx context.Context, req mcp.CallToolRequest, label string) func(time.Duration) {
	if req.Params.Meta == nil || req.Params.Meta.ProgressToken == nil {
		return nil
	}
	token := req.Params.Meta.ProgressToken
	srv := server.ServerFromContext(ctx)
	if srv == nil {
		return nil
	}
	return func(elapsed time.Duration) {
		// Best effort. A dropped heartbeat must never fail the invocation.
		_ = srv.SendNotificationToClient(ctx, "notifications/progress", map[string]any{
			"progressToken": token,
			"progress":      elapsed.Seconds(),
			"message":       fmt.Sprintf("%s: %s elapsed", label, elapsed.Round(time.Second)),
		})
	}
}

// jobDir is where finished invocations are mirrored. A user cache directory
// beats a temp directory here: a review that took ten minutes to produce
// should not vanish on the next reboot or tmp sweep.
func jobDir() string {
	if base, err := os.UserCacheDir(); err == nil {
		return filepath.Join(base, "critic", "jobs")
	}
	return filepath.Join(os.TempDir(), "critic-jobs")
}
