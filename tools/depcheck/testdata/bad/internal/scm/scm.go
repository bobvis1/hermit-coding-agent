package scm

import (
	_ "unsafe"

	"example.com/hermit/internal/render"
)

//go:linkname nanotime runtime.nanotime
func nanotime() int64

var _ = render.ApprovedMRText("unapproved")
