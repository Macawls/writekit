package image

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"

	"github.com/HugoSmits86/nativewebp"
	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

const (
	MaxInputBytes = 20 << 20
	MaxPixels     = 100_000_000
	MaxDimension  = 2048
)

func Process(src io.Reader) (webpBytes []byte, w, h, frameCount int, err error) {
	raw, err := io.ReadAll(io.LimitReader(src, MaxInputBytes+1))
	if err != nil {
		return nil, 0, 0, 0, fmt.Errorf("read input: %w", err)
	}
	if len(raw) > MaxInputBytes {
		return nil, 0, 0, 0, fmt.Errorf("input exceeds %d bytes", MaxInputBytes)
	}
	if len(raw) == 0 {
		return nil, 0, 0, 0, fmt.Errorf("empty input")
	}

	mime := http.DetectContentType(raw[:min(len(raw), 512)])
	switch mime {
	case "image/png", "image/jpeg", "image/gif", "image/webp":
	default:
		return nil, 0, 0, 0, fmt.Errorf("unsupported content type: %s", mime)
	}

	cfg, _, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return nil, 0, 0, 0, fmt.Errorf("decode config: %w", err)
	}
	if cfg.Width*cfg.Height > MaxPixels {
		return nil, 0, 0, 0, fmt.Errorf("image too large: %dx%d exceeds %d pixels", cfg.Width, cfg.Height, MaxPixels)
	}

	if mime == "image/gif" {
		g, gifErr := gif.DecodeAll(bytes.NewReader(raw))
		if gifErr == nil && len(g.Image) > 1 {
			return encodeAnimated(g)
		}
	}

	return encodeStill(raw)
}

func encodeStill(raw []byte) ([]byte, int, int, int, error) {
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, 0, 0, 0, fmt.Errorf("decode: %w", err)
	}
	scaled := scaleToMax(img, MaxDimension)
	var buf bytes.Buffer
	if err := nativewebp.Encode(&buf, scaled, nil); err != nil {
		return nil, 0, 0, 0, fmt.Errorf("encode webp: %w", err)
	}
	b := scaled.Bounds()
	return buf.Bytes(), b.Dx(), b.Dy(), 1, nil
}

func encodeAnimated(g *gif.GIF) ([]byte, int, int, int, error) {
	canvasW, canvasH := g.Config.Width, g.Config.Height
	if canvasW == 0 || canvasH == 0 {
		b := g.Image[0].Bounds()
		canvasW, canvasH = b.Dx(), b.Dy()
	}
	canvas := image.NewRGBA(image.Rect(0, 0, canvasW, canvasH))
	if g.BackgroundIndex > 0 && len(g.Image[0].Palette) > int(g.BackgroundIndex) {
		bg := g.Image[0].Palette[g.BackgroundIndex]
		draw.Draw(canvas, canvas.Bounds(), &image.Uniform{C: bg}, image.Point{}, draw.Src)
	}

	scale := dimScale(canvasW, canvasH, MaxDimension)
	outW := int(float64(canvasW) * scale)
	outH := int(float64(canvasH) * scale)
	if outW < 1 {
		outW = 1
	}
	if outH < 1 {
		outH = 1
	}

	frames := make([]image.Image, 0, len(g.Image))
	durations := make([]uint, 0, len(g.Image))
	disposals := make([]uint, 0, len(g.Image))

	var prevCanvas *image.RGBA
	for i, frame := range g.Image {
		var snapshotSrc *image.RGBA
		switch g.Disposal[i] {
		case gif.DisposalPrevious:
			snapshotSrc = image.NewRGBA(canvas.Bounds())
			copy(snapshotSrc.Pix, canvas.Pix)
		}

		draw.Draw(canvas, frame.Bounds(), frame, frame.Bounds().Min, draw.Over)

		frameSnapshot := image.NewRGBA(canvas.Bounds())
		copy(frameSnapshot.Pix, canvas.Pix)

		scaled := image.NewRGBA(image.Rect(0, 0, outW, outH))
		xdraw.CatmullRom.Scale(scaled, scaled.Bounds(), frameSnapshot, frameSnapshot.Bounds(), xdraw.Over, nil)
		frames = append(frames, scaled)

		delay := uint(g.Delay[i] * 10)
		if delay == 0 {
			delay = 100
		}
		durations = append(durations, delay)

		switch g.Disposal[i] {
		case gif.DisposalBackground:
			draw.Draw(canvas, frame.Bounds(), image.Transparent, image.Point{}, draw.Src)
			disposals = append(disposals, 1)
		case gif.DisposalPrevious:
			if snapshotSrc != nil {
				copy(canvas.Pix, snapshotSrc.Pix)
			} else if prevCanvas != nil {
				copy(canvas.Pix, prevCanvas.Pix)
			}
			disposals = append(disposals, 0)
		default:
			disposals = append(disposals, 0)
		}

		prevCanvas = image.NewRGBA(canvas.Bounds())
		copy(prevCanvas.Pix, canvas.Pix)
	}

	loop := uint16(0)
	if g.LoopCount > 0 {
		loop = uint16(g.LoopCount)
	}
	anim := &nativewebp.Animation{
		Images:    frames,
		Durations: durations,
		Disposals: disposals,
		LoopCount: loop,
	}
	var buf bytes.Buffer
	if err := nativewebp.EncodeAll(&buf, anim, &nativewebp.Options{UseExtendedFormat: true}); err != nil {
		return nil, 0, 0, 0, fmt.Errorf("encode animated webp: %w", err)
	}
	return buf.Bytes(), outW, outH, len(frames), nil
}

func scaleToMax(src image.Image, maxDim int) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= maxDim && h <= maxDim {
		return src
	}
	scale := dimScale(w, h, maxDim)
	outW := int(float64(w) * scale)
	outH := int(float64(h) * scale)
	if outW < 1 {
		outW = 1
	}
	if outH < 1 {
		outH = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, outW, outH))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, b, xdraw.Over, nil)
	return dst
}

func dimScale(w, h, maxDim int) float64 {
	longest := w
	if h > longest {
		longest = h
	}
	if longest <= maxDim {
		return 1
	}
	return float64(maxDim) / float64(longest)
}
