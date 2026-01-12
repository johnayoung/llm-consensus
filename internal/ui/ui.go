package ui

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// Color codes for terminal output.
const (
	Reset      = "\033[0m"
	Bold       = "\033[1m"
	Dim        = "\033[2m"
	Green      = "\033[32m"
	Yellow     = "\033[33m"
	Blue       = "\033[34m"
	Magenta    = "\033[35m"
	Cyan       = "\033[36m"
	Red        = "\033[31m"
	BoldGreen  = "\033[1;32m"
	BoldYellow = "\033[1;33m"
	BoldBlue   = "\033[1;34m"
	BoldCyan   = "\033[1;36m"
)

// ModelStatus represents the current state of a model query.
type ModelStatus int

const (
	StatusPending ModelStatus = iota
	StatusRunning
	StatusStreaming
	StatusComplete
	StatusFailed
)

// ModelState holds the state of a single model query.
type ModelState struct {
	Model     string
	Status    ModelStatus
	StartTime time.Time
	EndTime   time.Time
	Error     error
	CharCount int
	TokenEst  int // rough token estimate
	LastChunk string
}

// Progress displays real-time progress of LLM queries.
type Progress struct {
	mu        sync.Mutex
	w         io.Writer
	models    map[string]*ModelState
	order     []string
	startTime time.Time
	ticker    *time.Ticker
	done      chan struct{}
	quiet     bool
	rendered  bool
}

// NewProgress creates a new progress display.
func NewProgress(w io.Writer, models []string, quiet bool) *Progress {
	p := &Progress{
		w:         w,
		models:    make(map[string]*ModelState),
		order:     models,
		startTime: time.Now(),
		done:      make(chan struct{}),
		quiet:     quiet,
	}

	for _, m := range models {
		p.models[m] = &ModelState{
			Model:  m,
			Status: StatusPending,
		}
	}

	return p
}

// Start begins the progress display refresh loop.
func (p *Progress) Start() {
	if p.quiet {
		return
	}

	p.ticker = time.NewTicker(100 * time.Millisecond)
	go func() {
		for {
			select {
			case <-p.ticker.C:
				p.render()
			case <-p.done:
				return
			}
		}
	}()

	// Initial render
	p.render()
}

// Stop ends the progress display.
func (p *Progress) Stop() {
	if p.quiet {
		return
	}

	close(p.done)
	if p.ticker != nil {
		p.ticker.Stop()
	}
	if p.rendered {
		p.clearLines(len(p.order) + 2)
	}
}

// ModelStarted marks a model as starting its query.
func (p *Progress) ModelStarted(model string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if state, ok := p.models[model]; ok {
		state.Status = StatusRunning
		state.StartTime = time.Now()
	}
}

// ModelStreaming updates the streaming state for a model.
func (p *Progress) ModelStreaming(model string, chunk string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if state, ok := p.models[model]; ok {
		state.Status = StatusStreaming
		state.CharCount += len(chunk)
		state.TokenEst = state.CharCount / 4 // rough estimate: ~4 chars per token
		state.LastChunk = truncate(chunk, 30)
	}
}

// ModelCompleted marks a model as finished.
func (p *Progress) ModelCompleted(model string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if state, ok := p.models[model]; ok {
		state.Status = StatusComplete
		state.EndTime = time.Now()
	}
}

// ModelFailed marks a model as failed.
func (p *Progress) ModelFailed(model string, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if state, ok := p.models[model]; ok {
		state.Status = StatusFailed
		state.EndTime = time.Now()
		state.Error = err
	}
}

// render draws the current progress state.
func (p *Progress) render() {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Clear previous lines if we've rendered before
	if p.rendered {
		p.clearLines(len(p.order) + 2)
	}
	p.rendered = true

	elapsed := time.Since(p.startTime)

	// Header
	fmt.Fprintf(p.w, "%s⚡ Querying %d models%s %s(%.1fs)%s\n",
		BoldCyan, len(p.order), Reset,
		Dim, elapsed.Seconds(), Reset)

	// Model status lines
	for _, model := range p.order {
		state := p.models[model]
		p.renderModelLine(state)
	}

	// Empty line for spacing
	fmt.Fprintln(p.w)
}

// renderModelLine draws a single model's status.
func (p *Progress) renderModelLine(state *ModelState) {
	var icon, color, status string

	switch state.Status {
	case StatusPending:
		icon = "○"
		color = Dim
		status = "pending"
	case StatusRunning:
		icon = spinner(time.Now())
		color = Yellow
		elapsed := time.Since(state.StartTime)
		status = fmt.Sprintf("connecting... %.1fs", elapsed.Seconds())
	case StatusStreaming:
		icon = spinner(time.Now())
		color = Cyan
		elapsed := time.Since(state.StartTime)
		status = fmt.Sprintf("streaming ~%d tokens %.1fs", state.TokenEst, elapsed.Seconds())
	case StatusComplete:
		icon = "✓"
		color = Green
		duration := state.EndTime.Sub(state.StartTime)
		status = fmt.Sprintf("done ~%d tokens in %.1fs", state.TokenEst, duration.Seconds())
	case StatusFailed:
		icon = "✗"
		color = Red
		status = fmt.Sprintf("failed: %v", state.Error)
	}

	// Truncate model name if too long
	modelName := truncate(state.Model, 25)

	fmt.Fprintf(p.w, "  %s%s%s %-25s %s%s%s\n",
		color, icon, Reset,
		modelName,
		color, status, Reset)
}

// clearLines moves cursor up and clears lines.
func (p *Progress) clearLines(n int) {
	for i := 0; i < n; i++ {
		fmt.Fprintf(p.w, "\033[A\033[K") // Move up, clear line
	}
}

// spinner returns a spinning character based on time.
func spinner(t time.Time) string {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	idx := int(t.UnixMilli()/100) % len(frames)
	return frames[idx]
}

// truncate shortens a string to max length.
func truncate(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimSpace(s)
	if len(s) > max {
		return s[:max-1] + "…"
	}
	return s
}

// PrintHeader prints a styled header.
func PrintHeader(w io.Writer, prompt string) {
	fmt.Fprintf(w, "\n%s╭─ LLM Consensus ─╮%s\n", BoldCyan, Reset)
	displayPrompt := truncate(prompt, 60)
	fmt.Fprintf(w, "%s│%s Prompt: %s%s%s\n", Cyan, Reset, Dim, displayPrompt, Reset)
	fmt.Fprintf(w, "%s╰─────────────────╯%s\n\n", Cyan, Reset)
}

// PrintPhase prints a phase header.
func PrintPhase(w io.Writer, phase string) {
	fmt.Fprintf(w, "%s▸ %s%s\n", BoldYellow, phase, Reset)
}

// PrintSuccess prints a success message.
func PrintSuccess(w io.Writer, msg string) {
	fmt.Fprintf(w, "%s✓ %s%s\n", Green, msg, Reset)
}

// PrintError prints an error message.
func PrintError(w io.Writer, msg string) {
	fmt.Fprintf(w, "%s✗ %s%s\n", Red, msg, Reset)
}

// PrintModelResponse prints a model's response with formatting.
func PrintModelResponse(w io.Writer, model, provider, content string, latency time.Duration) {
	fmt.Fprintf(w, "\n%s┌─ %s (%s) [%.1fs] ─┐%s\n",
		Blue, model, provider, latency.Seconds(), Reset)

	// Print content with indentation
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		fmt.Fprintf(w, "%s│%s %s\n", Blue, Reset, line)
	}
	fmt.Fprintf(w, "%s└─────────────────────────┘%s\n", Blue, Reset)
}

// PrintConsensus prints the consensus response.
func PrintConsensus(w io.Writer, consensus string) {
	fmt.Fprintf(w, "\n%s╔═══ CONSENSUS ═══╗%s\n", BoldGreen, Reset)

	lines := strings.Split(consensus, "\n")
	for _, line := range lines {
		fmt.Fprintf(w, "%s║%s %s\n", Green, Reset, line)
	}
	fmt.Fprintf(w, "%s╚═════════════════╝%s\n", Green, Reset)
}

// PrintSummary prints a summary of the run.
func PrintSummary(w io.Writer, totalModels, successful, failed int, totalTime time.Duration) {
	fmt.Fprintf(w, "\n%s─── Summary ───%s\n", Dim, Reset)
	fmt.Fprintf(w, "Models queried: %d (%s%d succeeded%s, %s%d failed%s)\n",
		totalModels,
		Green, successful, Reset,
		Red, failed, Reset)
	fmt.Fprintf(w, "Total time: %.1fs\n", totalTime.Seconds())
}

// IsTerminal checks if the given file is a terminal.
func IsTerminal(f *os.File) bool {
	stat, _ := f.Stat()
	return (stat.Mode() & os.ModeCharDevice) != 0
}
