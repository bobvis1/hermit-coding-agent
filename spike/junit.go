package main

// Reading test results (REQUIREMENTS.md M15): the harness, not the agent,
// reads JUnit XML reports from the usual build-tool locations, which are
// normally git-ignored, and gives the agent a structured summary with a
// freshness check against the agent's last change.

import (
	"encoding/xml"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type junitSuites struct {
	Suites []junitSuite `xml:"testsuite"`
}

type junitSuite struct {
	Name   string       `xml:"name,attr"`
	Cases  []junitCase  `xml:"testcase"`
	Suites []junitSuite `xml:"testsuite"`
}

type junitCase struct {
	Name      string        `xml:"name,attr"`
	Classname string        `xml:"classname,attr"`
	File      string        `xml:"file,attr"`
	Failure   []junitResult `xml:"failure"`
	Error     []junitResult `xml:"error"`
	Skipped   *struct{}     `xml:"skipped"`
}

type junitResult struct {
	Message string `xml:"message,attr"`
	Type    string `xml:"type,attr"`
	Body    string `xml:",chardata"`
}

var skipDirs = map[string]bool{".git": true, "node_modules": true, ".venv": true, "venv": true, ".tox": true, ".gradle": true}

// isReportPath recognizes the default report locations (M15.1).
func isReportPath(p string) bool {
	lp := strings.ToLower(filepath.ToSlash(p))
	if !strings.HasSuffix(lp, ".xml") {
		return false
	}
	base := path.Base(lp)
	for _, d := range []string{"surefire-reports/", "failsafe-reports/", "test-results/", "test-reports/", "junit/"} {
		if strings.Contains(lp, d) {
			return true
		}
	}
	return strings.HasPrefix(base, "test-") || strings.Contains(base, "junit") || strings.Contains(base, "test-results") ||
		strings.Contains(base, "testresults") || strings.HasPrefix(base, "report")
}

type report struct {
	path  string
	mtime time.Time
}

func (g *Guard) findReports(extra []string) ([]report, error) {
	var found []report
	err := fs.WalkDir(g.root.FS(), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if p != "." && skipDirs[d.Name()] {
				return fs.SkipDir
			}
			return nil
		}
		ok := isReportPath(p)
		for _, pat := range extra {
			if m, _ := path.Match(pat, p); m {
				ok = true
			}
		}
		if !ok || unreadableName(p) {
			return nil
		}
		info, err := d.Info()
		if err != nil || !info.Mode().IsRegular() {
			return nil
		}
		found = append(found, report{path: p, mtime: info.ModTime()})
		return nil
	})
	sort.Slice(found, func(i, j int) bool { return found[i].mtime.After(found[j].mtime) })
	return found, err
}

// TestResults summarizes the latest run: reports written within ten minutes
// of the newest one.
func (g *Guard) TestResults(extra []string) string {
	reports, err := g.findReports(extra)
	if err != nil {
		return "Couldn't search for test reports: " + err.Error()
	}
	type failed struct{ name, kind, msg, body string }
	var total, skipped int
	var failures []failed
	var used []report
	var newest time.Time
	for _, r := range reports {
		data, err := g.root.ReadFile(r.path)
		if err != nil || len(data) > 20<<20 {
			continue
		}
		var suites junitSuites
		if xml.Unmarshal(data, &suites) != nil || len(suites.Suites) == 0 {
			var one junitSuite
			if xml.Unmarshal(data, &one) != nil {
				continue
			}
			suites.Suites = []junitSuite{one}
		}
		cases := flatten(suites.Suites)
		if len(cases) == 0 {
			continue
		}
		if newest.IsZero() {
			newest = r.mtime
		} else if newest.Sub(r.mtime) > 10*time.Minute {
			continue
		}
		used = append(used, r)
		for _, c := range cases {
			total++
			if c.Skipped != nil {
				skipped++
			}
			name := c.Name
			if c.Classname != "" {
				name = c.Classname + "." + c.Name
			}
			for _, f := range c.Failure {
				failures = append(failures, failed{name, "failure", f.Message, f.Body})
			}
			for _, e := range c.Error {
				failures = append(failures, failed{name, "error " + e.Type, e.Message, e.Body})
			}
		}
	}
	if len(used) == 0 {
		return "No JUnit XML test reports found. Ask the developer to run the tests with reports turned on " +
			"(for example Maven/Gradle test tasks, pytest --junitxml=..., or a Jest JUnit reporter), or to paste the failure."
	}
	var b strings.Builder
	age := time.Since(newest).Round(time.Second)
	fmt.Fprintf(&b, "Latest test run: %d tests, %d failed, %d skipped, from %d report file(s), written %s ago.\n",
		total, len(failures), skipped, len(used), age)
	if !g.lastAgentWrite.IsZero() && newest.Before(g.lastAgentWrite) {
		fmt.Fprintf(&b, "STALE: these results were written before your last change; the developer hasn't rerun the tests since.\n")
	}
	for i, f := range failures {
		if i == 50 {
			fmt.Fprintf(&b, "... and %d more failures\n", len(failures)-50)
			break
		}
		fmt.Fprintf(&b, "\nFAILED %s (%s)\n", f.name, f.kind)
		if f.msg != "" {
			fmt.Fprintf(&b, "  message: %s\n", truncate(f.msg, 2000))
		}
		if body := strings.TrimSpace(f.body); body != "" {
			lines := strings.Split(body, "\n")
			if len(lines) > 40 {
				lines = append(lines[:40], fmt.Sprintf("... (%d more lines)", len(lines)-40))
			}
			fmt.Fprintf(&b, "  %s\n", strings.Join(lines, "\n  "))
		}
	}
	return b.String()
}

func flatten(suites []junitSuite) []junitCase {
	var out []junitCase
	for _, s := range suites {
		out = append(out, s.Cases...)
		out = append(out, flatten(s.Suites)...)
	}
	return out
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
