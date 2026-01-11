package output

import (
	"github.com/johnayoung/llm-consensus/internal/provider"
)

// Result is the JSON output structure for the CLI.
type Result struct {
	Prompt       string              `json:"prompt"`
	Responses    []provider.Response `json:"responses"`
	Consensus    string              `json:"consensus"`
	Judge        string              `json:"judge"`
	Warnings     []string            `json:"warnings,omitempty"`
	FailedModels []string            `json:"failed_models,omitempty"`
}
