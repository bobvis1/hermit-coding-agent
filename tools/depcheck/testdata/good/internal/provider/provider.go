package provider

import "example.com/hermit/internal/tooldef"

func Send(s tooldef.Set) tooldef.Set { return s.Without("edit") }
