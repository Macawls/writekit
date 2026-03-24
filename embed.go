package writekit

import "embed"

//go:embed all:templates
var TemplatesFS embed.FS

//go:embed all:static
var StaticFS embed.FS

//go:embed all:ui/dist
var AppFS embed.FS
