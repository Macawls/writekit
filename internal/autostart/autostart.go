package autostart

import (
	"github.com/emersion/go-autostart"
)

const (
	appName  = "WriteKit"
	appLabel = "WriteKit"
)

func app() *autostart.App {
	return &autostart.App{
		Name:        appName,
		DisplayName: appLabel,
		Exec:        execCommand(),
	}
}

func IsEnabled() bool {
	return app().IsEnabled()
}

func Enable() error {
	a := app()
	if a.IsEnabled() {
		return nil
	}
	return a.Enable()
}

func Disable() error {
	a := app()
	if !a.IsEnabled() {
		return nil
	}
	return a.Disable()
}

func Set(enabled bool) error {
	if enabled {
		return Enable()
	}
	return Disable()
}
