# Proposed Features for llm-consensus

Analysis of the codebase identified these high-value feature opportunities organized by category.

---

## 1. Enhanced Model Management

### 1.1 Dynamic Model Discovery
Currently, models are hardcoded in `main.go`. Add runtime model discovery from provider APIs.

**Implementation:**
- Extend `model-registry-sync` to write to a config file
- Load available models at startup
- Add `--list-models` flag to show available models
- Validate user-specified models against registry

### 1.2 Model Aliases and Presets
Allow shorthand aliases and model groups for common configurations.

**Examples:**
```bash
llm-consensus --models @fast "prompt"      # claude-haiku, gpt-5.2
llm-consensus --models @all-claude "prompt" # all claude variants
llm-consensus --models @reasoning "prompt"  # models optimized for reasoning
```

**Implementation:** Config file (`~/.llm-consensus.yaml`) with alias definitions.

### 1.3 Model Cost Tracking
Track and display estimated API costs per query.

**Features:**
- Pricing data per model (input/output tokens)
- Per-model cost in summary output
- Total run cost
- Add `--budget` flag to warn/abort if estimated cost exceeds limit

---

## 2. Consensus & Synthesis Improvements

### 2.1 Configurable Judge Prompts
Allow custom judge prompts for different synthesis strategies.

**Use Cases:**
- Technical accuracy focus
- Creative writing synthesis
- Code review aggregation
- Research summary

**Implementation:**
- `--judge-prompt-file` flag
- Built-in prompt templates: `--judge-style [balanced|technical|creative]`

### 2.2 Multi-Round Consensus
For complex questions, allow iterative refinement.

**Flow:**
1. Initial responses from all models
2. Judge synthesizes v1
3. Models critique the synthesis
4. Judge produces final v2

**Implementation:** `--rounds N` flag (default 1).

### 2.3 Voting Mode
Instead of synthesis, let models vote on predefined options.

**Example:**
```bash
llm-consensus --vote --options "A,B,C" --models ... "Which option is best?"
```

**Output:** Vote counts, confidence scores, reasoning summaries.

### 2.4 Confidence Scoring
Have the judge rate confidence in the consensus.

**Output additions:**
- Confidence score (0-100)
- Agreement level (how much models aligned)
- Controversy points (where models disagreed)

---

## 3. Input & Prompt Features

### 3.1 Multi-Turn Conversations
Support conversation history for follow-up queries.

**Implementation:**
- `--continue <run-id>` to continue from previous run
- Store conversation history in run data
- Load context from prior runs

### 3.2 System Prompt Support
Allow custom system prompts for all models.

**Flags:**
- `--system "You are an expert in..."`
- `--system-file instructions.md`

### 3.3 Template Variables
Support variable substitution in prompts.

**Example:**
```bash
llm-consensus --var topic="quantum computing" --file template.md --models ...
```

Template: `Explain {{topic}} to a beginner.`

### 3.4 Batch Processing
Process multiple prompts from a file.

**Implementation:**
- `--batch prompts.jsonl` (one prompt per line)
- Parallel or sequential execution
- Aggregate output file

---

## 4. Output & Analysis

### 4.1 Diff View Mode
Show differences between model responses.

**Features:**
- Side-by-side comparison
- Highlight agreements/disagreements
- Semantic similarity scores

### 4.2 Response Caching
Cache model responses to avoid re-querying for identical prompts.

**Implementation:**
- Content-addressed cache (hash of model+prompt)
- `--no-cache` to force fresh queries
- `--cache-ttl` for expiration

### 4.3 Export Formats
Expand output format options.

**New formats:**
- `--format markdown` - Formatted markdown report
- `--format html` - Self-contained HTML report with styling
- `--format csv` - Tabular data for spreadsheet analysis

### 4.4 Run Comparison
Compare results across multiple runs.

```bash
llm-consensus compare <run-id-1> <run-id-2>
```

Shows: Response diffs, consensus evolution, timing comparisons.

---

## 5. Reliability & Performance

### 5.1 Retry Logic with Backoff
Add configurable retry for transient failures.

**Flags:**
- `--retries N` (default 0)
- `--retry-delay 1s,2s,4s` (exponential backoff)

### 5.2 Rate Limiting
Respect provider rate limits and queue requests appropriately.

**Implementation:**
- Per-provider rate limiters
- Token bucket algorithm
- Display rate limit status in UI

### 5.3 Request Prioritization
For large batches, prioritize faster/cheaper models first.

**Modes:**
- `--priority fastest` - Fast models first
- `--priority cheapest` - Minimize cost
- `--priority balanced` - Default parallel

### 5.4 Health Checks
Pre-flight checks before running queries.

```bash
llm-consensus --health-check
```

Validates: API keys, model availability, network connectivity.

---

## 6. Provider Expansion

### 6.1 OpenRouter Integration
Add OpenRouter as a meta-provider for accessing many models.

**Benefits:**
- Single API key for multiple providers
- Access to open-source models (Llama, Mistral, etc.)
- Unified billing

### 6.2 Local Model Support
Support local models via Ollama or similar.

**Implementation:**
- New `ollama` provider
- Auto-detect local models
- `--ollama-url` for custom endpoints

### 6.3 Azure OpenAI Support
Add Azure-hosted OpenAI models.

**Flags:**
- `--azure-endpoint`
- `--azure-deployment`
- Support for managed identity auth

### 6.4 AWS Bedrock Support
Add Amazon Bedrock provider for enterprise deployments.

---

## 7. Developer Experience

### 7.1 Configuration File
Support persistent configuration.

**Location:** `~/.llm-consensus.yaml` or `.llm-consensus.yaml` in project.

**Contents:**
- Default models
- Judge model preference
- API endpoints
- Aliases and presets
- Default flags

### 7.2 Interactive Mode
REPL-style interface for multiple queries.

```bash
llm-consensus --interactive --models claude-sonnet-4-5,gpt-5.2
> What is the capital of France?
[responses...]
> /models +gemini-3-pro  # add model
> What about Germany?
[responses...]
```

### 7.3 Plugin System
Allow custom providers via plugins.

**Mechanism:**
- Go plugin interface
- Load from `~/.llm-consensus/plugins/`
- Register custom models/providers

### 7.4 Debug Mode
Enhanced debugging output.

**Flags:**
- `--debug` - Show full request/response payloads
- `--trace` - Detailed timing breakdown
- `--dry-run` - Show what would be sent without executing

---

## 8. Analysis & Insights

### 8.1 Response Analytics
Automatic analysis of model responses.

**Metrics:**
- Response length distribution
- Vocabulary complexity
- Sentiment alignment
- Topic coverage

### 8.2 Model Performance Tracking
Track model behavior over time.

**Features:**
- Latency percentiles per model
- Failure rates
- Cost per model
- Quality scores (if user provides feedback)

### 8.3 Embedding-Based Similarity
Use embeddings to compute semantic similarity between responses.

**Output:**
- Similarity matrix
- Cluster visualization
- Outlier detection (model giving very different answer)

---

## Priority Recommendations

**High Impact, Low Effort:**
1. System prompt support (3.2)
2. Retry logic (5.1)
3. Configuration file (7.1)
4. `--list-models` flag (1.1)

**High Impact, Medium Effort:**
1. Response caching (4.2)
2. OpenRouter integration (6.1)
3. Configurable judge prompts (2.1)
4. Cost tracking (1.3)

**High Impact, High Effort:**
1. Multi-turn conversations (3.1)
2. Interactive mode (7.2)
3. Local model support (6.2)
4. Batch processing (3.4)

---

## Implementation Notes

Most features can be added incrementally without breaking changes:
- New flags default to current behavior
- Provider interface already supports extension
- Output structure can be extended with optional fields
- Auto-save format can version for backwards compatibility
