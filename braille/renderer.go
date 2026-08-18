package braille

import (
	"fmt"
	"strings"
)

// Braille Unicode patterns start at U+2800
// Each braille character can represent 8 dots (2x4 grid)
// Dot positions:
//
//	1  4
//	2  5
//	3  6
//	7  8
const brailleBase = 0x2800

// Color represents an ANSI color code
type Color int

const (
	ColorReset   Color = 0
	ColorRed     Color = 31
	ColorGreen   Color = 32
	ColorYellow  Color = 33
	ColorBlue    Color = 34
	ColorMagenta Color = 35
	ColorCyan    Color = 36
	ColorWhite   Color = 37
)

// Canvas represents a drawable surface using braille characters
type Canvas struct {
	Width  int // Width in pixels (not characters)
	Height int // Height in pixels (not characters)
	pixels [][]bool
	colors [][]Color // Color for each pixel
}

// NewCanvas creates a new braille canvas with the given pixel dimensions
func NewCanvas(width, height int) *Canvas {
	pixels := make([][]bool, height)
	colors := make([][]Color, height)
	for i := range pixels {
		pixels[i] = make([]bool, width)
		colors[i] = make([]Color, width)
	}
	return &Canvas{
		Width:  width,
		Height: height,
		pixels: pixels,
		colors: colors,
	}
}

// Set sets a pixel at the given coordinates
func (c *Canvas) Set(x, y int) {
	c.SetWithColor(x, y, ColorWhite)
}

// SetWithColor sets a pixel at the given coordinates with a specific color
func (c *Canvas) SetWithColor(x, y int, color Color) {
	if x >= 0 && x < c.Width && y >= 0 && y < c.Height {
		c.pixels[y][x] = true
		c.colors[y][x] = color
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
			c.colors[y][x] = ColorWhite
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
	c.FillCircleWithColor(cx, cy, r, ColorWhite)
}

// FillCircleWithColor draws a filled circle with center (cx, cy), radius r, and specific color
func (c *Canvas) FillCircleWithColor(cx, cy, r int, color Color) {
	for y := -r; y <= r; y++ {
		for x := -r; x <= r; x++ {
			if x*x+y*y <= r*r {
				c.SetWithColor(cx+x, cy+y, color)
			}
		}
	}
}

// Render converts the canvas to a string of braille characters with ANSI colors
func (c *Canvas) Render() string {
	charWidth := (c.Width + 1) / 2   // Each braille char is 2 pixels wide
	charHeight := (c.Height + 3) / 4 // Each braille char is 4 pixels tall

	var result strings.Builder
	result.Grow(charHeight * (charWidth + 1) * 10) // Extra space for ANSI codes

	currentColor := ColorReset

	for cy := range charHeight {
		for cx := range charWidth {
			// Calculate which pixels map to this braille character
			pattern := 0
			charColor := ColorWhite

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

			// Find the first set pixel to determine color for this character
			colorSet := false
			for _, dm := range dotMap {
				px := cx*2 + dm.dx
				py := cy*4 + dm.dy
				if c.Get(px, py) {
					pattern |= (1 << dm.bit)
					if !colorSet {
						charColor = c.colors[py][px]
						colorSet = true
					}
				}
			}

			// Only output color code if color changed
			if pattern != 0 && charColor != currentColor {
				fmt.Fprintf(&result, "\033[%dm", charColor)
				currentColor = charColor
			}

			result.WriteRune(rune(brailleBase + pattern))
		}
		if cy < charHeight-1 {
			result.WriteString("\r\n")
		}
	}

	// Reset color at the end
	if currentColor != ColorReset {
		result.WriteString("\033[0m")
	}

	return result.String()
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
