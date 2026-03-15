package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"time"

	xfont "golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

func newFace(size float64) (xfont.Face, error) {
	ttf, err := opentype.Parse(gobold.TTF)
	if err != nil {
		return nil, fmt.Errorf("parse font: %w", err)
	}
	face, err := opentype.NewFace(ttf, &opentype.FaceOptions{
		Size: size,
		DPI:  72,
	})
	if err != nil {
		return nil, fmt.Errorf("new face: %w", err)
	}
	return face, nil
}

func measureString(face xfont.Face, s string) int {
	d := xfont.Drawer{Face: face}
	return int(d.MeasureString(s) >> 6)
}

func drawString(img *image.RGBA, face xfont.Face, x, y int, s string, clr color.Color) {
	d := xfont.Drawer{
		Dst:  img,
		Src:  image.NewUniform(clr),
		Face: face,
		Dot:  fixed.P(x, y),
	}
	d.DrawString(s)
}

func renderFrame(img *image.RGBA, bigFace, smallFace xfont.Face, tt *timeTable, weatherStr string) {
	const border = 20
	w := img.Bounds().Max.X
	h := img.Bounds().Max.Y

	draw.Draw(img, img.Bounds(), image.NewUniform(color.Black), image.Point{}, draw.Src)

	grey := color.Gray{Y: 180}
	info := tt.getStopInfo()
	header := info.Name
	if info.Towards != "" {
		header += " - To: " + info.Towards
	}
	if header != "" {
		drawString(img, smallFace, border, 28, header, grey)
	}

	buses := tt.getBuses()
	y := 150
	count := 0
	for _, b := range buses {
		if count >= 3 {
			break
		}
		d := fromTime(b.ETA)
		if d < 0 {
			continue
		}
		drawString(img, bigFace, border, y, b.Number, color.White)

		etaStr := d.String()
		rightX := w - border - measureString(bigFace, etaStr)
		drawString(img, bigFace, rightX, y, etaStr, color.White)

		y += 130
		count++
	}

	bottomY := h - 20
	drawString(img, smallFace, border, bottomY, weatherStr, color.White)

	timeStr := time.Now().Format("3:04:05")
	drawString(img, smallFace, w-border-measureString(smallFace, timeStr), bottomY, timeStr, color.White)
}
