package main

// The version 1 tool set (REQUIREMENTS.md M1): list, read, search, edit and
// read test results, plus delete. No shell, no network, no history. The edit
// tool depends on the edit format under test (open decision 9).

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

type toolSpec struct {
	Name        string
	Description string
	Properties  map[string]any
	Required    []string
}

// Outcome of one tool call, for the log and the stats.
type Outcome struct {
	Result  string // sent to the model
	IsError bool
	Status  string // ok, refused, failed (edit didn't apply), error (bad input)
	Kind    string // refusal kind or failure reason
	Path    string
	Edit    bool // an edit attempt, counted per format
}

func str(desc string) map[string]any  { return map[string]any{"type": "string", "description": desc} }
func intg(desc string) map[string]any { return map[string]any{"type": "integer", "description": desc} }
func boolean(desc string) map[string]any {
	return map[string]any{"type": "boolean", "description": desc}
}

func toolSpecs(format string) []toolSpec {
	specs := []toolSpec{
		{Name: "list_files", Description: "List files in the repository. Without recursive, shows the immediate entries of a directory (directories end in /). Files you aren't allowed to read don't appear.",
			Properties: map[string]any{"path": str("Directory relative to the repository root; default the root."), "recursive": boolean("List every file under the directory.")}},
		{Name: "read_file", Description: "Read a text file. Output lines are prefixed with their line number and a tab; the prefix isn't part of the file. Reads up to 2000 lines at a time; use start_line for more.",
			Properties: map[string]any{"path": str("File path relative to the repository root."), "start_line": intg("First line to show, 1-based."), "end_line": intg("Last line to show.")},
			Required:   []string{"path"}},
		{Name: "search", Description: "Search file contents with a regular expression (RE2 syntax). Returns matching lines as path:line: text, at most 200.",
			Properties: map[string]any{"pattern": str("Regular expression."), "path": str("Limit to files under this directory."), "glob": str("Limit to file paths matching this glob, e.g. **/*.java or src/*.py."), "ignore_case": boolean("Match case-insensitively.")},
			Required:   []string{"pattern"}},
		{Name: "read_test_results", Description: "Read the results of the developer's most recent test run from the JUnit XML reports the build wrote: totals, failed tests, messages and stack traces. You can't run tests; ask the developer to run them, then call this.",
			Properties: map[string]any{}},
		{Name: "delete_file", Description: "Delete a file you have read.",
			Properties: map[string]any{"path": str("File path relative to the repository root.")}, Required: []string{"path"}},
	}
	switch format {
	case "replace":
		specs = append(specs,
			toolSpec{Name: "edit_file", Description: "Replace exact text in a file you have read. old_text must match the file exactly (without the line-number prefixes from read_file), including whitespace, and must occur exactly once unless replace_all is set. Include enough surrounding lines to make it unique.",
				Properties: map[string]any{"path": str("File path."), "old_text": str("Exact text to replace."), "new_text": str("Replacement text."), "replace_all": boolean("Replace every occurrence.")},
				Required:   []string{"path", "old_text", "new_text"}},
			toolSpec{Name: "create_file", Description: "Create a new file. Fails if the file exists; use edit_file to change existing files.",
				Properties: map[string]any{"path": str("File path."), "content": str("Full content of the new file.")},
				Required:   []string{"path", "content"}})
	case "diff":
		specs = append(specs,
			toolSpec{Name: "apply_patch", Description: "Apply a unified diff to one or more files. Each file needs ---/+++ headers (a/ and b/ prefixes are optional) and @@ hunks with a few lines of context. Use --- /dev/null to create a file and +++ /dev/null to delete one. Context and removed lines must match the file exactly; line numbers may be approximate. All files apply or none do. Files you change must have been read first.",
				Properties: map[string]any{"patch": str("The unified diff.")},
				Required:   []string{"patch"}})
	case "whole":
		specs = append(specs,
			toolSpec{Name: "write_file", Description: "Write the complete content of a file, creating it or replacing it entirely. To change an existing file, read it first and send the whole new version, unchanged parts included.",
				Properties: map[string]any{"path": str("File path."), "content": str("The complete new content.")},
				Required:   []string{"path", "content"}})
	}
	return specs
}

// Run dispatches one tool call.
func (g *Guard) Run(name string, raw json.RawMessage, reportGlobs []string) Outcome {
	var in struct {
		Path       string `json:"path"`
		Recursive  bool   `json:"recursive"`
		StartLine  int    `json:"start_line"`
		EndLine    int    `json:"end_line"`
		Pattern    string `json:"pattern"`
		Glob       string `json:"glob"`
		IgnoreCase bool   `json:"ignore_case"`
		OldText    string `json:"old_text"`
		NewText    string `json:"new_text"`
		ReplaceAll bool   `json:"replace_all"`
		Content    string `json:"content"`
		Patch      string `json:"patch"`
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		return Outcome{Result: "invalid tool input: " + err.Error(), IsError: true, Status: "error", Kind: "invalid-json"}
	}
	switch name {
	case "list_files":
		return g.list(in.Path, in.Recursive)
	case "read_file":
		return g.readTool(in.Path, in.StartLine, in.EndLine)
	case "search":
		return g.search(in.Pattern, in.Path, in.Glob, in.IgnoreCase)
	case "read_test_results":
		return Outcome{Result: g.TestResults(reportGlobs), Status: "ok"}
	case "delete_file":
		return g.deleteTool(in.Path)
	case "edit_file":
		return g.editReplace(in.Path, in.OldText, in.NewText, in.ReplaceAll)
	case "create_file":
		return g.create(in.Path, in.Content)
	case "apply_patch":
		return g.applyPatch(in.Patch)
	case "write_file":
		return g.writeWhole(in.Path, in.Content)
	}
	return Outcome{Result: "unknown tool " + name, IsError: true, Status: "error", Kind: "unknown-tool"}
}

func refused(r *Refusal, p string, edit bool) Outcome {
	return Outcome{Result: r.Msg, IsError: true, Status: "refused", Kind: r.Kind, Path: p, Edit: edit}
}

func (g *Guard) list(dir string, recursive bool) Outcome {
	d, r := g.clean(dir)
	if r != nil {
		return refused(r, dir, false)
	}
	files, err := g.files()
	if err != nil {
		return Outcome{Result: err.Error(), IsError: true, Status: "error", Kind: "git"}
	}
	prefix := ""
	if d != "." {
		prefix = d + "/"
	}
	seen := map[string]bool{}
	var out []string
	for _, f := range files {
		if !strings.HasPrefix(f, prefix) {
			continue
		}
		rest := strings.TrimPrefix(f, prefix)
		if !recursive {
			if i := strings.IndexByte(rest, '/'); i >= 0 {
				rest = rest[:i+1]
			}
		} else {
			rest = f
		}
		if !seen[rest] {
			seen[rest] = true
			out = append(out, rest)
		}
	}
	if len(out) == 0 {
		return Outcome{Result: dir + ": " + notAvailable + " (or empty)", Status: "ok", Path: d}
	}
	more := ""
	if len(out) > 1000 {
		more = fmt.Sprintf("\n... %d more entries; list a subdirectory", len(out)-1000)
		out = out[:1000]
	}
	return Outcome{Result: strings.Join(out, "\n") + more, Status: "ok", Path: d}
}

func (g *Guard) readTool(p string, start, end int) Outcome {
	content, r := g.Read(p)
	if r != nil {
		return refused(r, p, false)
	}
	lines := strings.Split(content, "\n")
	if strings.HasSuffix(content, "\n") {
		lines = lines[:len(lines)-1]
	}
	if start < 1 {
		start = 1
	}
	if end < 1 || end > len(lines) {
		end = len(lines)
	}
	if end-start >= 2000 {
		end = start + 1999
	}
	var b strings.Builder
	for i := start; i <= end && i <= len(lines); i++ {
		fmt.Fprintf(&b, "%6d\t%s\n", i, lines[i-1])
	}
	if len(lines) == 0 {
		b.WriteString("(empty file)\n")
	} else if end < len(lines) {
		fmt.Fprintf(&b, "... file has %d lines; showing %d-%d\n", len(lines), start, end)
	}
	return Outcome{Result: b.String(), Status: "ok", Path: p}
}

func globRegexp(glob string) (*regexp.Regexp, error) {
	var b strings.Builder
	b.WriteString("^")
	if !strings.Contains(glob, "/") {
		b.WriteString("(.*/)?") // a bare pattern matches a base name anywhere
	}
	for i := 0; i < len(glob); i++ {
		c := glob[i]
		switch {
		case c == '*' && i+1 < len(glob) && glob[i+1] == '*':
			i++
			if i+1 < len(glob) && glob[i+1] == '/' {
				i++
				b.WriteString("(.*/)?")
			} else {
				b.WriteString(".*")
			}
		case c == '*':
			b.WriteString("[^/]*")
		case c == '?':
			b.WriteString("[^/]")
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	b.WriteString("$")
	return regexp.Compile(b.String())
}

func (g *Guard) search(pattern, dir, glob string, ignoreCase bool) Outcome {
	if ignoreCase {
		pattern = "(?i)" + pattern
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return Outcome{Result: "bad pattern: " + err.Error(), IsError: true, Status: "error", Kind: "bad-pattern"}
	}
	var globRe *regexp.Regexp
	if glob != "" {
		if globRe, err = globRegexp(glob); err != nil {
			return Outcome{Result: "bad glob: " + err.Error(), IsError: true, Status: "error", Kind: "bad-glob"}
		}
	}
	d, r := g.clean(dir)
	if r != nil {
		return refused(r, dir, false)
	}
	files, err := g.files()
	if err != nil {
		return Outcome{Result: err.Error(), IsError: true, Status: "error", Kind: "git"}
	}
	var out []string
	total := 0
	for _, f := range files {
		if d != "." && !strings.HasPrefix(f, d+"/") {
			continue
		}
		if globRe != nil && !globRe.MatchString(f) {
			continue
		}
		data, err := g.root.ReadFile(f)
		if err != nil {
			continue
		}
		text := string(data)
		if strings.HasSuffix(strings.ToLower(f), ".ipynb") {
			text = stripNotebookOutputs(data)
		}
		for i, line := range strings.Split(text, "\n") {
			if re.MatchString(line) {
				total++
				if len(out) < 200 {
					out = append(out, fmt.Sprintf("%s:%d: %s", f, i+1, truncate(line, 300)))
				}
			}
		}
	}
	if total == 0 {
		return Outcome{Result: "no matches", Status: "ok"}
	}
	res := strings.Join(out, "\n")
	if total > len(out) {
		res += fmt.Sprintf("\n... %d matches in all; narrow the search", total)
	}
	return Outcome{Result: res, Status: "ok"}
}

func (g *Guard) deleteTool(p string) Outcome {
	p, r := g.clean(p)
	if r != nil {
		return refused(r, p, true)
	}
	exists, r := g.checkWrite(p)
	if r != nil {
		return refused(r, p, true)
	}
	if !exists {
		return refused(unavailable("missing", p), p, true)
	}
	if err := g.remove(p); err != nil {
		return Outcome{Result: err.Error(), IsError: true, Status: "error", Kind: "io", Path: p, Edit: true}
	}
	return Outcome{Result: "deleted " + p, Status: "ok", Path: p, Edit: true}
}

func (g *Guard) finishWrite(p, old, content, verb string) Outcome {
	if r := reviewable(p, old, content); r != nil {
		return refused(r, p, true)
	}
	if err := g.write(p, content); err != nil {
		return Outcome{Result: err.Error(), IsError: true, Status: "error", Kind: "io", Path: p, Edit: true}
	}
	return Outcome{Result: verb + " " + p, Status: "ok", Path: p, Edit: true}
}

func (g *Guard) editReplace(p, oldText, newText string, all bool) Outcome {
	p, r := g.clean(p)
	if r != nil {
		return refused(r, p, true)
	}
	content, exists, r := g.current(p)
	if r != nil {
		return refused(r, p, true)
	}
	if !exists {
		return refused(unavailable("missing", p), p, true)
	}
	failed := func(kind, msg string) Outcome {
		return Outcome{Result: msg, IsError: true, Status: "failed", Kind: kind, Path: p, Edit: true}
	}
	if oldText == "" {
		return failed("empty-old-text", "old_text is empty")
	}
	if oldText == newText {
		return failed("no-change", "old_text and new_text are the same")
	}
	n := strings.Count(content, oldText)
	switch {
	case n == 0:
		return failed("not-found", p+": old_text not found. It must match the file exactly, without line-number prefixes; read the file again.")
	case n > 1 && !all:
		return failed("not-unique", fmt.Sprintf("%s: old_text occurs %d times; add surrounding lines to make it unique, or set replace_all", p, n))
	}
	var updated string
	if all {
		updated = strings.ReplaceAll(content, oldText, newText)
	} else {
		updated = strings.Replace(content, oldText, newText, 1)
	}
	return g.finishWrite(p, content, updated, "edited")
}

func (g *Guard) create(p, content string) Outcome {
	p, r := g.clean(p)
	if r != nil {
		return refused(r, p, true)
	}
	exists, r := g.checkWrite(p)
	if exists && r == nil {
		return Outcome{Result: p + ": already exists; use edit_file", IsError: true, Status: "failed", Kind: "exists", Path: p, Edit: true}
	}
	if r != nil {
		return refused(r, p, true)
	}
	return g.finishWrite(p, "", content, "created")
}

func (g *Guard) writeWhole(p, content string) Outcome {
	p, r := g.clean(p)
	if r != nil {
		return refused(r, p, true)
	}
	old, exists, r := g.current(p)
	if r != nil {
		return refused(r, p, true)
	}
	verb := "wrote"
	if !exists {
		verb = "created"
	}
	return g.finishWrite(p, old, content, verb)
}

func (g *Guard) applyPatch(patch string) Outcome {
	failed := func(kind, msg string) Outcome {
		return Outcome{Result: msg, IsError: true, Status: "failed", Kind: kind, Edit: true}
	}
	fps, err := parsePatch(patch)
	if err != nil {
		return failed("parse", "patch not applied: "+err.Error())
	}
	type change struct {
		path, old, content string
		del                bool
	}
	var changes []change
	for _, fp := range fps {
		target := fp.newPath
		if target == "" {
			target = fp.oldPath
		}
		p, r := g.clean(target)
		if r != nil {
			return refused(r, target, true)
		}
		content, exists, r := g.current(p)
		if r != nil {
			return refused(r, p, true)
		}
		switch {
		case fp.oldPath == "" && exists:
			return failed("exists", p+": patch creates it, but it already exists")
		case fp.oldPath != "" && !exists:
			return refused(unavailable("missing", p), p, true)
		case fp.newPath == "":
			changes = append(changes, change{path: p, del: true})
			continue
		}
		updated, err := applyHunks(content, fp.hunks)
		if err != nil {
			o := failed("hunk-mismatch", fmt.Sprintf("patch not applied: %s: %v", p, err))
			o.Path = p
			return o
		}
		if r := reviewable(p, content, updated); r != nil {
			return refused(r, p, true)
		}
		changes = append(changes, change{path: p, old: content, content: updated})
	}
	var done []string
	for _, c := range changes {
		if c.del {
			if err := g.remove(c.path); err != nil {
				return Outcome{Result: fmt.Sprintf("deleting %s failed after applying %v: %v", c.path, done, err), IsError: true, Status: "error", Kind: "io", Edit: true}
			}
		} else if err := g.write(c.path, c.content); err != nil {
			return Outcome{Result: fmt.Sprintf("writing %s failed after applying %v: %v", c.path, done, err), IsError: true, Status: "error", Kind: "io", Edit: true}
		}
		done = append(done, c.path)
	}
	return Outcome{Result: "applied to " + strings.Join(done, ", "), Status: "ok", Path: strings.Join(done, ","), Edit: true}
}
