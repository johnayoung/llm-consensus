package consensus

import (
	"bytes"
	"context"
	"fmt"
	"text/template"

	"github.com/johnayoung/llm-consensus/internal/provider"
)

const judgePromptTemplate = `
Role
You are an expert synthesis judge and careful editor. Your job is to combine multiple AI model responses into one best-possible answer to the user.

Inputs
User's original prompt:
{{.Prompt}}

Model responses:
{{range .Responses}}
--- Model: {{.Model}} | Provider: {{.Provider}} ---
{{.Content}}

{{end}}

Task
Produce ONE final answer that directly addresses the user's original prompt by synthesizing the model responses.

Method
1) Infer the user's intent and constraints from the original prompt (scope, tone, formatting, assumptions). Follow them.
2) Extract the strongest points that are supported and/or repeated across responses.
3) Resolve conflicts:
   - Prefer statements that are more logically sound, more specific, and better justified.
   - Prefer safer, broadly valid guidance over speculative or brittle claims.
   - If uncertainty remains, choose the most defensible formulation and qualify it briefly.
4) Fill gaps only when needed to make the answer complete and usable. Do not invent facts; do not add extraneous content.

Output Requirements
- Output ONLY the final synthesized answer (no preamble, no meta-commentary, no mention of models or “consensus”).
- Do not quote or reference individual model responses.
- Keep the answer coherent, non-redundant, and well-structured (use bullets/steps/headings if helpful).
- Match formatting appropriate to the task (e.g., code blocks for code).
`

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
	return j.SynthesizeStream(ctx, originalPrompt, responses, nil)
}

// SynthesizeStream generates a consensus response with streaming callback.
func (j *Judge) SynthesizeStream(ctx context.Context, originalPrompt string, responses []provider.Response, callback provider.StreamCallback) (string, error) {
	if len(responses) == 0 {
		return "", fmt.Errorf("no responses to synthesize")
	}

	// If only one response, return it directly (no consensus needed)
	if len(responses) == 1 {
		if callback != nil {
			callback(responses[0].Content)
		}
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

	// Query judge model with streaming
	resp, err := j.provider.QueryStream(ctx, provider.Request{
		Model:  j.model,
		Prompt: buf.String(),
	}, callback)
	if err != nil {
		return "", fmt.Errorf("judge query failed: %w", err)
	}

	return resp.Content, nil
}
