package tools

import (
	"os/exec"

	"example.com/hermit/internal/fsguard"
)

var _ = exec.Command
var _ = fsguard.Read
