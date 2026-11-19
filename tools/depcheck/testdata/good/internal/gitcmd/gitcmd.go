package gitcmd

import "os/exec"

func Remote() ([]byte, error) { return exec.Command("git", "remote", "-v").Output() }
