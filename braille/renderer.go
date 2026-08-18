package braille

import (
	"strings"
)

// Braille Unicode patterns start at U+2800
// Each braille character can represent 8 dots (2x4 grid)
// Dot positions:
//  1  4
//  2  5
//  3  6
//  7  8
const brailleBase = 0x2800

// Canvas represents a drawable surface using braille characters
type Canvas struct {
	Width  int // Width in pixels (not characters)
	Height int // Height in pixels (not characters)
	pixels [][]bool
}

// NewCanvas creates a new braille canvas with the given pixel dimensions
func NewCanvas(width, height int) *Canvas {
	pixels := make([][]bool, height)
	for i := range pixels {
		pixels[i] = make([]bool, width)
	}
	return &Canvas{
		Width:  width,
		Height: height,
		pixels: pixels,
	}
}

// Set sets a pixel at the given coordinates
func (c *Canvas) Set(x, y int) {
	if x >= 0 && x < c.Width && y >= 0 && y < c.Height {
		c.pixels[y][x] = true
	}
}

// Unset clears a pixel at the given coordinates
func (c *Canvas) Unset(x, y int) {
	if x >= 0 && x < c.Width && y >= 0 && y < c.Height {
		c.pixels[y][x] = false
	}
}

// Get returns the state of a pixel at the given coordinates
func (c *Canvas) Get(x, y int) bool {
	if x >= 0 && x < c.Width && y >= 0 && y < c.Height {
		return c.pixels[y][x]
	}
	return false
}

// Clear clears all pixels on the canvas
func (c *Canvas) Clear() {
	for y := range c.pixels {
		for x := range c.pixels[y] {
			c.pixels[y][x] = false
		}
	}
}

// DrawLine draws a line from (x0, y0) to (x1, y1) using Bresenham's algorithm
func (c *Canvas) DrawLine(x0, y0, x1, y1 int) {
	dx := abs(x1 - x0)
	dy := abs(y1 - y0)
	sx := 1
	if x0 > x1 {
		sx = -1
	}
	sy := 1
	if y0 > y1 {
		sy = -1
	}
	err := dx - dy

	for {
		c.Set(x0, y0)
		if x0 == x1 && y0 == y1 {
			break
		}
		e2 := 2 * err
		if e2 > -dy {
			err -= dy
			x0 += sx
		}
		if e2 < dx {
			err += dx
			y0 += sy
		}
	}
}

// DrawCircle draws a circle with center (cx, cy) and radius r
func (c *Canvas) DrawCircle(cx, cy, r int) {
	x := 0
	y := r
	d := 3 - 2*r

	for x <= y {
		c.setCirclePoints(cx, cy, x, y)
		if d < 0 {
			d = d + 4*x + 6
		} else {
			d = d + 4*(x-y) + 10
			y--
		}
		x++
	}
}

func (c *Canvas) setCirclePoints(cx, cy, x, y int) {
	c.Set(cx+x, cy+y)
	c.Set(cx-x, cy+y)
	c.Set(cx+x, cy-y)
	c.Set(cx-x, cy-y)
	c.Set(cx+y, cy+x)
	c.Set(cx-y, cy+x)
	c.Set(cx+y, cy-x)
	c.Set(cx-y, cy-x)
}

// FillCircle draws a filled circle with center (cx, cy) and radius r
func (c *Canvas) FillCircle(cx, cy, r int) {
	for y := -r; y <= r; y++ {
		for x := -r; x <= r; x++ {
			if x*x+y*y <= r*r {
				c.Set(cx+x, cy+y)
			}
		}
	}
}

// Render converts the canvas to a string of braille characters
func (c *Canvas) Render() string {
	charWidth := (c.Width + 1) / 2   // Each braille char is 2 pixels wide
	charHeight := (c.Height + 3) / 4 // Each braille char is 4 pixels tall

	var result strings.Builder
	result.Grow(charHeight * (charWidth + 1)) // +1 for newlines

	for cy := 0; cy < charHeight; cy++ {
		for cx := 0; cx < charWidth; cx++ {
			// Calculate which pixels map to this braille character
			pattern := 0

			// Braille dot mapping:
			dotMap := []struct{ dx, dy, bit int }{
				{0, 0, 0}, // dot 1
				{0, 1, 1}, // dot 2
				{0, 2, 2}, // dot 3
				{0, 3, 6}, // dot 7
				{1, 0, 3}, // dot 4
				{1, 1, 4}, // dot 5
				{1, 2, 5}, // dot 6
				{1, 3, 7}, // dot 8
			}

			for _, dm := range dotMap {
				px := cx*2 + dm.dx
				py := cy*4 + dm.dy
				if c.Get(px, py) {
					pattern |= (1 << dm.bit)
				}
			}

			result.WriteRune(rune(brailleBase + pattern))
		}
		if cy < charHeight-1 {
			result.WriteRune('\n')
		}
	}

	return result.String()
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
