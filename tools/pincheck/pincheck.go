// Package pincheck checks that GitHub Actions workflows refer to every
// third-party action by a full commit hash, never a tag or branch
// (IMPLEMENTATION_PLAN.md, milestone 0).
package pincheck

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Violation is a "uses:" that isn't pinned.
type Violation struct {
	File string
	Line int
	Ref  string
}

func (v Violation) String() string {
	return fmt.Sprintf("%s:%d: %q is not pinned to a full commit hash", v.File, v.Line, v.Ref)
}

var (
	usesLine = regexp.MustCompile(`^\s*(?:-\s*)?uses\s*:\s*(.*)$`)
	pinned   = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+(/[^@\s]*)?@[0-9a-f]{40}$`)
	docker   = regexp.MustCompile(`^docker://[^@\s]+@sha256:[0-9a-f]{64}$`)
)

// CheckFile checks one workflow or action file's contents.
func CheckFile(name string, data []byte) []Violation {
	var vs []Violation
	for i, line := range strings.Split(string(data), "\n") {
		m := usesLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		ref := strings.TrimSpace(m[1])
		if j := strings.Index(ref, " #"); j >= 0 {
			ref = strings.TrimSpace(ref[:j])
		}
		ref = strings.Trim(ref, `"'`)
		if strings.HasPrefix(ref, "./") || pinned.MatchString(ref) || docker.MatchString(ref) {
			continue
		}
		vs = append(vs, Violation{File: name, Line: i + 1, Ref: ref})
	}
	return vs
}

// CheckDir checks every workflow and action file under dir (normally .github).
func CheckDir(dir string) ([]Violation, error) {
	var vs []Violation
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		ext := filepath.Ext(path)
		if d.IsDir() || (ext != ".yml" && ext != ".yaml") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		vs = append(vs, CheckFile(filepath.ToSlash(path), data)...)
		return nil
	})
	return vs, err
}
