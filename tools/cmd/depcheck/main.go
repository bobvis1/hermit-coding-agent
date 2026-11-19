// Command depcheck enforces Hermit's dependency rules; see package depcheck.
//
// Usage: depcheck -approved deps/approved-modules.txt -goos darwin,linux,windows [dir]
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/bobvis1/hermit-coding-agent/tools/depcheck"
)

func main() {
	approved := flag.String("approved", "deps/approved-modules.txt", "approved third-party modules list")
	goos := flag.String("goos", "darwin,linux,windows", "comma-separated targets to check")
	flag.Parse()
	dir := "."
	if flag.NArg() > 0 {
		dir = flag.Arg(0)
	}
	vs, err := depcheck.Run(depcheck.Config{Dir: dir, GOOS: strings.Split(*goos, ","), ApprovedFile: *approved})
	if err != nil {
		fmt.Fprintln(os.Stderr, "depcheck:", err)
		os.Exit(2)
	}
	for _, v := range vs {
		fmt.Println(v)
	}
	if len(vs) > 0 {
		fmt.Fprintf(os.Stderr, "depcheck: %d violation(s) of the dependency rules (IMPLEMENTATION_PLAN.md, section 1)\n", len(vs))
		os.Exit(1)
	}
}
