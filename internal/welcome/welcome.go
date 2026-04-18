package welcome

import (
	_ "embed"
)

//go:embed welcome.md
var Markdown string

const (
	Title      = "Welcome to WriteKit"
	Slug       = "welcome"
	Excerpt    = "WriteKit is a publishing platform you drive with your AI assistant. This is your first post — private to you until you delete or publish it."
	Visibility = "private"
)
