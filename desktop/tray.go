package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log/slog"
	"os"
	"os/exec"
	"runtime"
	"sync"

	"github.com/energye/systray"
)

type trayCallbacks struct {
	OnShow     func()
	OnCopyMCP  func()
	OnReveal   func()
	OnQuit     func()
	MCPURL     string
}

var (
	trayOnce      sync.Once
	trayReady     = make(chan struct{})
	trayQuitFunc  func()
)

func startTray(cb trayCallbacks) {
	trayOnce.Do(func() {
		trayQuitFunc = cb.OnQuit
		go systray.Run(func() { trayOnReady(cb) }, trayOnExit)
	})
}

func trayOnReady(cb trayCallbacks) {
	systray.SetIcon(defaultIcon())
	systray.SetTitle("")
	systray.SetTooltip("WriteKit — local MCP at " + cb.MCPURL)

	mOpen := systray.AddMenuItem("Open WriteKit", "Show the WriteKit window")
	mCopy := systray.AddMenuItem("Copy MCP URL", cb.MCPURL)
	mReveal := systray.AddMenuItem("Reveal data folder", "Open the WriteKit data folder")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Quit WriteKit", "Exit WriteKit")

	mOpen.Click(func() { safeInvoke(cb.OnShow) })
	mCopy.Click(func() { safeInvoke(cb.OnCopyMCP) })
	mReveal.Click(func() { safeInvoke(cb.OnReveal) })
	mQuit.Click(func() { safeInvoke(cb.OnQuit) })

	close(trayReady)
}

func trayOnExit() {
	slog.Info("tray exited")
}

func safeInvoke(f func()) {
	if f == nil {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			slog.Error("tray callback panic", "panic", r)
		}
	}()
	f()
}

// defaultIcon generates a minimal black rounded-square icon so we don't
// need to ship a binary asset. Replace with real branding in a later pass.
func defaultIcon() []byte {
	const size = 32
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	// Black fill
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			img.Set(x, y, color.Black)
		}
	}
	// Carve out corners for a rounded look
	cut := [][2]int{
		{0, 0}, {0, 1}, {1, 0},
		{size - 1, 0}, {size - 1, 1}, {size - 2, 0},
		{0, size - 1}, {0, size - 2}, {1, size - 1},
		{size - 1, size - 1}, {size - 1, size - 2}, {size - 2, size - 1},
	}
	for _, p := range cut {
		img.Set(p[0], p[1], color.RGBA{0, 0, 0, 0})
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil
	}
	return buf.Bytes()
}

func revealInExplorer(path string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", path)
	case "darwin":
		cmd = exec.Command("open", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	if err := cmd.Start(); err != nil {
		slog.Warn("reveal failed", "path", path, "err", err)
	}
}

func copyToClipboardOS(text string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "echo "+text+"|clip")
	case "darwin":
		cmd = exec.Command("pbcopy")
	default:
		cmd = exec.Command("xclip", "-selection", "clipboard")
	}
	if runtime.GOOS != "windows" {
		cmd.Stdin = bytes.NewReader([]byte(text))
	}
	return cmd.Run()
}

func quitTray() {
	defer func() {
		if r := recover(); r != nil {
			slog.Warn("systray.Quit panicked", "panic", r)
		}
	}()
	systray.Quit()
}

// envInfo helps us write useful tooltips/menus
func envInfo(dataDir string) string {
	return fmt.Sprintf("data: %s — pid %d", dataDir, os.Getpid())
}
