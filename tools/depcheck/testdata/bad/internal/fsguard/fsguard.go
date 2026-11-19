package fsguard

import (
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
)

func Read(name string) ([]byte, error) {
	_ = filepath.WalkDir(".", func(string, fs.DirEntry, error) error { return nil })
	_ = syscall.Getpid()
	return os.ReadFile(name)
}
