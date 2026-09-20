package main

import (
	"context"
	"errors"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

func testJobs(t *testing.T) *Jobs {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	js := NewJobs(ctx, t.TempDir())
	js.beatEvery = 10 * time.Millisecond
	return js
}

// The guarantee the whole design rests on: when the tool call that started the
// work goes away, the work does not. This is what made full-manuscript reviews
// unrecoverable before jobs existed.
func TestJobSurvivesCallerCancellation(t *testing.T) {
	js := testJobs(t)

	released := make(chan struct{})
	job := js.Start("codex", func(ctx context.Context) (string, string, error) {
		<-released
		return "the review", "sess-1", nil
	})

	callCtx, cancelCall := context.WithCancel(context.Background())
	cancelCall()

	if js.Wait(callCtx, job, 2*time.Second, nil) {
		t.Fatal("Wait reported completion for a cancelled call")
	}
	if got := job.Status().State; got != JobRunning {
		t.Fatalf("state after cancelled call = %q, want %q", got, JobRunning)
	}

	close(released)

	st := waitForState(t, job, JobDone)
	if st.Response != "the review" {
		t.Errorf("response = %q, want %q", st.Response, "the review")
	}
	if st.SessionID != "sess-1" {
		t.Errorf("session_id = %q, want %q", st.SessionID, "sess-1")
	}
}

func TestWaitReturnsInlineWhenWorkIsFast(t *testing.T) {
	js := testJobs(t)
	job := js.Start("pi", func(ctx context.Context) (string, string, error) {
		return "quick", "sess-2", nil
	})

	if !js.Wait(context.Background(), job, 2*time.Second, nil) {
		t.Fatal("fast job did not finish within budget")
	}
	if got := job.Status().State; got != JobDone {
		t.Fatalf("state = %q, want %q", got, JobDone)
	}
}

func TestWaitHandsBackHandleWhenBudgetExpires(t *testing.T) {
	js := testJobs(t)
	released := make(chan struct{})
	t.Cleanup(func() { close(released) })

	job := js.Start("codex", func(ctx context.Context) (string, string, error) {
		<-released
		return "late", "", nil
	})

	start := time.Now()
	if js.Wait(context.Background(), job, 50*time.Millisecond, nil) {
		t.Fatal("Wait reported completion before the job finished")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("Wait blocked %s, well past its budget", elapsed)
	}
	if _, ok := js.Get(job.ID()); !ok {
		t.Error("job is not collectable by id after the call returned")
	}
}

func TestZeroBudgetReturnsImmediately(t *testing.T) {
	js := testJobs(t)
	released := make(chan struct{})
	t.Cleanup(func() { close(released) })

	job := js.Start("pi", func(ctx context.Context) (string, string, error) {
		<-released
		return "", "", nil
	})

	start := time.Now()
	if js.Wait(context.Background(), job, 0, nil) {
		t.Fatal("zero budget reported completion")
	}
	if elapsed := time.Since(start); elapsed > 200*time.Millisecond {
		t.Fatalf("zero budget blocked for %s", elapsed)
	}
}

func TestHeartbeatFiresWhileWaiting(t *testing.T) {
	js := testJobs(t)
	released := make(chan struct{})
	t.Cleanup(func() { close(released) })

	job := js.Start("codex", func(ctx context.Context) (string, string, error) {
		<-released
		return "", "", nil
	})

	var beats atomic.Int64
	js.Wait(context.Background(), job, 120*time.Millisecond, func(time.Duration) {
		beats.Add(1)
	})

	if n := beats.Load(); n < 2 {
		t.Fatalf("heartbeat fired %d times over ~12 intervals, want at least 2", n)
	}
}

func TestHeartbeatStopsOnceJobFinishes(t *testing.T) {
	js := testJobs(t)
	job := js.Start("pi", func(ctx context.Context) (string, string, error) {
		time.Sleep(30 * time.Millisecond)
		return "done", "", nil
	})

	var beats atomic.Int64
	if !js.Wait(context.Background(), job, 2*time.Second, func(time.Duration) { beats.Add(1) }) {
		t.Fatal("job did not finish within budget")
	}
	before := beats.Load()
	time.Sleep(60 * time.Millisecond)
	if after := beats.Load(); after != before {
		t.Fatalf("heartbeat kept firing after completion: %d then %d", before, after)
	}
}

func TestResultIsMirroredToDisk(t *testing.T) {
	js := testJobs(t)
	job := js.Start("codex", func(ctx context.Context) (string, string, error) {
		return "a long review", "sess-3", nil
	})
	js.Wait(context.Background(), job, 2*time.Second, nil)

	st := job.Status()
	if st.Output == "" {
		t.Fatal("no output file recorded")
	}
	body, err := os.ReadFile(st.Output)
	if err != nil {
		t.Fatalf("read mirrored output: %v", err)
	}
	if string(body) != "a long review" {
		t.Errorf("mirrored output = %q, want %q", body, "a long review")
	}
}

func TestFailureRecordsErrorAndMirrorsIt(t *testing.T) {
	js := testJobs(t)
	job := js.Start("pi", func(ctx context.Context) (string, string, error) {
		return "", "", errors.New("pi cli: exit status 1")
	})
	js.Wait(context.Background(), job, 2*time.Second, nil)

	st := job.Status()
	if st.State != JobFailed {
		t.Fatalf("state = %q, want %q", st.State, JobFailed)
	}
	if st.Err == "" {
		t.Error("no error recorded")
	}
	if st.Output == "" {
		t.Error("failure was not mirrored to disk")
	}
}

func TestPruneKeepsRunningAndRecentJobs(t *testing.T) {
	js := testJobs(t)

	stale := js.Start("codex", func(ctx context.Context) (string, string, error) { return "old", "", nil })
	js.Wait(context.Background(), stale, time.Second, nil)

	fresh := js.Start("pi", func(ctx context.Context) (string, string, error) { return "new", "", nil })
	js.Wait(context.Background(), fresh, time.Second, nil)

	released := make(chan struct{})
	t.Cleanup(func() { close(released) })
	running := js.Start("claude", func(ctx context.Context) (string, string, error) {
		<-released
		return "", "", nil
	})

	// Age the first job past retention.
	stale.mu.Lock()
	stale.finished = time.Now().Add(-jobRetention - time.Minute)
	stale.mu.Unlock()

	js.prune()

	if _, ok := js.Get(stale.ID()); ok {
		t.Error("stale finished job was not pruned")
	}
	if _, ok := js.Get(fresh.ID()); !ok {
		t.Error("recent finished job was pruned")
	}
	if _, ok := js.Get(running.ID()); !ok {
		t.Error("running job was pruned")
	}
}

func waitForState(t *testing.T, job *Job, want JobState) JobStatus {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if st := job.Status(); st.State == want {
			return st
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("job never reached state %q (now %q)", want, job.Status().State)
	return JobStatus{}
}

func TestJobIDsDoNotCollideAcrossServerRuns(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	ids := map[string]bool{}
	for run := 0; run < 2; run++ {
		js := NewJobs(ctx, dir)
		job := js.Start("codex", func(ctx context.Context) (string, string, error) {
			return "review from run", "", nil
		})
		js.Wait(context.Background(), job, 2*time.Second, nil)

		st := job.Status()
		if ids[st.ID] {
			t.Fatalf("run %d reused job id %q; mirrored output would be overwritten", run, st.ID)
		}
		ids[st.ID] = true
		// Two runs a millisecond apart must not share a run token.
		time.Sleep(2 * time.Millisecond)
	}
}
