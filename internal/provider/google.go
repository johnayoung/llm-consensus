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

// Google Gemini Models
// Full list: https://ai.google.dev/gemini-api/docs/models
//
// Gemini 3:
//   - gemini-3-pro               : Most intelligent, multimodal understanding, agentic
//   - gemini-3-flash             : Most balanced, built for speed and scale
//
// Gemini 2.5:
//   - gemini-2.5-pro             : Advanced thinking model, complex reasoning
//   - gemini-2.5-flash           : Best price-performance, large scale processing
//   - gemini-2.5-flash-lite      : Fastest flash, cost-efficient, high throughput
//
// Gemini 2.0 (Previous):
//   - gemini-2.0-flash           : Second generation workhorse, 1M context
//   - gemini-2.0-flash-lite      : Second generation small workhorse, 1M context

// Google implements Provider for Google's Gemini API.
type Google struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// GoogleOption configures a Google provider.
type GoogleOption func(*Google)

// WithGoogleBaseURL sets a custom base URL.
func WithGoogleBaseURL(url string) GoogleOption {
	return func(g *Google) { g.baseURL = url }
}

// WithGoogleHTTPClient sets a custom HTTP client.
func WithGoogleHTTPClient(c *http.Client) GoogleOption {
	return func(g *Google) { g.httpClient = c }
}

// NewGoogle creates a Google/Gemini provider.
// Reads API key from GOOGLE_API_KEY environment variable.
func NewGoogle(opts ...GoogleOption) (*Google, error) {
	apiKey := os.Getenv("GOOGLE_API_KEY")
	if apiKey == "" {
		return nil, errors.New("GOOGLE_API_KEY environment variable required")
	}

	g := &Google{
		apiKey:     apiKey,
		baseURL:    "https://generativelanguage.googleapis.com/v1beta",
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}

	for _, opt := range opts {
		opt(g)
	}

	return g, nil
}

// Query sends a prompt to a Gemini model and returns the response.
func (g *Google) Query(ctx context.Context, req Request) (Response, error) {
	start := time.Now()

	payload := geminiRequest{
		Contents: []geminiContent{
			{
				Parts: []geminiPart{
					{Text: req.Prompt},
				},
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return Response{}, fmt.Errorf("marshaling request: %w", err)
	}

	// Gemini uses model name in URL path
	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", g.baseURL, req.Model, g.apiKey)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return Response{}, fmt.Errorf("creating request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := g.httpClient.Do(httpReq)
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

	var geminiResp geminiResponse
	if err := json.Unmarshal(respBody, &geminiResp); err != nil {
		return Response{}, fmt.Errorf("parsing response: %w", err)
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return Response{}, errors.New("no content in response")
	}

	return Response{
		Model:    req.Model,
		Content:  geminiResp.Candidates[0].Content.Parts[0].Text,
		Provider: "google",
		Latency:  time.Since(start),
	}, nil
}

type geminiRequest struct {
	Contents []geminiContent `json:"contents"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}
