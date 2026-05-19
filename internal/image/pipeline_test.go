package image

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"strings"
	"testing"
)

func TestProcessLargePNGDownscales(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 4000, 3000))
	for y := 0; y < 3000; y += 100 {
		for x := 0; x < 4000; x += 100 {
			src.Set(x, y, color.RGBA{R: uint8(x % 255), G: uint8(y % 255), B: 0, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, src); err != nil {
		t.Fatalf("encode src: %v", err)
	}

	out, w, h, frames, err := Process(&buf)
	if err != nil {
		t.Fatalf("process: %v", err)
	}
	if frames != 1 {
		t.Errorf("frames: want 1, got %d", frames)
	}
	if w > MaxDimension || h > MaxDimension {
		t.Errorf("size %dx%d exceeds %d", w, h, MaxDimension)
	}
	if w != MaxDimension {
		t.Errorf("expected longest side scaled to %d, got w=%d h=%d", MaxDimension, w, h)
	}
	if !bytes.Contains(out[:16], []byte("RIFF")) || !bytes.Contains(out[:16], []byte("WEBP")) {
		t.Errorf("not a webp: %x", out[:16])
	}
	ratio := float64(w) / float64(h)
	want := 4000.0 / 3000.0
	if ratio < want*0.99 || ratio > want*1.01 {
		t.Errorf("aspect ratio drift: want %f got %f", want, ratio)
	}
}

func TestProcessAnimatedGIF(t *testing.T) {
	g := &gif.GIF{LoopCount: 0}
	palette := color.Palette{color.RGBA{0, 0, 0, 255}, color.RGBA{255, 0, 0, 255}, color.RGBA{0, 255, 0, 255}}
	for i := 0; i < 3; i++ {
		frame := image.NewPaletted(image.Rect(0, 0, 50, 50), palette)
		for y := 0; y < 50; y++ {
			for x := 0; x < 50; x++ {
				frame.SetColorIndex(x, y, uint8((i+1)%3))
			}
		}
		g.Image = append(g.Image, frame)
		g.Delay = append(g.Delay, 10)
		g.Disposal = append(g.Disposal, gif.DisposalNone)
	}
	g.Config = image.Config{ColorModel: palette, Width: 50, Height: 50}

	var buf bytes.Buffer
	if err := gif.EncodeAll(&buf, g); err != nil {
		t.Fatalf("encode gif: %v", err)
	}

	out, w, h, frames, err := Process(&buf)
	if err != nil {
		t.Fatalf("process: %v", err)
	}
	if frames != 3 {
		t.Errorf("frames: want 3, got %d", frames)
	}
	if w != 50 || h != 50 {
		t.Errorf("size: want 50x50, got %dx%d", w, h)
	}
	if !bytes.Contains(out[:16], []byte("WEBP")) {
		t.Errorf("not a webp: %x", out[:16])
	}
}

func TestProcessRejectsNonImage(t *testing.T) {
	_, _, _, _, err := Process(strings.NewReader("this is not an image"))
	if err == nil {
		t.Fatal("want error for non-image input")
	}
	if !strings.Contains(err.Error(), "unsupported content type") {
		t.Errorf("want content-type rejection, got: %v", err)
	}
}

func TestDimScaleClampsLongestSide(t *testing.T) {
	if s := dimScale(4000, 3000, 2048); s >= 1 || s <= 0 {
		t.Errorf("4000x3000 → 2048: got scale %f", s)
	}
	if s := dimScale(100, 100, 2048); s != 1 {
		t.Errorf("100x100 → 2048: want 1, got %f", s)
	}
	if s := dimScale(3000, 4000, 2048); s >= 1 || s <= 0 {
		t.Errorf("3000x4000 → 2048: got scale %f", s)
	}
}
