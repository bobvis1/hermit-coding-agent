// Command spike is the throwaway prototype for IMPLEMENTATION_PLAN.md's
// spike: is a coding agent with no shell, no test runs, no history and no
// access to manifests or ignored files still worth using?
//
// It never merges into main. Run it only on the spike's public repositories.
//
//	HERMIT_SPIKE_ANTHROPIC_KEY="$(security find-generic-password -s hermit-spike-anthropic -w)" \
//	  ./spike -repo ~/src/project -format replace -task 1 -who alice [-task-file task1.md]
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go/v2"
	"github.com/anthropics/anthropic-sdk-go/v2/option"
)

const systemPrompt = `You are a coding agent working in a developer's git repository. You work only through the tools provided, and the developer reviews every change you make in their working tree before committing it.

Limits, by design:
- You can't run commands, builds or tests. When a change needs testing, say which tests to run (and the command, if you know it), stop, and ask the developer to run them; then read the results with read_test_results.
- You can't see git history.
- Some files are protected: build and CI configuration, dependency manifests and lockfiles, tool and IDE settings, git internals, and instructions for other AI tools. You can read them but not change them. When a change needs one, describe the exact change for the developer to make.
- Some files aren't available at all: git-ignored files (build output, installed dependencies, generated code, local configuration), secrets and keys, and binary files. They don't appear in listings or search. If you need something from one, ask the developer to paste the relevant part.
- Don't add dependencies. If a change needs a new package, describe it (package, version, purpose) for the developer to decide on, rather than writing code that assumes it is installed.
- You never commit or push.

Working style:
- Read a file before changing it. If an edit is refused because the file changed since you read it, the developer has edited it: read it again.
- For a change that spans several files, start with a short plan, then make the changes.
- When you finish, summarize what you changed and what the developer should run or check.`

type logger struct {
	f *os.File
}

func (l *logger) event(kind string, fields map[string]any) {
	if fields == nil {
		fields = map[string]any{}
	}
	fields["ts"] = time.Now().Format(time.RFC3339Nano)
	fields["event"] = kind
	b, _ := json.Marshal(fields)
	l.f.Write(append(b, '\n'))
}

type stats struct {
	start       time.Time
	modelTurns  int
	apiTime     time.Duration
	inTokens    int64
	outTokens   int64
	cacheRead   int64
	cacheWrite  int64
	toolCalls   map[string]int
	refusals    map[string]int
	edits       int
	editsFailed map[string]int
	devMessages int
	models      map[string]int
	refusedTurn int
}

func main() {
	repo := flag.String("repo", ".", "repository to work in")
	format := flag.String("format", "", "edit format under test: replace, diff or whole")
	task := flag.String("task", "", "task number from the spike protocol")
	who := flag.String("who", "", "participant identifier")
	taskFile := flag.String("task-file", "", "file holding the task text, sent as the first message")
	model := flag.String("model", "claude-opus-5-5", "model")
	effort := flag.String("effort", "high", "effort: low, medium, high, xhigh or max")
	logDir := flag.String("log-dir", filepath.Join(os.Getenv("HOME"), "hermit-spike-logs"), "where session logs go")
	reports := flag.String("reports", "", "extra comma-separated globs for JUnit XML reports")
	flag.Parse()

	if *format != "replace" && *format != "diff" && *format != "whole" {
		fatal("-format must be replace, diff or whole")
	}
	if *task == "" || *who == "" {
		fatal("-task and -who are required, so the session can be matched to the recording sheet")
	}
	key := os.Getenv("HERMIT_SPIKE_ANTHROPIC_KEY")
	if key == "" {
		fatal("HERMIT_SPIKE_ANTHROPIC_KEY is not set; see the command in the package comment")
	}
	guard, err := NewGuard(*repo)
	if err != nil {
		fatal(err.Error())
	}
	var reportGlobs []string
	if *reports != "" {
		reportGlobs = strings.Split(*reports, ",")
	}

	if err := os.MkdirAll(*logDir, 0o700); err != nil {
		fatal(err.Error())
	}
	logName := filepath.Join(*logDir, fmt.Sprintf("%s-task%s-%s-%s.jsonl", time.Now().Format("20060102-150405"), *task, *who, *format))
	lf, err := os.OpenFile(logName, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o600)
	if err != nil {
		fatal(err.Error())
	}
	defer lf.Close()
	log := &logger{f: lf}
	head, _ := gitOut(guard.dir, "rev-parse", "HEAD")
	dirty, _ := gitOut(guard.dir, "status", "--porcelain")
	log.event("session_start", map[string]any{"task": *task, "who": *who, "format": *format, "model": *model,
		"effort": *effort, "repo": guard.dir, "head": strings.TrimSpace(head), "dirty_at_start": dirty != ""})

	client := anthropic.NewClient(option.WithAPIKey(key), option.WithMaxRetries(3))

	var tools []anthropic.BetaToolUnionParam
	for _, s := range toolSpecs(*format) {
		props := s.Properties
		if props == nil {
			props = map[string]any{}
		}
		t := anthropic.BetaToolParam{
			Name:                s.Name,
			Description:         anthropic.String(s.Description),
			InputSchema:         anthropic.BetaToolInputSchemaParam{Properties: props, Required: s.Required},
			EagerInputStreaming: anthropic.Bool(true),
		}
		tools = append(tools, anthropic.BetaToolUnionParam{OfTool: &t})
	}

	st := &stats{start: time.Now(), toolCalls: map[string]int{}, refusals: map[string]int{},
		editsFailed: map[string]int{}, models: map[string]int{}}
	var messages []anthropic.BetaMessageParam

	interrupts := make(chan os.Signal, 1)
	signal.Notify(interrupts, os.Interrupt)

	in := bufio.NewReader(os.Stdin)
	first := ""
	if *taskFile != "" {
		b, err := os.ReadFile(*taskFile)
		if err != nil {
			fatal(err.Error())
		}
		first = strings.TrimSpace(string(b))
		fmt.Printf("Task (from %s):\n%s\n", *taskFile, first)
	}
	fmt.Printf("\nHermit spike: task %s, format %s, %s. Log: %s\n", *task, *format, *model, logName)
	fmt.Println(`Type a message and press Enter. For several lines, type """ alone on a line, then the text, then """ again.`)
	fmt.Println(`Commands: /stats, /done (end the task). Ctrl-C stops the agent's current turn.`)

	finish := func() {
		summary := st.summary(*format)
		log.event("session_end", summary)
		fmt.Println()
		printSummary(summary)
		fmt.Println("Log written to", logName)
	}

	for {
		var text string
		if first != "" {
			text, first = first, ""
		} else {
			var ok bool
			text, ok = prompt(in, interrupts)
			if !ok {
				finish()
				return
			}
		}
		switch strings.TrimSpace(text) {
		case "":
			continue
		case "/done", "/quit", "/exit":
			finish()
			return
		case "/stats":
			printSummary(st.summary(*format))
			continue
		}
		st.devMessages++
		log.event("developer_message", map[string]any{"chars": len(text), "text": text})
		block := anthropic.NewBetaTextBlock(text)
		if n := len(messages); n > 0 && messages[n-1].OfUser != nil {
			// The last turn was interrupted: add to the waiting user message.
			messages[n-1].OfUser.Content = append(messages[n-1].OfUser.Content, block)
		} else {
			messages = append(messages, anthropic.NewBetaUserMessage(block))
		}
		messages = runTurn(&client, *model, *effort, tools, messages, guard, reportGlobs, log, st, interrupts)
	}
}

// prompt reads one message; ok is false at end of input or on Ctrl-C.
func prompt(in *bufio.Reader, interrupts chan os.Signal) (string, bool) {
	type line struct {
		s   string
		err error
	}
	read := func() (string, error, bool) {
		ch := make(chan line, 1)
		go func() {
			s, err := in.ReadString('\n')
			ch <- line{s, err}
		}()
		select {
		case l := <-ch:
			return l.s, l.err, true
		case <-interrupts:
			return "", nil, false
		}
	}
	fmt.Print("\n> ")
	s, err, ok := read()
	if !ok || (err != nil && s == "") {
		return "", false
	}
	if strings.TrimSpace(s) != `"""` {
		return strings.TrimRight(s, "\r\n"), true
	}
	var b strings.Builder
	for {
		s, err, ok := read()
		if !ok {
			return "", false
		}
		if strings.TrimSpace(s) == `"""` || (err != nil && s == "") {
			break
		}
		b.WriteString(s)
		if err != nil {
			break
		}
	}
	return strings.TrimRight(b.String(), "\r\n"), true
}

// runTurn runs the agent until it stops calling tools, or is interrupted.
func runTurn(client *anthropic.Client, model, effort string, tools []anthropic.BetaToolUnionParam,
	messages []anthropic.BetaMessageParam, guard *Guard, reportGlobs []string, log *logger, st *stats,
	interrupts chan os.Signal) []anthropic.BetaMessageParam {

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stopWatch := make(chan struct{})
	defer close(stopWatch)
	go func() {
		select {
		case <-interrupts:
			fmt.Println("\n[interrupted]")
			cancel()
		case <-stopWatch:
		}
	}()

	for step := 0; step < 100; step++ {
		params := anthropic.BetaMessageNewParams{
			Model:        anthropic.Model(model),
			MaxTokens:    64000,
			System:       []anthropic.BetaTextBlockParam{{Text: systemPrompt}},
			Messages:     messages,
			Tools:        tools,
			Thinking:     anthropic.BetaThinkingConfigParamUnion{OfAdaptive: &anthropic.BetaThinkingConfigAdaptiveParam{Display: anthropic.BetaThinkingConfigAdaptiveDisplaySummarized}},
			OutputConfig: anthropic.BetaOutputConfigParam{Effort: anthropic.BetaOutputConfigEffort(effort)},
			CacheControl: anthropic.BetaCacheControlEphemeralParam{},
			Fallbacks:    anthropic.BetaFallbacksParamOfDefault(),
			Betas:        []anthropic.AnthropicBeta{anthropic.AnthropicBetaServerSideFallback2026_07_01},
		}
		t0 := time.Now()
		stream := client.Beta.Messages.NewStreaming(ctx, params)
		msg := anthropic.BetaMessage{}
		thinking := false
		for stream.Next() {
			ev := stream.Current()
			if err := msg.Accumulate(ev); err != nil {
				break
			}
			if d, ok := ev.AsAny().(anthropic.BetaRawContentBlockDeltaEvent); ok {
				switch dv := d.Delta.AsAny().(type) {
				case anthropic.BetaTextDelta:
					if thinking {
						fmt.Print("\x1b[0m\n")
						thinking = false
					}
					fmt.Print(stripEscapes(dv.Text))
				case anthropic.BetaThinkingDelta:
					if !thinking {
						fmt.Print("\x1b[2m")
						thinking = true
					}
					fmt.Print(stripEscapes(dv.Thinking))
				}
			}
		}
		if thinking {
			fmt.Print("\x1b[0m")
		}
		elapsed := time.Since(t0)
		if err := stream.Err(); err != nil {
			if errors.Is(err, context.Canceled) || ctx.Err() != nil {
				log.event("interrupted", map[string]any{"step": step})
				return messages
			}
			fmt.Printf("\n[API error: %v]\n", err)
			log.event("api_error", map[string]any{"error": err.Error()})
			return messages
		}
		st.modelTurns++
		st.apiTime += elapsed
		st.inTokens += msg.Usage.InputTokens
		st.outTokens += msg.Usage.OutputTokens
		st.cacheRead += msg.Usage.CacheReadInputTokens
		st.cacheWrite += msg.Usage.CacheCreationInputTokens
		st.models[string(msg.Model)]++
		var texts []string
		for _, b := range msg.Content {
			if t, ok := b.AsAny().(anthropic.BetaTextBlock); ok {
				texts = append(texts, t.Text)
			}
		}
		log.event("model_turn", map[string]any{"n": st.modelTurns, "ms": elapsed.Milliseconds(), "stop_reason": msg.StopReason,
			"model": msg.Model, "input_tokens": msg.Usage.InputTokens, "output_tokens": msg.Usage.OutputTokens,
			"cache_read": msg.Usage.CacheReadInputTokens, "cache_write": msg.Usage.CacheCreationInputTokens,
			"text": strings.Join(texts, "\n")})

		// The model's turn goes back exactly as received, thinking included.
		messages = append(messages, msg.ToParam())

		if msg.StopReason == anthropic.BetaStopReasonRefusal {
			st.refusedTurn++
			fmt.Println("\n[the model declined this request]")
		}
		if msg.StopReason != anthropic.BetaStopReasonToolUse {
			if msg.StopReason == anthropic.BetaStopReasonMaxTokens {
				fmt.Println("\n[response hit the output limit]")
			}
			fmt.Println()
			return messages
		}

		var results []anthropic.BetaContentBlockParamUnion
		for _, b := range msg.Content {
			tu, ok := b.AsAny().(anthropic.BetaToolUseBlock)
			if !ok {
				continue
			}
			raw := json.RawMessage(tu.JSON.Input.Raw())
			t1 := time.Now()
			out := Outcome{Result: "no such tool: " + tu.Name, IsError: true, Status: "error", Kind: "unknown-tool"}
			for _, t := range tools {
				if t.OfTool != nil && t.OfTool.Name == tu.Name {
					out = guard.Run(tu.Name, raw, reportGlobs)
				}
			}
			st.toolCalls[tu.Name]++
			if out.Status == "refused" {
				st.refusals[out.Kind]++
			}
			if out.Edit && out.Status != "refused" {
				st.edits++
				if out.Status != "ok" {
					st.editsFailed[out.Kind]++
				}
			}
			mark := "ok"
			if out.Status != "ok" {
				mark = out.Status + ": " + out.Kind
			}
			fmt.Printf("\n  \x1b[36m→ %s %s\x1b[0m [%s]", tu.Name, describe(tu.Name, raw), mark)
			log.event("tool_call", map[string]any{"tool": tu.Name, "status": out.Status, "kind": out.Kind,
				"path": out.Path, "edit": out.Edit, "ms": time.Since(t1).Milliseconds(),
				"detail": failureDetail(out)})
			results = append(results, anthropic.NewBetaToolResultBlock(tu.ID, out.Result, out.IsError))
		}
		fmt.Println()
		messages = append(messages, anthropic.NewBetaUserMessage(results...))
		if ctx.Err() != nil {
			log.event("interrupted", map[string]any{"step": step, "after_tools": true})
			return messages
		}
	}
	fmt.Println("\n[stopped after 100 model calls in one turn]")
	return messages
}

func failureDetail(o Outcome) string {
	if o.Status == "ok" {
		return ""
	}
	return truncate(o.Result, 500)
}

func describe(tool string, raw json.RawMessage) string {
	var in map[string]any
	if json.Unmarshal(raw, &in) != nil {
		return "(invalid input)"
	}
	switch tool {
	case "search":
		return fmt.Sprintf("%q %v", in["pattern"], nonEmpty(in["path"], in["glob"]))
	case "apply_patch":
		p, _ := in["patch"].(string)
		var files []string
		for _, l := range strings.Split(p, "\n") {
			if strings.HasPrefix(l, "+++ ") {
				files = append(files, strings.TrimPrefix(strings.TrimPrefix(l[4:], "b/"), " "))
			}
		}
		return strings.Join(files, ", ")
	}
	if p, ok := in["path"].(string); ok {
		return p
	}
	return ""
}

func nonEmpty(vals ...any) string {
	var out []string
	for _, v := range vals {
		if s, ok := v.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return strings.Join(out, " ")
}

// stripEscapes removes terminal control sequences from model text.
func stripEscapes(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return r
		}
		if r < 0x20 || r == 0x7f || (r >= 0x80 && r < 0xa0) || (r >= 0x202a && r <= 0x202e) || (r >= 0x2066 && r <= 0x2069) {
			return -1
		}
		return r
	}, s)
}

func (st *stats) summary(format string) map[string]any {
	failed := 0
	for _, n := range st.editsFailed {
		failed += n
	}
	rate := 0.0
	if st.edits > 0 {
		rate = float64(failed) / float64(st.edits)
	}
	return map[string]any{
		"elapsed_s": int(time.Since(st.start).Seconds()), "api_s": int(st.apiTime.Seconds()),
		"model_turns": st.modelTurns, "developer_messages": st.devMessages,
		"input_tokens": st.inTokens, "output_tokens": st.outTokens, "cache_read": st.cacheRead, "cache_write": st.cacheWrite,
		"tool_calls": st.toolCalls, "refusals": st.refusals,
		"edit_format": format, "edits_attempted": st.edits, "edits_failed": failed, "edit_failure_rate": rate,
		"edit_failures_by_reason": st.editsFailed, "models": st.models, "model_refusals": st.refusedTurn,
	}
}

func printSummary(s map[string]any) {
	keys := make([]string, 0, len(s))
	for k := range s {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	fmt.Println("Session so far:")
	for _, k := range keys {
		fmt.Printf("  %-24s %v\n", k, s[k])
	}
}

func fatal(msg string) {
	fmt.Fprintln(os.Stderr, "spike:", msg)
	os.Exit(2)
}
