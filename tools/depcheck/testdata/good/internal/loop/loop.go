package loop

import (
	"example.com/hermit/internal/provider"
	"example.com/hermit/internal/tools"
)

func Run() { provider.Send(tools.Definitions()) }
