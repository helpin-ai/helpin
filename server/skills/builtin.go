package skills

import "embed"

const BuiltInRoot = "system"

//go:embed system
var BuiltIn embed.FS
