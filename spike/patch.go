package main

// A unified diff applier for the "diff" edit format. Hunks are found by
// exact match of their context and removed lines, starting at the stated
// line number and searching outward, because models often get line numbers
// slightly wrong but the text right. Counts in hunk headers are ignored.

import (
	"fmt"
	"strconv"
	"strings"
)

type filePatch struct {
	oldPath, newPath string // "" for /dev/null
	hunks            []hunk
}

type hunk struct {
	oldStart int // 1-based, from the header
	old, new []string
	header   string
}

func parsePatch(text string) ([]filePatch, error) {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	var out []filePatch
	var cur *filePatch
	var h *hunk
	// Empty strings after the patch's final newlines aren't blank
	// context lines.
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	flushHunk := func() {
		if h != nil && cur != nil {
			cur.hunks = append(cur.hunks, *h)
		}
		h = nil
	}
	for i := 0; i < len(lines); i++ {
		l := lines[i]
		switch {
		case strings.HasPrefix(l, "--- ") && i+1 < len(lines) && strings.HasPrefix(lines[i+1], "+++ "):
			flushHunk()
			out = append(out, filePatch{oldPath: patchPath(l[4:]), newPath: patchPath(lines[i+1][4:])})
			cur = &out[len(out)-1]
			i++
		case strings.HasPrefix(l, "@@"):
			if cur == nil {
				return nil, fmt.Errorf("hunk %q before any ---/+++ file header", l)
			}
			flushHunk()
			start, err := hunkStart(l)
			if err != nil {
				return nil, err
			}
			h = &hunk{oldStart: start, header: l}
		case h != nil && strings.HasPrefix(l, "\\"):
			// "\ No newline at end of file": ignored
		case h != nil && strings.HasPrefix(l, "+"):
			h.new = append(h.new, l[1:])
		case h != nil && strings.HasPrefix(l, "-"):
			h.old = append(h.old, l[1:])
		case h != nil && strings.HasPrefix(l, " "):
			h.old = append(h.old, l[1:])
			h.new = append(h.new, l[1:])
		case h != nil && l == "":
			h.old = append(h.old, "")
			h.new = append(h.new, "")
		case strings.HasPrefix(l, "diff "), strings.HasPrefix(l, "index "), strings.HasPrefix(l, "new file mode"),
			strings.HasPrefix(l, "deleted file mode"):
			flushHunk()
		case strings.HasPrefix(l, "rename ") || strings.HasPrefix(l, "similarity index") || strings.HasPrefix(l, "copy "):
			return nil, fmt.Errorf("renames and copies aren't supported; create the new file and delete the old one")
		default:
			if h != nil {
				return nil, fmt.Errorf("line %d: %q doesn't start with ' ', '+' or '-'", i+1, l)
			}
		}
	}
	flushHunk()
	if len(out) == 0 {
		return nil, fmt.Errorf("no file headers (--- a/path, +++ b/path) found")
	}
	for _, fp := range out {
		if fp.oldPath == "" && fp.newPath == "" {
			return nil, fmt.Errorf("a file can't go from /dev/null to /dev/null")
		}
		if len(fp.hunks) == 0 && fp.newPath != "" {
			return nil, fmt.Errorf("%s: no hunks", fp.newPath)
		}
		if fp.oldPath != "" && fp.newPath != "" && fp.oldPath != fp.newPath {
			return nil, fmt.Errorf("renames aren't supported (%s -> %s)", fp.oldPath, fp.newPath)
		}
	}
	return out, nil
}

func patchPath(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\t'); i >= 0 {
		s = s[:i]
	}
	if s == "/dev/null" {
		return ""
	}
	for _, pre := range []string{"a/", "b/"} {
		if strings.HasPrefix(s, pre) {
			return s[2:]
		}
	}
	return s
}

func hunkStart(header string) (int, error) {
	// @@ -12,7 +12,8 @@ optional section
	f := strings.Fields(header)
	if len(f) < 3 || !strings.HasPrefix(f[1], "-") {
		return 0, fmt.Errorf("bad hunk header %q", header)
	}
	n, err := strconv.Atoi(strings.SplitN(f[1][1:], ",", 2)[0])
	if err != nil {
		return 0, fmt.Errorf("bad hunk header %q", header)
	}
	return n, nil
}

// applyHunks applies a file's hunks to its content.
func applyHunks(content string, hunks []hunk) (string, error) {
	trailingNL := strings.HasSuffix(content, "\n")
	lines := strings.Split(content, "\n")
	if trailingNL {
		lines = lines[:len(lines)-1]
	}
	if content == "" {
		lines = nil
	}
	pos := 0 // hunks apply in order, after the previous one
	for _, h := range hunks {
		at := -1
		if len(h.old) == 0 {
			at = h.oldStart // insertion after line oldStart
			if at > len(lines) {
				at = len(lines)
			}
			if at < pos {
				at = pos
			}
		} else {
			at = findBlock(lines, h.old, h.oldStart-1, pos)
		}
		if at < 0 {
			return "", fmt.Errorf("hunk %q: its context and removed lines don't match the file; read the file again and resend the patch", h.header)
		}
		lines = append(lines[:at], append(append([]string{}, h.new...), lines[at+len(h.old):]...)...)
		pos = at + len(h.new)
	}
	out := strings.Join(lines, "\n")
	if trailingNL || (content == "" && len(lines) > 0) {
		out += "\n"
	}
	return out, nil
}

// findBlock returns the start of the match of block in lines nearest to
// hint, at or after min, or -1.
func findBlock(lines, block []string, hint, min int) int {
	match := func(i int) bool {
		if i < min || i+len(block) > len(lines) {
			return false
		}
		for j, b := range block {
			if lines[i+j] != b {
				return false
			}
		}
		return true
	}
	if hint < min {
		hint = min
	}
	for d := 0; d <= len(lines); d++ {
		if match(hint + d) {
			return hint + d
		}
		if d > 0 && match(hint-d) {
			return hint - d
		}
	}
	return -1
}
