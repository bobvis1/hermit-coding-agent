package loop

import (
	"os"

	"example.com/hermit/internal/tooldef"
)

func Run() {
	_, _ = os.ReadFile("x")
	_, _ = os.StartProcess("/bin/sh", nil, nil)
	_ = tooldef.NewSet("shell")
	os.Exit(0)
}
