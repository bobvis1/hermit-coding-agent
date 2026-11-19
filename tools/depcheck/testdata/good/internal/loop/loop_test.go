package loop

import (
	"os"
	"os/exec"
	"testing"
)

// Test files may do what the rules forbid.
func TestFixture(t *testing.T) {
	_, _ = os.ReadFile("x")
	_ = exec.Command("true").Run()
}
