// Command pincheck fails if a GitHub Actions workflow refers to an action by
// tag or branch rather than by commit hash; see package pincheck.
//
// Usage: pincheck [dir]   (default .github)
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/bobvis1/hermit-coding-agent/tools/pincheck"
)

func main() {
	flag.Parse()
	dir := ".github"
	if flag.NArg() > 0 {
		dir = flag.Arg(0)
	}
	vs, err := pincheck.CheckDir(dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "pincheck:", err)
		os.Exit(2)
	}
	for _, v := range vs {
		fmt.Println(v)
	}
	if len(vs) > 0 {
		os.Exit(1)
	}
}
