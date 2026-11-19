package tui

import (
	"os"

	"example.com/hermit/internal/render"
)

func Show(s string) render.ApprovedMRText {
	_, _ = os.Stdout.WriteString(s)
	return render.ApprovedMRText(s) + render.Approve(s)
}
