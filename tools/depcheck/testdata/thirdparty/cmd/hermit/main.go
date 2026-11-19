package main

import (
	_ "example.com/evil"
	_ "example.com/evil/spawn"
	_ "example.com/hermit/internal/credstore"
	_ "example.com/hermit/internal/worktree"
	_ "github.com/aws/aws-sdk-go-v2/config"
)

func main() {}
