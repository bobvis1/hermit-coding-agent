package depcheck

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

var allGOOS = []string{"darwin", "linux", "windows"}

func run(t *testing.T, fixture string) []Violation {
	t.Helper()
	vs, err := Run(Config{Dir: filepath.Join("testdata", fixture), GOOS: allGOOS, ApprovedFile: filepath.Join("testdata", "approved.txt")})
	if err != nil {
		t.Fatalf("Run(%s): %v", fixture, err)
	}
	return vs
}

// pairs returns the distinct "rule package" pairs, with the fixture's module
// path shortened, for comparing with an expected list.
func pairs(vs []Violation) []string {
	seen := map[string]bool{}
	var out []string
	for _, v := range vs {
		k := v.Rule + " " + strings.TrimPrefix(v.Pkg, "example.com/hermit/")
		if !seen[k] {
			seen[k] = true
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

func compare(t *testing.T, vs []Violation, want []string) {
	t.Helper()
	got := pairs(vs)
	sort.Strings(want)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("violations:\n  %s\nwant:\n  %s\nfull report:", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
		for _, v := range vs {
			t.Log(v)
		}
	}
}

// Every rule's allowed case passes: gitcmd runs git, fsguard reads files,
// netclient imports net/http, tools builds a tooldef.Set, tui approves MR
// text, provider derives a Set it was given, and test files are exempt.
func TestAllowedCases(t *testing.T) {
	compare(t, run(t, "good"), nil)
}

// Each first-party rule fails on a deliberate violation.
func TestFirstPartyViolations(t *testing.T) {
	compare(t, run(t, "bad"), []string{
		"process internal/tools",             // imports os/exec
		"process internal/loop",              // os.StartProcess
		"filesystem internal/loop",           // os.ReadFile, os.StartProcess
		"constructor internal/loop",          // tooldef.NewSet, render.Approve through a variable
		"filesystem internal/scan",           // io/ioutil
		"cgo internal/scan",                  // import "C"
		"filesystem internal/tui",            // filepath.WalkDir
		"constructor internal/tui",           // tooldef.Set{} literal
		"constructor internal/scm",           // conversion to render.ApprovedMRText
		"linkname internal/scm",              // //go:linkname
		"syscall internal/scm",               // unsafe
		"syscall internal/render",            // syscall
		"self-contained internal/render",     // imports tooldef
		"provider-imports internal/provider", // reaches tools and fsguard
		"net-http internal/audit",            // net/http outside netclient
		"plugin internal/worktree",           // plugin
	})
}

// Third-party code linked into the module may not start a process either,
// by os/exec, by syscall, or by naming a function for purego to load. A
// package used only by tests is not linked, so it doesn't count.
func TestThirdPartyViolations(t *testing.T) {
	vs := run(t, "thirdparty")
	compare(t, vs, []string{
		"process example.com/evil",                    // imports os/exec
		"process example.com/evil/spawn",              // syscall.StartProcess
		"process internal/credstore",                  // purego.Dlsym(..., "posix_spawn")
		"process github.com/aws/aws-sdk-go-v2/config", // forbidden package
		"syscall internal/worktree",                   // purego outside credstore
		"approved-modules go.mod",                     // example.com/evil, replace directives
		"vendor go.mod",                               // requirements but no vendor directory
	})
	for _, v := range vs {
		if strings.Contains(v.Pkg, "testonly") {
			t.Errorf("test-only dependency reported: %v", v)
		}
		if v.Pkg == "example.com/hermit/internal/credstore" && strings.Contains(v.Msg, "SecItemAdd") {
			t.Errorf("SecItemAdd reported: %v", v)
		}
	}
}

func TestReadApproved(t *testing.T) {
	dir := t.TempDir()
	write := func(s string) string {
		name := filepath.Join(dir, "approved.txt")
		if err := os.WriteFile(name, []byte(s), 0o600); err != nil {
			t.Fatal(err)
		}
		return name
	}
	a, err := ReadApproved(write("# comment\n\ngolang.org/x/sys BSD-3-Clause -- notes here\nexample.com/a  MIT AND Apache-2.0\n"))
	if err != nil {
		t.Fatal(err)
	}
	if a["golang.org/x/sys"] != "BSD-3-Clause" || a["example.com/a"] != "MIT AND Apache-2.0" || len(a) != 2 {
		t.Errorf("got %v", a)
	}
	for _, bad := range []string{
		"example.com/gpl GPL-3.0\n",
		"example.com/nolicense\n",
		"example.com/a MIT\nexample.com/a MIT\n",
	} {
		if _, err := ReadApproved(write(bad)); err == nil {
			t.Errorf("%q: accepted", bad)
		}
	}
	if _, err := ReadApproved(filepath.Join(dir, "missing.txt")); err == nil {
		t.Error("missing list accepted")
	}
}

// The repository's own approved list parses.
func TestRepositoryApprovedList(t *testing.T) {
	if _, err := ReadApproved(filepath.Join("..", "..", "deps", "approved-modules.txt")); err != nil {
		t.Fatal(err)
	}
}

// A module that fails to load is an error, never a pass.
func TestLoadErrorFails(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/broken\n\ngo 1.25.0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "x.go"), []byte("package x\n\nvar _ = undefined\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(Config{Dir: dir, GOOS: []string{"linux"}, ApprovedFile: filepath.Join("testdata", "approved.txt")}); err == nil {
		t.Error("broken module passed")
	}
}
