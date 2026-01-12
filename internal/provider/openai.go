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
// Uses the Responses API for better reasoning performance and pro model support.
func (o *OpenAI) Query(ctx context.Context, req Request) (Response, error) {
	start := time.Now()

	payload := responsesRequest{
		Model: req.Model,
		Input: req.Prompt,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return Response{}, fmt.Errorf("marshaling request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL+"/responses", bytes.NewReader(body))
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

	var responsesResp responsesResponse
	if err := json.Unmarshal(respBody, &responsesResp); err != nil {
		return Response{}, fmt.Errorf("parsing response: %w", err)
	}

	// Extract text from output items
	content := extractResponseText(responsesResp.Output)
	if content == "" {
		return Response{}, errors.New("no content in response")
	}

	return Response{
		Model:    req.Model,
		Content:  content,
		Provider: "openai",
		Latency:  time.Since(start),
	}, nil
}

// QueryStream sends a prompt to an OpenAI model and streams the response.
// Uses the Responses API with streaming enabled.
func (o *OpenAI) QueryStream(ctx context.Context, req Request, callback StreamCallback) (Response, error) {
	start := time.Now()

	payload := responsesStreamRequest{
		Model:  req.Model,
		Input:  req.Prompt,
		Stream: true,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return Response{}, fmt.Errorf("marshaling request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL+"/responses", bytes.NewReader(body))
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
		if data == "[DONE]" {
			break
		}

		var event responsesStreamEvent
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		// Handle text delta events
		if event.Type == "response.output_text.delta" && event.Delta != "" {
			fullContent.WriteString(event.Delta)
			if callback != nil {
				callback(event.Delta)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return Response{}, fmt.Errorf("reading stream: %w", err)
	}

	return Response{
		Model:    req.Model,
		Content:  fullContent.String(),
		Provider: "openai",
		Latency:  time.Since(start),
	}, nil
}

// Responses API types (recommended for GPT-5 and reasoning models)
// https://platform.openai.com/docs/api-reference/responses

type responsesRequest struct {
	Model        string `json:"model"`
	Input        string `json:"input"`
	Instructions string `json:"instructions,omitempty"`
}

type responsesStreamRequest struct {
	Model        string `json:"model"`
	Input        string `json:"input"`
	Instructions string `json:"instructions,omitempty"`
	Stream       bool   `json:"stream"`
}

type responsesResponse struct {
	ID     string            `json:"id"`
	Output []responsesOutput `json:"output"`
}

type responsesOutput struct {
	Type    string             `json:"type"`
	Content []responsesContent `json:"content,omitempty"`
}

type responsesContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type responsesStreamEvent struct {
	Type  string `json:"type"`
	Delta string `json:"delta,omitempty"`
}

// extractResponseText extracts text content from Responses API output.
func extractResponseText(outputs []responsesOutput) string {
	var result strings.Builder
	for _, output := range outputs {
		if output.Type == "message" {
			for _, content := range output.Content {
				if content.Type == "output_text" {
					result.WriteString(content.Text)
				}
			}
		}
	}
	return result.String()
}
