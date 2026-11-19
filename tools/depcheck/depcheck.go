// Package depcheck enforces Hermit's dependency rules over a Go module's
// non-test code (IMPLEMENTATION_PLAN.md, section 1 and milestone 0):
//
//   - only internal/gitcmd may start a process, checked over every package
//     linked into the module's commands, third-party ones included;
//   - only fsguard, localstore and policy may use file-system APIs;
//   - only fsguard, localstore, policy and credstore may make system calls;
//   - fsguard, scan and render import nothing else from Hermit;
//   - provider may not reach fsguard, tools, scm or testresults;
//   - only netclient imports net/http;
//   - tooldef.Set is constructed only in tools, render.ApprovedMRText only in
//     tui;
//   - no cgo, no plugins, no go:linkname;
//   - every module in go.mod is on the approved list, with a compatible
//     license, vendored, and not replaced.
//
// Test files are not checked: they may do what the rules forbid.
package depcheck

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/mod/modfile"
	"golang.org/x/tools/go/packages"
)

// Violation is one broken rule.
type Violation struct {
	Rule string // short name of the rule
	Pkg  string // package path, or go.mod
	Pos  string // file:line, when there is one
	Msg  string
	GOOS string // the first target it was found for
}

func (v Violation) String() string {
	where := v.Pkg
	if v.Pos != "" {
		where = v.Pos
	}
	s := fmt.Sprintf("%s: [%s] %s", where, v.Rule, v.Msg)
	if v.GOOS != "" {
		s += " (GOOS=" + v.GOOS + ")"
	}
	return s
}

// Config says what to check.
type Config struct {
	Dir          string   // root of the module to check
	GOOS         []string // targets to load the packages for
	ApprovedFile string   // the approved third-party modules list
}

// Run checks the module and returns every violation, sorted. An error means
// the check could not be completed, which callers must treat as a failure.
func Run(cfg Config) ([]Violation, error) {
	if len(cfg.GOOS) == 0 {
		return nil, fmt.Errorf("no GOOS targets given")
	}
	approved, err := ReadApproved(cfg.ApprovedFile)
	if err != nil {
		return nil, err
	}
	modPath, vs, err := checkModules(cfg.Dir, approved)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, goos := range cfg.GOOS {
		pkgs, err := load(cfg.Dir, goos)
		if err != nil {
			return nil, err
		}
		c := &checker{modPath: modPath, fset: token.NewFileSet()}
		for _, v := range c.check(pkgs) {
			key := v.Rule + "\x00" + v.Pkg + "\x00" + v.Pos + "\x00" + v.Msg
			if seen[key] {
				continue
			}
			seen[key] = true
			v.GOOS = goos
			vs = append(vs, v)
		}
	}
	sort.SliceStable(vs, func(i, j int) bool {
		if vs[i].Pkg != vs[j].Pkg {
			return vs[i].Pkg < vs[j].Pkg
		}
		if vs[i].Pos != vs[j].Pos {
			return vs[i].Pos < vs[j].Pos
		}
		return vs[i].Msg < vs[j].Msg
	})
	return vs, nil
}

// Approved is the approved third-party modules list: module path to license.
type Approved map[string]string

// ReadApproved parses the approved list. Each non-comment line is a module
// path, its SPDX license expression, and optional notes after " -- ".
func ReadApproved(name string) (Approved, error) {
	data, err := os.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("reading approved list: %w", err)
	}
	a := Approved{}
	for i, line := range strings.Split(string(data), "\n") {
		line, _, _ = strings.Cut(line, " -- ")
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		path, license, ok := strings.Cut(line, " ")
		license = strings.TrimSpace(license)
		if !ok || license == "" {
			return nil, fmt.Errorf("%s:%d: want a module path and a license", name, i+1)
		}
		if !compatibleLicenses[license] {
			return nil, fmt.Errorf("%s:%d: license %q of %s is not on the compatible list", name, i+1, license, path)
		}
		if _, dup := a[path]; dup {
			return nil, fmt.Errorf("%s:%d: %s listed twice", name, i+1, path)
		}
		a[path] = license
	}
	return a, nil
}

func checkModules(dir string, approved Approved) (string, []Violation, error) {
	name := filepath.Join(dir, "go.mod")
	data, err := os.ReadFile(name)
	if err != nil {
		return "", nil, err
	}
	f, err := modfile.Parse(name, data, nil)
	if err != nil {
		return "", nil, err
	}
	if f.Module == nil {
		return "", nil, fmt.Errorf("%s: no module line", name)
	}
	var vs []Violation
	add := func(rule, msg string) {
		vs = append(vs, Violation{Rule: rule, Pkg: "go.mod", Msg: msg})
	}
	for _, r := range f.Require {
		if _, ok := approved[r.Mod.Path]; !ok {
			add("approved-modules", fmt.Sprintf("%s is not on the approved list; adding it goes through QUESTION.md", r.Mod.Path))
		}
	}
	for _, r := range f.Replace {
		add("approved-modules", fmt.Sprintf("replace directive for %s: dependencies come from the approved, vendored versions only", r.Old.Path))
	}
	if len(f.Require) > 0 {
		if _, err := os.Stat(filepath.Join(dir, "vendor", "modules.txt")); err != nil {
			add("vendor", "go.mod has requirements but vendor/modules.txt is missing; run go mod vendor")
		}
	}
	return f.Module.Mod.Path, vs, nil
}

func load(dir, goos string) ([]*packages.Package, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedImports | packages.NeedDeps |
			packages.NeedTypes | packages.NeedSyntax | packages.NeedTypesInfo | packages.NeedModule,
		Dir:   dir,
		Env:   append(os.Environ(), "GOOS="+goos, "CGO_ENABLED=0", "GOWORK=off"),
		Tests: false,
	}
	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return nil, fmt.Errorf("loading packages for GOOS=%s: %w", goos, err)
	}
	var errs []string
	packages.Visit(pkgs, nil, func(p *packages.Package) {
		for _, e := range p.Errors {
			errs = append(errs, e.Error())
		}
	})
	if len(errs) > 0 {
		return nil, fmt.Errorf("loading packages for GOOS=%s:\n  %s", goos, strings.Join(errs, "\n  "))
	}
	if len(pkgs) == 0 {
		return nil, fmt.Errorf("no packages found in %s", dir)
	}
	return pkgs, nil
}

type checker struct {
	modPath string
	fset    *token.FileSet // for files not loaded by go/packages
	out     []Violation
}

func (c *checker) report(p *packages.Package, pos token.Pos, rule, format string, args ...any) {
	v := Violation{Rule: rule, Pkg: p.PkgPath, Msg: fmt.Sprintf(format, args...)}
	if pos.IsValid() {
		position := p.Fset.Position(pos)
		v.Pos = fmt.Sprintf("%s:%d", position.Filename, position.Line)
	}
	c.out = append(c.out, v)
}

// rel returns the package path relative to the module, and whether the
// package belongs to the module.
func (c *checker) rel(p *packages.Package) (string, bool) {
	if p.Module == nil || !p.Module.Main {
		return "", false
	}
	if p.PkgPath == c.modPath {
		return ".", true
	}
	r, ok := strings.CutPrefix(p.PkgPath, c.modPath+"/")
	return r, ok
}

func (c *checker) check(roots []*packages.Package) []Violation {
	all := map[string]*packages.Package{}
	packages.Visit(roots, nil, func(p *packages.Package) { all[p.PkgPath] = p })
	paths := make([]string, 0, len(all))
	for path := range all {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		p := all[path]
		rel, first := c.rel(p)
		if why, ok := forbiddenPackages[p.PkgPath]; ok {
			c.report(p, token.NoPos, "process", "%s may not be linked: %s", p.PkgPath, why)
		}
		c.checkProcess(p, rel, first)
		if first {
			c.checkFirstParty(p, rel)
		}
	}
	return c.out
}

// isStd reports whether p is in the standard library.
func isStd(p *packages.Package) bool {
	return p.Module == nil && !strings.Contains(strings.SplitN(p.PkgPath, "/", 2)[0], ".")
}

// definesProcessAPIs reports whether p is one of the packages that define
// the process-starting APIs; uses of those APIs are checked at their callers.
func definesProcessAPIs(p *packages.Package) bool {
	if isStd(p) {
		switch p.PkgPath {
		case "os", "os/exec", "syscall", "runtime":
			return true
		}
		return strings.HasPrefix(p.PkgPath, "internal/") || strings.Contains(p.PkgPath, "/internal/") ||
			strings.HasPrefix(p.PkgPath, "vendor/")
	}
	return p.PkgPath == "golang.org/x/sys" || strings.HasPrefix(p.PkgPath, "golang.org/x/sys/")
}

func (c *checker) checkProcess(p *packages.Package, rel string, first bool) {
	if _, ok := p.Imports["os/exec"]; ok && !(first && rel == pkgGitcmd) {
		c.report(p, importPos(p, "os/exec"), "process", "imports os/exec; only %s may start a process", pkgGitcmd)
	}
	if definesProcessAPIs(p) {
		return
	}
	for id, obj := range p.TypesInfo.Uses {
		if obj.Pkg() == nil || obj.Parent() != obj.Pkg().Scope() {
			continue
		}
		key := obj.Pkg().Path() + "." + obj.Name()
		if processObjects[key] {
			c.report(p, id.Pos(), "process", "uses %s; only %s may start a process, through os/exec", key, pkgGitcmd)
		}
	}
	loads := false
	for imp := range p.Imports {
		if dynamicLoaders[imp] {
			loads = true
		}
	}
	if !loads {
		return
	}
	for _, f := range p.Syntax {
		ast.Inspect(f, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			if s, err := strconv.Unquote(lit.Value); err == nil && processSymbols[s] {
				c.report(p, lit.Pos(), "process", "names %q, a function that starts a process, in a package that can call functions by name", s)
			}
			return true
		})
	}
}

func (c *checker) checkFirstParty(p *packages.Package, rel string) {
	// Imports.
	for imp := range p.Imports {
		pos := importPos(p, imp)
		impRel, impFirst := c.rel(p.Imports[imp])
		switch {
		case imp == "plugin":
			c.report(p, pos, "plugin", "imports plugin; Hermit loads no code at run time")
		case fileImportsDenied[imp] && !fileSystemPackages[rel]:
			c.report(p, pos, "filesystem", "imports %s; only %s may use file-system APIs", imp, names(fileSystemPackages))
		case (imp == "syscall" || imp == "unsafe" || imp == "golang.org/x/sys" || strings.HasPrefix(imp, "golang.org/x/sys/")) && !systemCallPackages[rel]:
			c.report(p, pos, "syscall", "imports %s; only %s may make system calls", imp, names(systemCallPackages))
		case imp == "github.com/ebitengine/purego" && !puregoPackages[rel]:
			c.report(p, pos, "syscall", "imports purego; only %s may", names(puregoPackages))
		case (imp == "net/http" || strings.HasPrefix(imp, "net/http/")) && rel != pkgNetclient:
			c.report(p, pos, "net-http", "imports %s; every outbound connection goes through %s", imp, pkgNetclient)
		case impFirst && selfContained[rel]:
			c.report(p, pos, "self-contained", "imports %s; %s imports nothing else from Hermit", impRel, rel)
		}
	}
	if rel == pkgProvider {
		reached := map[string]bool{}
		var walk func(q *packages.Package)
		walk = func(q *packages.Package) {
			for _, dep := range q.Imports {
				if reached[dep.PkgPath] {
					continue
				}
				reached[dep.PkgPath] = true
				if depRel, ok := c.rel(dep); ok && providerMayNotReach[depRel] {
					c.report(p, token.NoPos, "provider-imports", "reaches %s; provider's only source of content is scan", depRel)
				}
				walk(dep)
			}
		}
		walk(p)
	}

	// Uses of objects: file-system APIs and restricted constructors.
	for id, obj := range p.TypesInfo.Uses {
		if obj.Pkg() == nil {
			continue
		}
		if !fileSystemPackages[rel] {
			if msg := fileAPI(obj); msg != "" {
				c.report(p, id.Pos(), "filesystem", "uses %s; only %s may use file-system APIs", msg, names(fileSystemPackages))
			}
		}
		// Constructors: functions, methods, and variables or fields of
		// function type, defined in the restricted type's own package, that
		// return one. Functions elsewhere can only return a value they got
		// from one of these, or from a literal or conversion, which are
		// checked below. Passing a constructor out of an allowed package as
		// a function value is not detected; review covers that.
		if sig, ok := obj.Type().Underlying().(*types.Signature); ok {
			name := obj.Pkg().Path() + "." + obj.Name()
			if fn, ok := obj.(*types.Func); ok {
				name = fn.FullName()
			}
			for i := 0; i < sig.Results().Len(); i++ {
				t := restricted(c.modPath, sig.Results().At(i).Type())
				if t == "" || restrictedTypes[t][rel] || c.modPath+"/"+t[:strings.LastIndex(t, ".")] != obj.Pkg().Path() {
					continue
				}
				if recv := sig.Recv(); recv != nil && restricted(c.modPath, recv.Type()) == t {
					continue // derives a value from one the caller already has
				}
				c.report(p, id.Pos(), "constructor", "uses %s, which returns %s; only %s may construct one", name, t, names(restrictedTypes[t]))
			}
		}
	}
	for _, f := range p.Syntax {
		ast.Inspect(f, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.CompositeLit:
				if tv, ok := p.TypesInfo.Types[n]; ok {
					if t := restricted(c.modPath, tv.Type); t != "" && !restrictedTypes[t][rel] {
						c.report(p, n.Pos(), "constructor", "composite literal of %s; only %s may construct one", t, names(restrictedTypes[t]))
					}
				}
			case *ast.CallExpr:
				if tv, ok := p.TypesInfo.Types[n.Fun]; ok && tv.IsType() {
					if t := restricted(c.modPath, tv.Type); t != "" && !restrictedTypes[t][rel] {
						c.report(p, n.Pos(), "constructor", "conversion to %s; only %s may construct one", t, names(restrictedTypes[t]))
					}
				}
			}
			return true
		})
		for _, cg := range f.Comments {
			for _, cm := range cg.List {
				if strings.HasPrefix(cm.Text, "//go:linkname") {
					c.report(p, cm.Pos(), "linkname", "go:linkname reaches around the package rules")
				}
			}
		}
	}

	// Files excluded from this build, such as cgo files under CGO_ENABLED=0.
	for _, name := range p.IgnoredFiles {
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(c.fset, name, nil, parser.ImportsOnly)
		if err != nil {
			c.out = append(c.out, Violation{Rule: "parse", Pkg: p.PkgPath, Pos: name, Msg: err.Error()})
			continue
		}
		for _, imp := range f.Imports {
			if imp.Path.Value == `"C"` {
				pos := c.fset.Position(imp.Pos())
				c.out = append(c.out, Violation{Rule: "cgo", Pkg: p.PkgPath, Pos: fmt.Sprintf("%s:%d", pos.Filename, pos.Line),
					Msg: `imports "C"; Hermit builds with CGO_ENABLED=0`})
			}
		}
	}
}

// fileAPI describes obj if it is a file-system API, or returns "".
func fileAPI(obj types.Object) string {
	pkg := obj.Pkg().Path()
	if fn, ok := obj.(*types.Func); ok && fileFuncsDenied[fn.FullName()] {
		return fn.FullName()
	}
	if obj.Parent() != obj.Pkg().Scope() {
		return ""
	}
	name := pkg + "." + obj.Name()
	switch obj.(type) {
	case *types.Func, *types.Var:
	default:
		return "" // types and constants
	}
	switch pkg {
	case "os":
		if !osAllowed[obj.Name()] {
			return name
		}
	case "io/fs":
		if _, isFunc := obj.(*types.Func); isFunc && !ioFSAllowed[obj.Name()] {
			return name
		}
	case "path/filepath":
		if filepathDenied[obj.Name()] {
			return name
		}
	}
	return ""
}

// restricted returns the module-relative name of t's restricted type, after
// removing one pointer, or "".
func restricted(modPath string, t types.Type) string {
	t = types.Unalias(t)
	if ptr, ok := t.(*types.Pointer); ok {
		t = types.Unalias(ptr.Elem())
	}
	named, ok := t.(*types.Named)
	if !ok || named.Obj().Pkg() == nil {
		return ""
	}
	rel, ok := strings.CutPrefix(named.Obj().Pkg().Path(), modPath+"/")
	if !ok {
		return ""
	}
	key := rel + "." + named.Obj().Name()
	if _, ok := restrictedTypes[key]; ok {
		return key
	}
	return ""
}

func importPos(p *packages.Package, path string) token.Pos {
	for _, f := range p.Syntax {
		for _, imp := range f.Imports {
			if s, err := strconv.Unquote(imp.Path.Value); err == nil && s == path {
				return imp.Pos()
			}
		}
	}
	return token.NoPos
}

func names(m map[string]bool) string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return strings.Join(out, ", ")
}
