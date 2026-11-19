package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fixture(t *testing.T) *Guard {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		".gitignore":                   "target/\nbuild/\n*.log\n",
		"pom.xml":                      "<project/>\n",
		"package.json":                 "{}\n",
		".github/workflows/ci.yml":     "on: push\n",
		".env":                         "SECRET=hunter2\n",
		"src/main/App.java":            "class App {\n  int a = 1;\n  int b = 2;\n  int a2 = 1;\n}\n",
		"src/util.py":                  "def f():\n    return 1\n\n\ndef g():\n    return 2\n",
		"README.md":                    "# readme\n",
		"run.sh":                       "#!/bin/sh\necho hi\n",
		"target/generated/Client.java": "class Client {}\n",
		"debug.log":                    "hunter2 in a log\n",
	}
	for p, c := range files {
		full := filepath.Join(dir, p)
		os.MkdirAll(filepath.Dir(full), 0o755)
		if err := os.WriteFile(full, []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	os.Chmod(filepath.Join(dir, "run.sh"), 0o755)
	os.WriteFile(filepath.Join(dir, "data.bin"), []byte{0x50, 0x4b, 3, 4, 0, 1}, 0o644)
	for _, args := range [][]string{{"init", "-q"}, {"add", "."}, {"-c", "user.email=t@e", "-c", "user.name=t", "commit", "-qm", "init"}} {
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	g, err := NewGuard(dir)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func run(t *testing.T, g *Guard, tool string, in map[string]any) Outcome {
	t.Helper()
	b, _ := json.Marshal(in)
	return g.Run(tool, b, nil)
}

func want(t *testing.T, o Outcome, status, kind string) {
	t.Helper()
	if o.Status != status || (kind != "" && o.Kind != kind) {
		t.Errorf("got %s/%s (%q), want %s/%s", o.Status, o.Kind, o.Result, status, kind)
	}
}

func TestListingHidesUnavailable(t *testing.T) {
	g := fixture(t)
	o := run(t, g, "list_files", map[string]any{"recursive": true})
	for _, hidden := range []string{".env", "target/", "debug.log", "data.bin"} {
		if strings.Contains(o.Result, hidden) {
			t.Errorf("listing shows %s:\n%s", hidden, o.Result)
		}
	}
	for _, shown := range []string{"pom.xml", "src/main/App.java", ".github/workflows/ci.yml"} {
		if !strings.Contains(o.Result, shown) {
			t.Errorf("listing lacks %s", shown)
		}
	}
	if o := run(t, g, "search", map[string]any{"pattern": "hunter2"}); o.Result != "no matches" {
		t.Errorf("search found unavailable content: %s", o.Result)
	}
	if o := run(t, g, "search", map[string]any{"pattern": "int a", "glob": "**/*.java"}); !strings.Contains(o.Result, "src/main/App.java:2:") {
		t.Errorf("search: %s", o.Result)
	}
}

func TestReadRefusalsLookLikeMissing(t *testing.T) {
	g := fixture(t)
	for _, p := range []string{".env", "target/generated/Client.java", "debug.log", "data.bin", "nope.txt", "../outside", ".git/config"} {
		o := run(t, g, "read_file", map[string]any{"path": p})
		if o.Status != "refused" || !strings.HasSuffix(o.Result, notAvailable) {
			t.Errorf("%s: %s %q", p, o.Status, o.Result)
		}
	}
	want(t, run(t, g, "read_file", map[string]any{"path": "pom.xml"}), "ok", "")
}

func TestProtectedAndIgnoredWrites(t *testing.T) {
	g := fixture(t)
	run(t, g, "read_file", map[string]any{"path": "pom.xml"})
	want(t, run(t, g, "write_file", map[string]any{"path": "pom.xml", "content": "x"}), "refused", "protected")
	want(t, run(t, g, "write_file", map[string]any{"path": ".github/workflows/new.yml", "content": "x"}), "refused", "protected")
	want(t, run(t, g, "create_file", map[string]any{"path": "Package.JSON", "content": "x"}), "refused", "protected")
	want(t, run(t, g, "write_file", map[string]any{"path": "build/x.txt", "content": "x"}), "refused", "ignored")
	want(t, run(t, g, "write_file", map[string]any{"path": ".env.local", "content": "x"}), "refused", "unreadable")
	want(t, run(t, g, "write_file", map[string]any{"path": ".git/hooks/pre-commit", "content": "x"}), "refused", "unreadable")
	want(t, run(t, g, "write_file", map[string]any{"path": "src/new.py", "content": "x = 1\n"}), "ok", "")
}

func TestStaleAndUnreadEdits(t *testing.T) {
	g := fixture(t)
	edit := map[string]any{"path": "src/util.py", "old_text": "return 1", "new_text": "return 10"}
	want(t, run(t, g, "edit_file", edit), "refused", "not-read")
	run(t, g, "read_file", map[string]any{"path": "src/util.py"})
	os.WriteFile(filepath.Join(g.dir, "src/util.py"), []byte("def f():\n    return 1  # developer\n"), 0o644)
	want(t, run(t, g, "edit_file", edit), "refused", "stale")
	run(t, g, "read_file", map[string]any{"path": "src/util.py"})
	want(t, run(t, g, "edit_file", edit), "ok", "")
	// The agent's own write doesn't make its next edit stale.
	want(t, run(t, g, "edit_file", map[string]any{"path": "src/util.py", "old_text": "return 10", "new_text": "return 11"}), "ok", "")
}

func TestReplaceFailures(t *testing.T) {
	g := fixture(t)
	run(t, g, "read_file", map[string]any{"path": "src/main/App.java"})
	want(t, run(t, g, "edit_file", map[string]any{"path": "src/main/App.java", "old_text": "     2\t  int b = 2;", "new_text": "x"}), "failed", "not-found")
	want(t, run(t, g, "edit_file", map[string]any{"path": "src/main/App.java", "old_text": "= 1;", "new_text": "= 5;"}), "failed", "not-unique")
	want(t, run(t, g, "edit_file", map[string]any{"path": "src/main/App.java", "old_text": "= 1;", "new_text": "= 5;", "replace_all": true}), "ok", "")
}

func TestPatchFormat(t *testing.T) {
	g := fixture(t)
	run(t, g, "read_file", map[string]any{"path": "src/util.py"})
	run(t, g, "read_file", map[string]any{"path": "src/main/App.java"})
	// Line numbers off by several lines still apply; context must match.
	patch := "--- a/src/util.py\n+++ b/src/util.py\n@@ -9,2 +9,2 @@\n def g():\n-    return 2\n+    return 3\n" +
		"--- /dev/null\n+++ b/src/new_test.py\n@@ -0,0 +1,2 @@\n+def test_g():\n+    assert True\n"
	want(t, run(t, g, "apply_patch", map[string]any{"patch": patch}), "ok", "")
	got, _ := os.ReadFile(filepath.Join(g.dir, "src/util.py"))
	if !strings.Contains(string(got), "return 3") || !strings.Contains(string(got), "return 1") {
		t.Errorf("util.py after patch:\n%s", got)
	}
	if b, _ := os.ReadFile(filepath.Join(g.dir, "src/new_test.py")); string(b) != "def test_g():\n    assert True\n" {
		t.Errorf("new file: %q", b)
	}
	// A bad hunk in the second file leaves the first untouched.
	bad := "--- a/src/main/App.java\n+++ b/src/main/App.java\n@@ -2,1 +2,1 @@\n-  int a = 1;\n+  int a = 9;\n" +
		"--- a/src/util.py\n+++ b/src/util.py\n@@ -1,2 +1,2 @@\n def f():\n-    return 42\n+    return 43\n"
	want(t, run(t, g, "apply_patch", map[string]any{"patch": bad}), "failed", "hunk-mismatch")
	if b, _ := os.ReadFile(filepath.Join(g.dir, "src/main/App.java")); strings.Contains(string(b), "= 9") {
		t.Error("partial patch was written")
	}
	want(t, run(t, g, "apply_patch", map[string]any{"patch": "@@ -1 +1 @@\n-x\n+y\n"}), "failed", "parse")
	want(t, run(t, g, "apply_patch", map[string]any{"patch": "--- a/pom.xml\n+++ b/pom.xml\n@@ -1 +1 @@\n-<project/>\n+<p/>\n"}), "refused", "")
}

func TestWholeFileKeepsModeAndChecksLines(t *testing.T) {
	g := fixture(t)
	run(t, g, "read_file", map[string]any{"path": "run.sh"})
	want(t, run(t, g, "write_file", map[string]any{"path": "run.sh", "content": "#!/bin/sh\necho bye\n"}), "ok", "")
	if info, _ := os.Stat(filepath.Join(g.dir, "run.sh")); info.Mode().Perm()&0o100 == 0 {
		t.Error("executable bit lost")
	}
	long := strings.Repeat("x", 600)
	want(t, run(t, g, "write_file", map[string]any{"path": "src/long.py", "content": "s = '" + long + "'\n"}), "refused", "unreviewable")
	want(t, run(t, g, "write_file", map[string]any{"path": "docs/long.md", "content": long + "\n"}), "ok", "")
}

func TestTestResults(t *testing.T) {
	g := fixture(t)
	if o := run(t, g, "read_test_results", nil); !strings.Contains(o.Result, "No JUnit XML") {
		t.Errorf("without reports: %s", o.Result)
	}
	xml := `<?xml version="1.0"?><testsuite name="AppTest" tests="2"><testcase classname="AppTest" name="ok"/>` +
		`<testcase classname="AppTest" name="adds"><failure message="expected 3 but was 2" type="AssertionError">at AppTest.adds(AppTest.java:12)</failure></testcase></testsuite>`
	os.MkdirAll(filepath.Join(g.dir, "target/surefire-reports"), 0o755)
	os.WriteFile(filepath.Join(g.dir, "target/surefire-reports/TEST-AppTest.xml"), []byte(xml), 0o644)
	o := run(t, g, "read_test_results", nil)
	for _, s := range []string{"2 tests, 1 failed", "AppTest.adds", "expected 3 but was 2", "AppTest.java:12"} {
		if !strings.Contains(o.Result, s) {
			t.Errorf("results lack %q:\n%s", s, o.Result)
		}
	}
	if strings.Contains(o.Result, "STALE") {
		t.Error("fresh results marked stale")
	}
	time.Sleep(20 * time.Millisecond)
	run(t, g, "read_file", map[string]any{"path": "src/util.py"})
	run(t, g, "edit_file", map[string]any{"path": "src/util.py", "old_text": "return 1", "new_text": "return 5"})
	if o := run(t, g, "read_test_results", nil); !strings.Contains(o.Result, "STALE") {
		t.Errorf("results after an edit not marked stale:\n%s", o.Result)
	}
}

func TestToolSpecsPerFormat(t *testing.T) {
	for format, edit := range map[string]string{"replace": "edit_file", "diff": "apply_patch", "whole": "write_file"} {
		var names []string
		for _, s := range toolSpecs(format) {
			names = append(names, s.Name)
		}
		joined := strings.Join(names, " ")
		if !strings.Contains(joined, edit) {
			t.Errorf("%s: no %s in %s", format, edit, joined)
		}
		for _, banned := range []string{"bash", "shell", "run", "exec", "fetch", "git"} {
			for _, n := range names {
				if n == banned {
					t.Errorf("%s: tool %s exists", format, n)
				}
			}
		}
	}
}

// Only the current format's edit tool is offered; the loop refuses the others
// by checking the offered list (see runTurn).
func TestFormatsOfferOneEditTool(t *testing.T) {
	edits := map[string]bool{"edit_file": true, "create_file": true, "apply_patch": true, "write_file": true}
	for _, format := range []string{"replace", "diff", "whole"} {
		n := 0
		for _, s := range toolSpecs(format) {
			if edits[s.Name] {
				n++
			}
		}
		if (format == "replace" && n != 2) || (format != "replace" && n != 1) {
			t.Errorf("%s offers %d edit tools", format, n)
		}
	}
}
