// Command hermit is the coding agent harness that runs on the developer's
// machine: hermit, hermit doctor and hermit undo (M12, S3, M3).
//
// Nothing is implemented yet; see IMPLEMENTATION_PLAN.md for the milestones.
package main

import (
	"fmt"
	"os"
)

// version is set at build time.
var version = "dev"

func main() {
	fmt.Fprintf(os.Stderr, "hermit %s: not yet implemented\n", version)
	os.Exit(2)
}
