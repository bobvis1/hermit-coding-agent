package render

import (
	"syscall"

	"example.com/hermit/internal/tooldef"
)

type ApprovedMRText string

var _ = syscall.Getpid
var _ tooldef.Set

func approve(s string) ApprovedMRText { return ApprovedMRText(s) }

var Approve = approve
