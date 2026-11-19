module example.com/hermit

go 1.25.0

require (
	example.com/evil v0.0.0
	github.com/aws/aws-sdk-go-v2 v0.0.0
	github.com/ebitengine/purego v0.0.0
)

replace (
	example.com/evil => ./mods/evil
	github.com/aws/aws-sdk-go-v2 => ./mods/aws
	github.com/ebitengine/purego => ./mods/purego
)
