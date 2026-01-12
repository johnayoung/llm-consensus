package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Anthropic Claude Models
// Full list: https://platform.claude.com/docs/en/about-claude/models/overview
//
// Claude 4.5 (Latest):
//   - claude-sonnet-4-5-20250929  : Smart model for complex agents and coding
//   - claude-haiku-4-5-20251001   : Fastest with near-frontier intelligence
//   - claude-opus-4-5-20251101    : Maximum intelligence, premium performance
//
// Legacy (Claude 4 and earlier):
//   - claude-opus-4-1-20250805    : Previous Opus generation
//   - claude-sonnet-4-20250514    : Previous Sonnet generation
//   - claude-3-7-sonnet-20250219  : Claude 3.7 Sonnet
//   - claude-opus-4-20250514      : Claude 4 Opus
//   - claude-3-haiku-20240307     : Fast and cost-effective

// Anthropic implements Provider for Anthropic's Claude API.
type Anthropic struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// AnthropicOption configures an Anthropic provider.
type AnthropicOption func(*Anthropic)

// WithAnthropicBaseURL sets a custom base URL.
func WithAnthropicBaseURL(url string) AnthropicOption {
	return func(a *Anthropic) { a.baseURL = url }
}

// WithAnthropicHTTPClient sets a custom HTTP client.
func WithAnthropicHTTPClient(c *http.Client) AnthropicOption {
	return func(a *Anthropic) { a.httpClient = c }
}

// NewAnthropic creates an Anthropic provider.
// Reads API key from ANTHROPIC_API_KEY environment variable.
func NewAnthropic(opts ...AnthropicOption) (*Anthropic, error) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return nil, errors.New("ANTHROPIC_API_KEY environment variable required")
	}

	a := &Anthropic{
		apiKey:     apiKey,
		baseURL:    "https://api.anthropic.com/v1",
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}

	for _, opt := range opts {
		opt(a)
	}

	return a, nil
}

// Query sends a prompt to a Claude model and returns the response.
func (a *Anthropic) Query(ctx context.Context, req Request) (Response, error) {
	start := time.Now()

	payload := anthropicRequest{
		Model:     req.Model,
		MaxTokens: 4096,
		Messages: []anthropicMessage{
			{Role: "user", Content: req.Prompt},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return Response{}, fmt.Errorf("marshaling request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+"/messages", bytes.NewReader(body))
	if err != nil {
		return Response{}, fmt.Errorf("creating request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", a.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := a.httpClient.Do(httpReq)
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

	var anthropicResp anthropicResponse
	if err := json.Unmarshal(respBody, &anthropicResp); err != nil {
		return Response{}, fmt.Errorf("parsing response: %w", err)
	}

	if len(anthropicResp.Content) == 0 {
		return Response{}, errors.New("no content in response")
	}

	return Response{
		Model:    req.Model,
		Content:  anthropicResp.Content[0].Text,
		Provider: "anthropic",
		Latency:  time.Since(start),
	}, nil
}

// QueryStream sends a prompt to a Claude model and streams the response.
func (a *Anthropic) QueryStream(ctx context.Context, req Request, callback StreamCallback) (Response, error) {
	start := time.Now()

	payload := anthropicStreamRequest{
		Model:     req.Model,
		MaxTokens: 4096,
		Messages: []anthropicMessage{
			{Role: "user", Content: req.Prompt},
		},
		Stream: true,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return Response{}, fmt.Errorf("marshaling request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+"/messages", bytes.NewReader(body))
	if err != nil {
		return Response{}, fmt.Errorf("creating request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", a.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return Response{}, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return Response{}, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var fullContent strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")

		var event anthropicStreamEvent
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		if event.Type == "content_block_delta" && event.Delta.Type == "text_delta" {
			chunk := event.Delta.Text
			fullContent.WriteString(chunk)
			if callback != nil {
				callback(chunk)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return Response{}, fmt.Errorf("reading stream: %w", err)
	}

	return Response{
		Model:    req.Model,
		Content:  fullContent.String(),
		Provider: "anthropic",
		Latency:  time.Since(start),
	}, nil
}

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	Messages  []anthropicMessage `json:"messages"`
}

type anthropicStreamRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	Messages  []anthropicMessage `json:"messages"`
	Stream    bool               `json:"stream"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
}

type anthropicStreamEvent struct {
	Type  string `json:"type"`
	Delta struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"delta,omitempty"`
}
