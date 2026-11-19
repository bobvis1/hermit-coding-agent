package main

// The file guard approximates Hermit's read and write rules for the spike
// (REQUIREMENTS.md M8, M13, M6, M3.2). It is throwaway code: good enough to
// show whether the constraints leave the agent useful, not a security
// boundary. Unlike Hermit, it asks git directly.

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Refusal is a rule stopping a read or write. Kind is logged; Msg goes to
// the model.
type Refusal struct {
	Kind string
	Msg  string
}

func (r *Refusal) Error() string { return r.Msg }

const notAvailable = "not available"

func unavailable(kind, p string) *Refusal {
	return &Refusal{Kind: kind, Msg: fmt.Sprintf("%s: %s", p, notAvailable)}
}

type Guard struct {
	root           *os.Root
	dir            string
	read           map[string][32]byte // content hash when last read or written by the agent
	lastAgentWrite time.Time
	binCache       map[string]binEntry
}

type binEntry struct {
	size  int64
	mtime time.Time
	bin   bool
}

func NewGuard(dir string) (*Guard, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(abs)
	if err != nil {
		return nil, err
	}
	if _, err := gitOut(abs, "rev-parse", "--show-toplevel"); err != nil {
		return nil, fmt.Errorf("%s is not a git repository: %w", abs, err)
	}
	return &Guard{root: root, dir: abs, read: map[string][32]byte{}, binCache: map[string]binEntry{}}, nil
}

func gitOut(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return string(out), nil
}

// clean turns a model-supplied path into a clean relative slash path, or
// refuses it.
func (g *Guard) clean(p string) (string, *Refusal) {
	p = strings.TrimSpace(filepath.ToSlash(p))
	if p == "" || p == "." || p == "./" {
		return ".", nil
	}
	if strings.HasPrefix(p, "/") {
		if rel, err := filepath.Rel(g.dir, p); err == nil && !strings.HasPrefix(rel, "..") {
			p = filepath.ToSlash(rel)
		} else {
			return "", unavailable("outside-root", p)
		}
	}
	c := path.Clean(p)
	if c == ".." || strings.HasPrefix(c, "../") {
		return "", unavailable("outside-root", p)
	}
	return c, nil
}

// --- Rules -----------------------------------------------------------------

// Unreadable names (M13.1), matched case-insensitively on the base name or
// any path element.
func unreadableName(p string) bool {
	lp := strings.ToLower(p)
	for _, el := range strings.Split(lp, "/") {
		if el == ".git" {
			return true
		}
	}
	base := path.Base(lp)
	switch {
	case strings.HasPrefix(base, ".env"),
		strings.HasSuffix(base, ".pem"), strings.HasSuffix(base, ".key"),
		strings.HasSuffix(base, ".p12"), strings.HasSuffix(base, ".pfx"),
		strings.HasSuffix(base, ".jks"), strings.HasSuffix(base, ".keystore"),
		strings.HasPrefix(base, "id_rsa"), strings.HasPrefix(base, "id_ed25519"),
		strings.HasPrefix(base, "id_ecdsa"), strings.HasPrefix(base, "id_dsa"),
		base == ".netrc", base == ".npmrc", base == ".pypirc", base == "credentials",
		base == "credentials.json", base == ".git-credentials", base == ".agentignore":
		return true
	}
	return false
}

// Protected paths (M8.1): the agent may read but not write them.
var protectedBase = map[string]bool{
	".gitlab-ci.yml": true, "jenkinsfile": true, "makefile": true, "gnumakefile": true, "dockerfile": true,
	"pom.xml": true, "mvnw": true, "mvnw.cmd": true, "gradlew": true, "gradlew.bat": true,
	"build.gradle": true, "build.gradle.kts": true, "settings.gradle": true, "settings.gradle.kts": true,
	"gradle.properties": true,
	"package.json":      true, "package-lock.json": true, "npm-shrinkwrap.json": true, "yarn.lock": true,
	"pnpm-lock.yaml": true, "pnpm-workspace.yaml": true, "bun.lockb": true, "bun.lock": true,
	"pyproject.toml": true, "setup.py": true, "setup.cfg": true, "poetry.lock": true, "pipfile": true,
	"pipfile.lock": true, "uv.lock": true, "pdm.lock": true, "tox.ini": true, "noxfile.py": true,
	"cargo.toml": true, "cargo.lock": true, "build.rs": true, "go.mod": true, "go.sum": true,
	".npmrc": true, ".yarnrc": true, ".yarnrc.yml": true, ".pypirc": true, "pip.conf": true,
	".gitattributes": true, ".gitmodules": true, ".gitignore": true,
	".envrc": true, ".pre-commit-config.yaml": true, "lefthook.yml": true, "codeowners": true,
	"agents.md": true, "claude.md": true, ".cursorrules": true, "copilot-instructions.md": true,
	".project": true, ".classpath": true,
}

var protectedDirs = []string{
	".git/", ".github/workflows/", ".github/actions/", ".mvn/", "buildsrc/", "gradle/wrapper/",
	".devcontainer/", ".vscode/", ".idea/", ".settings/", ".externaltoolbuilders/", ".husky/",
	".circleci/", ".cargo/", ".claude/", ".cursor/",
}

func protected(p string) bool {
	lp := strings.ToLower(p)
	base := path.Base(lp)
	if protectedBase[base] {
		return true
	}
	if strings.HasPrefix(base, "requirements") && strings.HasSuffix(base, ".txt") {
		return true
	}
	if (strings.HasPrefix(base, "docker-compose") || strings.HasPrefix(base, "compose")) &&
		(strings.HasSuffix(base, ".yml") || strings.HasSuffix(base, ".yaml")) {
		return true
	}
	if strings.HasSuffix(base, ".code-workspace") || strings.HasSuffix(base, ".launch") {
		return true
	}
	for _, d := range protectedDirs {
		if strings.HasPrefix(lp+"/", d) || strings.Contains("/"+lp+"/", "/"+d) {
			return true
		}
	}
	return false
}

var prose = map[string]bool{".md": true, ".markdown": true, ".txt": true, ".rst": true, ".adoc": true}

// --- The readable set (M13.2, M13.3) ---------------------------------------

// files lists the paths the agent may see: tracked or untracked-but-not-
// ignored, still present, not unreadable by name, and not binary.
func (g *Guard) files() ([]string, error) {
	out, err := gitOut(g.dir, "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var list []string
	for _, p := range strings.Split(out, "\x00") {
		if p == "" || seen[p] || unreadableName(p) {
			continue
		}
		seen[p] = true
		info, err := g.root.Lstat(p)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		if g.isBinary(p, info) {
			continue
		}
		list = append(list, p)
	}
	sort.Strings(list)
	return list, nil
}

func (g *Guard) isBinary(p string, info fs.FileInfo) bool {
	if e, ok := g.binCache[p]; ok && e.size == info.Size() && e.mtime.Equal(info.ModTime()) {
		return e.bin
	}
	bin := true
	if f, err := g.root.Open(p); err == nil {
		buf := make([]byte, 8192)
		n, _ := f.Read(buf)
		f.Close()
		bin = looksBinary(buf[:n])
	}
	g.binCache[p] = binEntry{size: info.Size(), mtime: info.ModTime(), bin: bin}
	return bin
}

// looksBinary recognizes binary and container formats by content (M13.4).
func looksBinary(b []byte) bool {
	magic := [][]byte{[]byte("PK\x03\x04"), []byte("%PDF"), []byte("SQLite format 3"), []byte("\x89PNG"),
		[]byte("GIF8"), []byte("\xff\xd8\xff"), []byte("PAR1"), {0xd0, 0xcf, 0x11, 0xe0}, {0x1f, 0x8b}}
	for _, m := range magic {
		if bytes.HasPrefix(b, m) {
			return true
		}
	}
	if len(b) > 132 && string(b[128:132]) == "DICM" {
		return true
	}
	return bytes.IndexByte(b, 0) >= 0
}

// readable reports whether the agent may read p; the refusal never says
// whether the file exists.
func (g *Guard) readable(p string) *Refusal {
	if unreadableName(p) {
		return unavailable("unreadable", p)
	}
	info, err := g.root.Lstat(p)
	if err != nil || !info.Mode().IsRegular() {
		return unavailable("missing", p)
	}
	if g.ignored(p) {
		return unavailable("ignored", p)
	}
	if g.isBinary(p, info) {
		return unavailable("binary", p)
	}
	return nil
}

// ignored asks git whether a path is ignored and untracked.
func (g *Guard) ignored(p string) bool {
	if _, err := gitOut(g.dir, "ls-files", "--error-unmatch", "--", p); err == nil {
		return false // tracked
	}
	cmd := exec.Command("git", "-C", g.dir, "check-ignore", "-q", "--", p)
	err := cmd.Run()
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 {
		return false
	}
	return true // ignored, or git failed: refuse
}

// --- Reads -----------------------------------------------------------------

func (g *Guard) Read(p string) (string, *Refusal) {
	p, r := g.clean(p)
	if r != nil {
		return "", r
	}
	if r := g.readable(p); r != nil {
		return "", r
	}
	data, err := g.root.ReadFile(p)
	if err != nil {
		return "", unavailable("missing", p)
	}
	g.read[p] = sha256.Sum256(data)
	if strings.HasSuffix(strings.ToLower(p), ".ipynb") {
		return stripNotebookOutputs(data), nil
	}
	return string(data), nil
}

// stripNotebookOutputs removes outputs from a notebook's view (M13.5).
func stripNotebookOutputs(data []byte) string {
	var nb map[string]any
	if json.Unmarshal(data, &nb) != nil {
		return string(data)
	}
	if cells, ok := nb["cells"].([]any); ok {
		for _, c := range cells {
			if cell, ok := c.(map[string]any); ok {
				if _, has := cell["outputs"]; has {
					cell["outputs"] = []any{}
				}
				delete(cell, "attachments")
			}
		}
	}
	delete(nb, "widgets")
	out, _ := json.MarshalIndent(nb, "", " ")
	return string(out)
}

// --- Writes ----------------------------------------------------------------

// checkWrite applies the write rules. exists says whether p is already a
// file; for an existing file the agent must have read the current version
// (M3.2).
func (g *Guard) checkWrite(p string) (exists bool, r *Refusal) {
	if unreadableName(p) {
		return false, unavailable("unreadable", p)
	}
	info, err := g.root.Lstat(p)
	exists = err == nil
	if exists && info.Mode()&fs.ModeSymlink != 0 {
		return true, &Refusal{Kind: "symlink", Msg: p + ": is a symbolic link; writes to links are refused"}
	}
	if exists && !info.Mode().IsRegular() {
		return true, unavailable("not-regular", p)
	}
	if g.ignored(p) {
		return exists, unavailable("ignored", p)
	}
	if protected(p) {
		return exists, &Refusal{Kind: "protected", Msg: p + ": is protected (build, CI, dependency, tool or IDE configuration, git internals, or instructions for other AI tools). Describe the change for the developer to make instead."}
	}
	if exists {
		if g.isBinary(p, info) {
			return true, unavailable("binary", p)
		}
		last, ok := g.read[p]
		if !ok {
			return true, &Refusal{Kind: "not-read", Msg: p + ": read the file before changing it"}
		}
		data, err := g.root.ReadFile(p)
		if err != nil {
			return true, unavailable("missing", p)
		}
		if sha256.Sum256(data) != last {
			delete(g.read, p)
			return true, &Refusal{Kind: "stale", Msg: p + ": changed since you last read it (probably by the developer). Read it again before editing."}
		}
	}
	return exists, nil
}

// reviewable applies M6 to the lines a write adds.
func reviewable(p, oldContent, newContent string) *Refusal {
	if strings.IndexByte(newContent, 0) >= 0 {
		return &Refusal{Kind: "unreviewable", Msg: p + ": binary content can't be reviewed; write refused"}
	}
	if prose[strings.ToLower(path.Ext(p))] {
		return nil
	}
	old := map[string]int{}
	for _, l := range strings.Split(oldContent, "\n") {
		old[l]++
	}
	for i, l := range strings.Split(newContent, "\n") {
		if old[l] > 0 {
			old[l]--
			continue
		}
		if len([]rune(l)) > 500 {
			return &Refusal{Kind: "unreviewable", Msg: fmt.Sprintf("%s: line %d is over 500 characters, too long to review; split it", p, i+1)}
		}
	}
	return nil
}

// Write replaces or creates a file atomically, keeping its permissions.
func (g *Guard) write(p string, content string) error {
	mode := fs.FileMode(0o644)
	if info, err := g.root.Stat(p); err == nil {
		mode = info.Mode().Perm()
	}
	if dir := path.Dir(p); dir != "." {
		if err := g.root.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	tmp := fmt.Sprintf("%s.spike-tmp-%d", p, time.Now().UnixNano())
	f, err := g.root.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(content); err != nil {
		f.Close()
		g.root.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		g.root.Remove(tmp)
		return err
	}
	if err := g.root.Chmod(tmp, mode); err != nil {
		g.root.Remove(tmp)
		return err
	}
	if err := g.root.Rename(tmp, p); err != nil {
		g.root.Remove(tmp)
		return err
	}
	g.read[p] = sha256.Sum256([]byte(content))
	g.lastAgentWrite = time.Now()
	return nil
}

func (g *Guard) remove(p string) error {
	if err := g.root.Remove(p); err != nil {
		return err
	}
	delete(g.read, p)
	g.lastAgentWrite = time.Now()
	return nil
}

// current returns the file's content for an edit, after the write checks.
func (g *Guard) current(p string) (content string, exists bool, r *Refusal) {
	exists, r = g.checkWrite(p)
	if r != nil || !exists {
		return "", exists, r
	}
	data, err := g.root.ReadFile(p)
	if err != nil {
		return "", true, unavailable("missing", p)
	}
	return string(data), true, nil
}
