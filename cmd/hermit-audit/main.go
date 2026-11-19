// Command hermit-audit is organization-run tooling, never installed on
// developer machines: log cross-checks, anchor checks, alerts and
// investigator search (M16).
//
// Nothing is implemented yet; see IMPLEMENTATION_PLAN.md, milestone 8.
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "hermit-audit: not yet implemented")
	os.Exit(2)
}
