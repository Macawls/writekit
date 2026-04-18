package writekit

import "embed"

//go:embed all:templates
var TemplatesFS embed.FS

//go:embed all:static
var StaticFS embed.FS

//go:embed all:apps/user/dist
var AppFS embed.FS
