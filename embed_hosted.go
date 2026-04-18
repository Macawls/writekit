//go:build !desktop

package writekit

import "embed"

//go:embed all:apps/admin/dist
var AdminFS embed.FS
