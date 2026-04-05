package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
)

// OpenAICompat runs prompts through any OpenAI-compatible API endpoint.
// Manages conversation history in-memory for session continuity.
type OpenAICompat struct {
	BaseURL string
	APIKey  string
	Model   string

	mu       sync.Mutex
	sessions map[string][]chatMessage
	nextID   atomic.Int64
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func NewOpenAICompat(baseURL, apiKey, model string) *OpenAICompat {
	return &OpenAICompat{
		BaseURL:  strings.TrimRight(baseURL, "/"),
		APIKey:   apiKey,
		Model:    model,
		sessions: make(map[string][]chatMessage),
	}
}

func (o *OpenAICompat) Run(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	messages := []chatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}
	return o.call(ctx, messages)
}

func (o *OpenAICompat) RunSession(ctx context.Context, systemPrompt, userPrompt string) (string, string, error) {
	messages := []chatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	resp, err := o.call(ctx, messages)
	if err != nil {
		return "", "", err
	}

	// Store the conversation for resumption
	messages = append(messages, chatMessage{Role: "assistant", Content: resp})
	sessionID := fmt.Sprintf("oai-%d", o.nextID.Add(1))

	o.mu.Lock()
	o.sessions[sessionID] = messages
	o.mu.Unlock()

	return resp, sessionID, nil
}

func (o *OpenAICompat) Resume(ctx context.Context, sessionID string, userPrompt string) (string, error) {
	o.mu.Lock()
	messages, ok := o.sessions[sessionID]
	if !ok {
		o.mu.Unlock()
		// No session found — run as a fresh call with just the user prompt
		return o.Run(ctx, "", userPrompt)
	}
	// Append the new user message
	messages = append(messages, chatMessage{Role: "user", Content: userPrompt})
	o.mu.Unlock()

	resp, err := o.call(ctx, messages)
	if err != nil {
		return "", err
	}

	// Update session with the new exchange
	o.mu.Lock()
	o.sessions[sessionID] = append(messages, chatMessage{Role: "assistant", Content: resp})
	o.mu.Unlock()

	return resp, nil
}

func (o *OpenAICompat) call(ctx context.Context, messages []chatMessage) (string, error) {
	reqBody := chatRequest{
		Model:    o.Model,
		Messages: messages,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	url := o.BaseURL + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if o.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+o.APIKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("api call: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("api error (status %d): %s", resp.StatusCode, string(body))
	}

	var chatResp chatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return "", fmt.Errorf("parse response: %w\nraw: %s", err, string(body))
	}

	if chatResp.Error != nil {
		return "", fmt.Errorf("api error: %s", chatResp.Error.Message)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("api returned no choices")
	}

	text := strings.TrimSpace(chatResp.Choices[0].Message.Content)
	if text == "" {
		return "", fmt.Errorf("api returned empty response")
	}
	return text, nil
}
