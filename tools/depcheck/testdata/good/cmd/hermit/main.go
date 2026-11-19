package main

import (
	"fmt"
	"os"

	"example.com/hermit/internal/fsguard"
	"example.com/hermit/internal/gitcmd"
	"example.com/hermit/internal/loop"
	"example.com/hermit/internal/netclient"
	"example.com/hermit/internal/tui"
)

func main() {
	fmt.Fprintln(os.Stderr, os.Getenv("HOME"), os.Args, fsguard.Read, gitcmd.Remote, netclient.New, tui.Show, loop.Run)
	os.Exit(0)
}
