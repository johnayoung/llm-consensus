# llm-consensus

CLI tool that queries multiple LLMs with the same prompt and synthesizes a consensus response using LLM-as-Judge.

## Features

- **Multi-model queries** - Query multiple LLMs in parallel
- **Streaming responses** - Real-time progress with token counts
- **Interactive UI** - Colored output with spinners and progress indicators
- **Auto-save runs** - Each run saved to `data/<run-id>/` for history
- **LLM-as-Judge** - Synthesizes consensus from multiple responses
- **Minimal dependencies** - Only uses `golang.org/x/sync`

## Installation

### From GitHub Releases (recommended)

Download the latest binary for your platform from [Releases](https://github.com/johnayoung/llm-consensus/releases).

### Using Go

```bash
go install github.com/johnayoung/llm-consensus/cmd/llm-consensus@latest
```

### Build from source

```bash
git clone https://github.com/johnayoung/llm-consensus.git
cd llm-consensus
go build -o llm-consensus ./cmd/llm-consensus
```

## Configuration

Set API keys as environment variables:

```bash
export OPENAI_API_KEY=sk-...
export ANTHROPIC_API_KEY=sk-ant-...
export GOOGLE_API_KEY=AI...
```

## Usage

```bash
llm-consensus --models <model1,model2,...> [options] [prompt]
```

### Flags

| Flag          | Description                                        | Default                  |
| ------------- | -------------------------------------------------- | ------------------------ |
| `--models`    | Comma-separated list of models to query (required) | -                        |
| `--judge`     | Model for consensus synthesis                      | `gpt-5.2-pro-2025-12-11` |
| `--file`      | Read prompt from file                              | -                        |
| `--output`    | Write JSON to specific file (overrides auto-save)  | -                        |
| `--data-dir`  | Directory for auto-saved runs                      | `data`                   |
| `--timeout`   | Per-model timeout in seconds                       | `120`                    |
| `--json`      | Output JSON to stdout (no UI, no auto-save)        | `false`                  |
| `--no-save`   | Disable auto-save to data directory                | `false`                  |
| `-q, --quiet` | Suppress progress output                           | `false`                  |
| `--version`   | Print version information                          | -                        |

## Examples

```bash
# Basic query
llm-consensus --models gpt-5.2-2025-12-11,claude-sonnet-4-5 "What causes aurora borealis?"

# Custom judge model
llm-consensus --models gpt-5.2-2025-12-11,claude-sonnet-4-5 --judge gemini-3-pro-preview "Explain quicksort"

# From file
llm-consensus --models gpt-5.2-2025-12-11,claude-sonnet-4-5 --file prompt.md

# From stdin
cat complex_prompt.md | llm-consensus --models gpt-5.2-2025-12-11,claude-sonnet-4-5

# JSON output for scripting
llm-consensus --models gpt-5.2-2025-12-11,claude-sonnet-4-5 --json "What is the capital of France?" | jq -r '.consensus'
```

## Supported Models

| Provider  | Models                                                                  |
| --------- | ----------------------------------------------------------------------- |
| OpenAI    | `gpt-5.2-2025-12-11`, `gpt-5.2-pro-2025-12-11` (default judge)          |
| Anthropic | `claude-sonnet-4-5`, `claude-haiku-4-5`, `claude-opus-4-5`              |
| Google    | `gemini-3-pro-preview`                                                  |

## Output

Auto-saved runs are stored in `data/<run-id>/`:

```
data/20260112-143052-a1b2c3/
├── result.json    # Full JSON output
├── prompt.txt     # Original prompt
└── consensus.md   # Consensus answer
```

JSON structure:

```json
{
  "prompt": "What is 2+2?",
  "responses": [
    {"model": "gpt-5.2-2025-12-11", "provider": "openai", "content": "4", "latency_ms": 1234}
  ],
  "consensus": "The answer is 4.",
  "judge": "gpt-5.2-pro-2025-12-11",
  "warnings": [],
  "failed_models": []
}
```

## Project Structure

```
llm-consensus/
├── cmd/
│   ├── llm-consensus/           # Main CLI application
│   └── model-registry-sync/     # Utility to sync available models
├── internal/
│   ├── consensus/               # LLM-as-Judge synthesis
│   ├── provider/                # LLM provider implementations (OpenAI, Anthropic, Google)
│   ├── runner/                  # Parallel query orchestration
│   ├── output/                  # JSON output formatting
│   └── ui/                      # Terminal UI and progress display
└── data/                        # Auto-saved run history (gitignored)
```

## License

MIT
