# llm-consensus

CLI tool that queries multiple LLMs with the same prompt and synthesizes a consensus response using LLM-as-Judge.

## Features

- **Multi-model queries** - Query multiple LLMs in parallel
- **Streaming responses** - Real-time progress with token counts
- **Interactive UI** - Colored output with spinners and progress indicators
- **Auto-save runs** - Each run saved to `data/<run-id>/` for history
- **LLM-as-Judge** - Synthesizes consensus from multiple responses

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

### Verify installation

```bash
llm-consensus --version
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

### Basic query
```bash
llm-consensus --models gpt-5.2-2025-12-11,claude-sonnet-4-5 "What causes aurora borealis?"
```

### Custom judge model
```bash
llm-consensus --models gpt-5.2-2025-12-11,claude-sonnet-4-5 --judge gemini-3-pro-preview "Explain quicksort"
```

### From file
```bash
llm-consensus --models gpt-5.2-2025-12-11,claude-sonnet-4-5 --file prompt.md
```

### From stdin
```bash
echo "What is 2+2?" | llm-consensus --models gpt-5.2-2025-12-11,claude-sonnet-4-5
```

```bash
cat complex_prompt.md | llm-consensus --models gpt-5.2-2025-12-11,claude-sonnet-4-5
```

### JSON output for scripting
```bash
llm-consensus --models gpt-5.2-2025-12-11,claude-sonnet-4-5 --json "What is the capital of France?" | jq -r '.consensus'
```

### Save to specific file
```bash
llm-consensus --models gpt-5.2-2025-12-11,claude-sonnet-4-5 --output result.json "Explain Go interfaces"
```

### Disable auto-save
```bash
llm-consensus --models gpt-5.2-2025-12-11,claude-sonnet-4-5 --no-save "Quick question"
```

## Auto-Save

By default, each run is saved to `data/<run-id>/` containing:

```
data/20260112-143052-a1b2c3/
├── result.json    # Full JSON output with all responses
├── prompt.txt     # Original prompt
└── consensus.md   # Just the consensus (easy to read/share)
```

Run IDs use the format `YYYYMMDD-HHMMSS-<random>` for easy sorting and uniqueness.

Use `--no-save` to disable, `--data-dir` to change the directory, or `--output` to specify an exact file.

## Interactive UI

When running in a terminal, you'll see real-time progress:

```
╭─ LLM Consensus ─╮
│ Prompt: What causes aurora borealis?
╰─────────────────╯

▸ Querying models...
⚡ Querying 3 models (2.5s)
  ✓ gpt-5.2-2025-12-11       done ~450 tokens in 1.8s
  ⠙ claude-sonnet-4-5        streaming ~320 tokens 1.5s
  ✓ gemini-3-pro-preview     done ~380 tokens in 2.1s
```

The UI auto-detects terminal vs pipe and adjusts output accordingly.

## Output

JSON output structure:

```json
{
  "prompt": "What is 2+2?",
  "responses": [
    {"model": "gpt-5.2-2025-12-11", "provider": "openai", "content": "4", "latency_ms": 1234},
    {"model": "claude-sonnet-4-5", "provider": "anthropic", "content": "4", "latency_ms": 1456}
  ],
  "consensus": "The answer is 4.",
  "judge": "gpt-5.2-pro-2025-12-11",
  "warnings": [],
  "failed_models": []
}
```

## Supported Models

### OpenAI
Uses the [Responses API](https://platform.openai.com/docs/api-reference/responses) for better reasoning performance.

- `gpt-5.2-2025-12-11` - Best for coding and agentic tasks
- `gpt-5.2-pro-2025-12-11` - Smarter, more precise responses (default judge)

### Anthropic

- `claude-sonnet-4-5` - Smart model for complex agents and coding
- `claude-haiku-4-5` - Fastest with near-frontier intelligence
- `claude-opus-4-5` - Maximum intelligence, premium performance

### Google

- `gemini-3-pro-preview` - Most intelligent, multimodal understanding

## License

MIT
