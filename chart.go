package main

import (
	"image"
	"image/color"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
)

// Chart is a minimal auto-scaling line-chart widget for a rolling window
// of float64 samples.
type Chart struct {
	widget.BaseWidget

	mu        sync.Mutex
	data      []float64
	maxPoints int
}

func NewChart(maxPoints int) *Chart {
	c := &Chart{maxPoints: maxPoints}
	c.ExtendBaseWidget(c)
	return c
}

func (c *Chart) Push(v float64) {
	c.mu.Lock()
	c.data = append(c.data, v)
	if len(c.data) > c.maxPoints {
		c.data = c.data[len(c.data)-c.maxPoints:]
	}
	c.mu.Unlock()
	c.Refresh()
}

func (c *Chart) Clear() {
	c.mu.Lock()
	c.data = nil
	c.mu.Unlock()
	c.Refresh()
}

func (c *Chart) CreateRenderer() fyne.WidgetRenderer {
	raster := canvas.NewRaster(c.draw)
	bg := canvas.NewRectangle(color.RGBA{R: 24, G: 26, B: 30, A: 255})
	return &chartRenderer{chart: c, raster: raster, bg: bg, objects: []fyne.CanvasObject{bg, raster}}
}

var (
	chartLineColor = color.RGBA{R: 80, G: 200, B: 255, A: 255}
	chartZeroColor = color.RGBA{R: 90, G: 95, B: 105, A: 255}
)

func (c *Chart) draw(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	c.mu.Lock()
	data := make([]float64, len(c.data))
	copy(data, c.data)
	c.mu.Unlock()

	if len(data) == 0 || w <= 1 || h <= 1 {
		return img
	}

	lo, hi := data[0], data[0]
	for _, v := range data {
		if v < lo {
			lo = v
		}
		if v > hi {
			hi = v
		}
	}
	span := hi - lo
	if span == 0 {
		span = 1
	}
	pad := span * 0.1
	lo -= pad
	hi += pad
	span = hi - lo

	yFor := func(v float64) int {
		norm := (v - lo) / span // 0..1
		y := h - 1 - int(norm*float64(h-1))
		if y < 0 {
			y = 0
		}
		if y > h-1 {
			y = h - 1
		}
		return y
	}

	// zero line, if in range
	if lo <= 0 && hi >= 0 {
		zy := yFor(0)
		for x := 0; x < w; x++ {
			img.Set(x, zy, chartZeroColor)
		}
	}

	xStep := float64(w-1) / float64(max(len(data)-1, 1))
	prevX, prevY := 0, yFor(data[0])
	for i, v := range data {
		x := int(float64(i) * xStep)
		y := yFor(v)
		if i > 0 {
			drawLine(img, prevX, prevY, x, y, chartLineColor)
		}
		prevX, prevY = x, y
	}
	return img
}

// drawLine draws a simple Bresenham line into img.
func drawLine(img *image.RGBA, x0, y0, x1, y1 int, col color.Color) {
	dx := abs(x1 - x0)
	dy := -abs(y1 - y0)
	sx, sy := 1, 1
	if x0 > x1 {
		sx = -1
	}
	if y0 > y1 {
		sy = -1
	}
	err := dx + dy
	for {
		img.Set(x0, y0, col)
		if x0 == x1 && y0 == y1 {
			break
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x0 += sx
		}
		if e2 <= dx {
			err += dx
			y0 += sy
		}
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

type chartRenderer struct {
	chart   *Chart
	raster  *canvas.Raster
	bg      *canvas.Rectangle
	objects []fyne.CanvasObject
}

func (r *chartRenderer) Layout(size fyne.Size) {
	r.bg.Resize(size)
	r.raster.Resize(size)
}

func (r *chartRenderer) MinSize() fyne.Size          { return fyne.NewSize(200, 100) }
func (r *chartRenderer) Refresh()                    { canvas.Refresh(r.raster) }
func (r *chartRenderer) Objects() []fyne.CanvasObject { return r.objects }
func (r *chartRenderer) Destroy()                     {}
