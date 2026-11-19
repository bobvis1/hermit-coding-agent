package render

type ApprovedMRText string

func approve(s string) ApprovedMRText { return ApprovedMRText(s) }

var Approve = approve
