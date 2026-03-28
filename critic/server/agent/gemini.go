package agent

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/genai"
)

// Gemini runs prompts through the Google GenAI API.
type Gemini struct {
	Model  string
	client *genai.Client
}

func NewGemini(model, apiKey string) (*Gemini, error) {
	cfg := &genai.ClientConfig{}
	if apiKey != "" {
		cfg.APIKey = apiKey
		cfg.Backend = genai.BackendGeminiAPI
	} else {
		// No API key — try Vertex AI backend which uses Application Default
		// Credentials (gcloud auth, service account, GOOGLE_APPLICATION_CREDENTIALS).
		cfg.Backend = genai.BackendVertexAI
	}

	client, err := genai.NewClient(context.Background(), cfg)
	if err != nil {
		if apiKey == "" {
			return nil, fmt.Errorf("no gemini credentials found — set an API key via /critic:settings gemini_api_key <key>, or set GEMINI_API_KEY, or configure gcloud ADC")
		}
		return nil, fmt.Errorf("create gemini client: %w", err)
	}

	return &Gemini{
		Model:  model,
		client: client,
	}, nil
}

func (g *Gemini) Run(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	config := &genai.GenerateContentConfig{
		SystemInstruction: genai.NewContentFromText(systemPrompt, genai.RoleUser),
	}

	result, err := g.client.Models.GenerateContent(ctx, g.Model, genai.Text(userPrompt), config)
	if err != nil {
		return "", fmt.Errorf("gemini generate: %w", err)
	}

	var text string
	for _, candidate := range result.Candidates {
		if candidate.Content != nil {
			for _, part := range candidate.Content.Parts {
				if part.Text != "" {
					text += part.Text
				}
			}
		}
	}

	text = strings.TrimSpace(text)
	if text == "" {
		return "", fmt.Errorf("gemini returned empty response")
	}
	return text, nil
}
