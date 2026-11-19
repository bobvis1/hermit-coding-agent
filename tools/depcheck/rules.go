package depcheck

// The rules below implement IMPLEMENTATION_PLAN.md, section 1 ("Required
// paths" and "Other dependency rules") and milestone 0. Package names are
// relative to the checked module's path. Every list is an allowlist where
// that is practical, so that something new is refused until it is reviewed.

const (
	pkgGitcmd    = "internal/gitcmd"
	pkgFsguard   = "internal/fsguard"
	pkgLocal     = "internal/localstore"
	pkgPolicy    = "internal/policy"
	pkgCredstore = "internal/credstore"
	pkgNetclient = "internal/netclient"
	pkgProvider  = "internal/provider"
	pkgTooldef   = "internal/tooldef"
	pkgTools     = "internal/tools"
	pkgRender    = "internal/render"
	pkgTUI       = "internal/tui"
)

// fileSystemPackages may use file-system APIs. Only fsguard is given
// repository paths; review checks that.
var fileSystemPackages = set(pkgFsguard, pkgLocal, pkgPolicy)

// systemCallPackages may import syscall, golang.org/x/sys and unsafe.
// credstore needs system calls for the credential stores, not for files.
var systemCallPackages = set(pkgFsguard, pkgLocal, pkgPolicy, pkgCredstore)

// puregoPackages may import purego (approved list, IMPLEMENTATION_PLAN.md).
// The listener check (milestone 7) is added here when it is written.
var puregoPackages = set(pkgCredstore)

// selfContained import nothing else from Hermit.
var selfContained = set(pkgFsguard, "internal/scan", pkgRender)

// providerMayNotReach are packages provider may not import, directly or
// indirectly, so that it has no source of content other than scan.
var providerMayNotReach = set(pkgFsguard, pkgTools, "internal/scm", "internal/testresults")

// restrictedTypes may be constructed only by the listed packages: by using a
// function, method or function-typed variable of the type's own package that
// returns one, by a composite literal, or by a conversion. Keys are relative
// to the module path.
var restrictedTypes = map[string]map[string]bool{
	pkgTooldef + ".Set":           set(pkgTooldef, pkgTools),
	pkgRender + ".ApprovedMRText": set(pkgRender, pkgTUI),
}

// osAllowed are the package-level functions and variables of package os that
// code outside fileSystemPackages may use. Types and constants are allowed.
var osAllowed = set(
	"Args", "Exit", "Getenv", "LookupEnv", "Environ",
	"Stdin", "Stdout", "Stderr",
	"Getpid", "Getppid", "Getuid", "Geteuid", "Getgid", "Getegid", "Getpagesize", "Hostname",
	"Interrupt", "Kill",
	"IsExist", "IsNotExist", "IsPermission", "IsTimeout", "NewSyscallError",
	"ErrInvalid", "ErrPermission", "ErrExist", "ErrNotExist", "ErrClosed",
	"ErrNoDeadline", "ErrDeadlineExceeded", "ErrProcessDone",
)

// ioFSAllowed are the io/fs functions code outside fileSystemPackages may use.
var ioFSAllowed = set("ValidPath", "FormatFileInfo", "FormatDirEntry", "FileInfoToDirEntry")

// filepathDenied are path/filepath functions that touch the file system.
var filepathDenied = set("Walk", "WalkDir", "Glob", "EvalSymlinks", "Abs")

// fileFuncsDenied are other standard library functions and methods that open
// files by name, as types.Func.FullName prints them.
var fileFuncsDenied = set(
	"archive/zip.OpenReader",
	"crypto/tls.LoadX509KeyPair",
	"debug/elf.Open", "debug/macho.Open", "debug/macho.OpenFat", "debug/pe.Open", "debug/plan9obj.Open",
	"go/parser.ParseDir",
	"html/template.ParseFiles", "html/template.ParseGlob",
	"(*html/template.Template).ParseFiles", "(*html/template.Template).ParseGlob",
	"text/template.ParseFiles", "text/template.ParseGlob",
	"(*text/template.Template).ParseFiles", "(*text/template.Template).ParseGlob",
)

// fileImportsDenied are packages whose purpose is file access.
var fileImportsDenied = set("io/ioutil", "os/user")

// processObjects start a process. Keys are package path and object name.
var processObjects = set(
	"os.StartProcess",
	"syscall.StartProcess", "syscall.ForkExec", "syscall.Exec",
	"syscall.CreateProcess", "syscall.CreateProcessAsUser",
	"syscall.SYS_EXECVE", "syscall.SYS_EXECVEAT", "syscall.SYS_FORK", "syscall.SYS_VFORK",
	"syscall.SYS_CLONE", "syscall.SYS_CLONE3", "syscall.SYS_POSIX_SPAWN",
	"golang.org/x/sys/unix.Exec", "golang.org/x/sys/unix.ForkExec",
	"golang.org/x/sys/unix.PosixSpawn", "golang.org/x/sys/unix.PosixSpawnp",
	"golang.org/x/sys/unix.SYS_EXECVE", "golang.org/x/sys/unix.SYS_EXECVEAT",
	"golang.org/x/sys/unix.SYS_FORK", "golang.org/x/sys/unix.SYS_VFORK",
	"golang.org/x/sys/unix.SYS_CLONE", "golang.org/x/sys/unix.SYS_CLONE3",
	"golang.org/x/sys/unix.SYS_POSIX_SPAWN",
	"golang.org/x/sys/windows.CreateProcess", "golang.org/x/sys/windows.CreateProcessAsUser",
	"golang.org/x/sys/windows.ShellExecute", "golang.org/x/sys/windows.ShellExecuteEx",
)

// processSymbols are C and Windows function names that start a process. A
// string literal equal to one of them, in a package that can load functions
// dynamically (purego, syscall, golang.org/x/sys/windows), is refused.
var processSymbols = set(
	"posix_spawn", "posix_spawnp", "execve", "execv", "execvp", "execvpe", "execl", "execlp", "execle",
	"fexecve", "system", "popen",
	"CreateProcessW", "CreateProcessA", "CreateProcessAsUserW", "CreateProcessAsUserA",
	"CreateProcessWithLogonW", "CreateProcessWithTokenW",
	"ShellExecuteW", "ShellExecuteA", "ShellExecuteExW", "ShellExecuteExA", "WinExec",
)

// dynamicLoaders are imports that let a package call functions by name.
var dynamicLoaders = set("github.com/ebitengine/purego", "syscall", "golang.org/x/sys/windows")

// forbiddenPackages may never be linked, whatever imports them
// (IMPLEMENTATION_PLAN.md, approved third-party code).
var forbiddenPackages = map[string]string{
	"github.com/aws/aws-sdk-go-v2/config":                   "its default loading runs credential_process programs",
	"github.com/aws/aws-sdk-go-v2/credentials/processcreds": "it runs credential_process programs",
}

// compatibleLicenses are the SPDX identifiers accepted on the approved list,
// as compatible with Apache-2.0.
var compatibleLicenses = set("Apache-2.0", "MIT", "BSD-2-Clause", "BSD-3-Clause", "ISC", "MIT AND Apache-2.0")

func set(items ...string) map[string]bool {
	m := make(map[string]bool, len(items))
	for _, s := range items {
		m[s] = true
	}
	return m
}
