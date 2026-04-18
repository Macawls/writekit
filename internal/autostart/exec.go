package autostart

import "os"

func execCommand() []string {
	exe, err := os.Executable()
	if err != nil {
		return []string{"writekit"}
	}
	return []string{exe}
}
