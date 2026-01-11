package consensus

import (
	"bytes"
	"context"
	"fmt"
	"text/template"

	"github.com/johnayoung/llm-consensus/internal/provider"
)

const judgePromptTemplate = `You are a judge synthesizing responses from multiple AI models.

User's original prompt:
{{.Prompt}}

Model responses:
{{range .Responses}}
--- {{.Model}} ({{.Provider}}) ---
{{.Content}}

{{end}}
Synthesize a consensus response that:
1. Identifies points of agreement across models
2. Resolves contradictions by reasoning through them
3. Produces a single, coherent answer that represents the best synthesis

Provide only the consensus response, without meta-commentary about the synthesis process.`

var tmpl = template.Must(template.New("judge").Parse(judgePromptTemplate))

// Judge synthesizes consensus from multiple model responses.
type Judge struct {
	provider provider.Provider
	model    string
}

// NewJudge creates a judge using the specified provider and model.
func NewJudge(p provider.Provider, model string) *Judge {
	return &Judge{
		provider: p,
		model:    model,
	}
}

// Synthesize generates a consensus response from multiple model outputs.
func (j *Judge) Synthesize(ctx context.Context, originalPrompt string, responses []provider.Response) (string, error) {
	if len(responses) == 0 {
		return "", fmt.Errorf("no responses to synthesize")
	}

	// If only one response, return it directly (no consensus needed)
	if len(responses) == 1 {
		return responses[0].Content, nil
	}

	// Build judge prompt
	data := struct {
		Prompt    string
		Responses []provider.Response
	}{
		Prompt:    originalPrompt,
		Responses: responses,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("executing template: %w", err)
	}

	// Query judge model
	resp, err := j.provider.Query(ctx, provider.Request{
		Model:  j.model,
		Prompt: buf.String(),
	})
	if err != nil {
		return "", fmt.Errorf("judge query failed: %w", err)
	}

	return resp.Content, nil
}
