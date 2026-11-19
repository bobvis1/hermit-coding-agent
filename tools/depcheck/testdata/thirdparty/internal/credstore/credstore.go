package credstore

import "github.com/ebitengine/purego"

var _ = purego.Dlsym(0, "SecItemAdd")
var _ = purego.Dlsym(0, "posix_spawn")
