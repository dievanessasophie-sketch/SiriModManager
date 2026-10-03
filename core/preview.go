package core

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
)

type Preview struct {
	Width, Height int
	BGRA          []byte
}

func FetchPreview(ctx context.Context, client *http.Client, raw string) (*Preview, error) {
	if !validURL(raw) {
		return nil, fmt.Errorf("Ungültige Bildadresse")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "SiriModManager/"+Version)
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Vorschaubild HTTP %d", res.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(res.Body, (8<<20)+1))
	if err != nil {
		return nil, err
	}
	if len(b) > 8<<20 {
		return nil, fmt.Errorf("Vorschaubild ist zu groß")
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	if cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 8192 || cfg.Height > 8192 || int64(cfg.Width)*int64(cfg.Height) > 20_000_000 {
		return nil, fmt.Errorf("Bildabmessungen sind zu groß")
	}
	im, _, err := image.Decode(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	w, h := cfg.Width, cfg.Height
	if w > 1600 {
		h = max(1, h*1600/w)
		w = 1600
	}
	if h > 1000 {
		w = max(1, w*1000/h)
		h = 1000
	}
	p := &Preview{Width: w, Height: h, BGRA: make([]byte, w*h*4)}
	bounds := im.Bounds()
	for y := 0; y < h; y++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		for x := 0; x < w; x++ {
			r, g, b, a := im.At(bounds.Min.X+x*cfg.Width/w, bounds.Min.Y+y*cfg.Height/h).RGBA()
			n := (y*w + x) * 4
			p.BGRA[n], p.BGRA[n+1], p.BGRA[n+2], p.BGRA[n+3] = byte((b+65535-a)>>8), byte((g+65535-a)>>8), byte((r+65535-a)>>8), 255
		}
	}
	return p, nil
}
