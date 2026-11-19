package tools

import "example.com/hermit/internal/tooldef"

func Definitions() tooldef.Set { return tooldef.NewSet("read", "edit") }
