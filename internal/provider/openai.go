package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// OpenAI Models
// Full list: https://platform.openai.com/docs/models
//
// Frontier (GPT-5):
//   - gpt-5.2              : Best model for coding and agentic tasks
//   - gpt-5.2-pro          : Smarter, more precise responses
//   - gpt-5                : Previous intelligent reasoning model
//   - gpt-5-mini           : Faster, cost-efficient for well-defined tasks
//   - gpt-5-nano           : Fastest, most cost-efficient GPT-5
//
// GPT-4.1:
//   - gpt-4.1              : Smartest non-reasoning model
//   - gpt-4.1-mini         : Smaller, faster version of GPT-4.1
//   - gpt-4.1-nano         : Smallest GPT-4.1 variant
//
// Reasoning (o-series):
//   - o3                   : Reasoning model for complex tasks
//   - o3-pro               : More compute for better responses
//   - o4-mini              : Fast, cost-efficient reasoning
//
// Previous:
//   - gpt-4o               : Fast, intelligent, flexible GPT model
//   - gpt-4o-mini          : Fast, affordable for focused tasks

// OpenAI implements Provider for OpenAI's API.
type OpenAI struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// OpenAIOption configures an OpenAI provider.
type OpenAIOption func(*OpenAI)

// WithOpenAIBaseURL sets a custom base URL (useful for proxies or compatible APIs).
func WithOpenAIBaseURL(url string) OpenAIOption {
	return func(o *OpenAI) { o.baseURL = url }
}

// WithOpenAIHTTPClient sets a custom HTTP client.
func WithOpenAIHTTPClient(c *http.Client) OpenAIOption {
	return func(o *OpenAI) { o.httpClient = c }
}

// NewOpenAI creates an OpenAI provider.
// Reads API key from OPENAI_API_KEY environment variable.
func NewOpenAI(opts ...OpenAIOption) (*OpenAI, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, errors.New("OPENAI_API_KEY environment variable required")
	}

	o := &OpenAI{
		apiKey:     apiKey,
		baseURL:    "https://api.openai.com/v1",
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}

	for _, opt := range opts {
		opt(o)
	}

	return o, nil
}

// Query sends a prompt to an OpenAI model and returns the response.
func (o *OpenAI) Query(ctx context.Context, req Request) (Response, error) {
	start := time.Now()

	payload := openAIRequest{
		Model: req.Model,
		Messages: []openAIMessage{
			{Role: "user", Content: req.Prompt},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return Response{}, fmt.Errorf("marshaling request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return Response{}, fmt.Errorf("creating request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+o.apiKey)

	resp, err := o.httpClient.Do(httpReq)
	if err != nil {
		return Response{}, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return Response{}, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return Response{}, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var openAIResp openAIResponse
	if err := json.Unmarshal(respBody, &openAIResp); err != nil {
		return Response{}, fmt.Errorf("parsing response: %w", err)
	}

	if len(openAIResp.Choices) == 0 {
		return Response{}, errors.New("no choices in response")
	}

	return Response{
		Model:    req.Model,
		Content:  openAIResp.Choices[0].Message.Content,
		Provider: "openai",
		Latency:  time.Since(start),
	}, nil
}

type openAIRequest struct {
	Model    string          `json:"model"`
	Messages []openAIMessage `json:"messages"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}
