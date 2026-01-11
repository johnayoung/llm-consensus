# llm-consensus

CLI tool that queries multiple LLMs with the same prompt and synthesizes a consensus response using LLM-as-Judge.

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
llm-consensus --models <model1,model2,...> [--judge <model>] [--file <path>] [--timeout <seconds>] [prompt]
```

| Flag        | Description                                        | Default  |
| ----------- | -------------------------------------------------- | -------- |
| `--models`  | Comma-separated list of models to query (required) | -        |
| `--judge`   | Model for consensus synthesis                      | `gpt-4o` |
| `--file`    | Read prompt from file                              | -        |
| `--timeout` | Per-model timeout in seconds                       | `30`     |

## Examples

Basic query:
```bash
llm-consensus --models gpt-4o,claude-sonnet-4-5-20250929,gemini-1.5-pro "What causes aurora borealis?"
```

Custom judge model:
```bash
llm-consensus --models gpt-4o,claude-sonnet-4-5-20250929 --judge gemini-1.5-pro "Explain quicksort"
```

From file:
```bash
llm-consensus --models gpt-4o,claude-sonnet-4-5-20250929 --file prompt.txt
```

From stdin:
```bash
echo "What is 2+2?" | llm-consensus --models gpt-4o,claude-sonnet-4-5-20250929
```

```bash
cat complex_prompt.md | llm-consensus --models gpt-4o,claude-sonnet-4-5-20250929
```

Parse JSON output with jq:
```bash
llm-consensus --models gpt-4o,claude-sonnet-4-5-20250929 "What is the capital of France?" | jq -r '.consensus'
```

## Output

JSON output includes:

```json
{
  "prompt": "What is 2+2?",
  "responses": [
    {"model": "gpt-4o", "provider": "openai", "content": "4", "latency_ms": 1234},
    {"model": "claude-sonnet-4-5-20250929", "provider": "anthropic", "content": "4", "latency_ms": 1456}
  ],
  "consensus": "The answer is 4.",
  "judge": "gpt-4o",
  "warnings": [],
  "failed_models": []
}
```

## Supported Models

### OpenAI
https://platform.openai.com/docs/models

- `gpt-4o` - Most capable, multimodal
- `gpt-4o-mini` - Fast, cost-effective
- `o1`, `o1-mini`, `o3-mini` - Reasoning models
- `gpt-4-turbo`, `gpt-3.5-turbo` - Legacy

### Anthropic
https://platform.claude.com/docs/en/about-claude/models/overview

- `claude-sonnet-4-5-20250929` - Smart model for complex agents and coding
- `claude-haiku-4-5-20251001` - Fastest with near-frontier intelligence
- `claude-opus-4-5-20251101` - Maximum intelligence, premium performance
- `claude-opus-4-1-20250805`, `claude-sonnet-4-5-20250929`, `claude-3-haiku-20240307` - Legacy

### Google
https://ai.google.dev/gemini-api/docs/models/gemini

- `gemini-2.0-flash`, `gemini-2.0-flash-lite` - Gemini 2.0
- `gemini-1.5-pro`, `gemini-1.5-flash`, `gemini-1.5-flash-8b` - Gemini 1.5
