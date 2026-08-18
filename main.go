package main

import (
	"bytes"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"termboids/boids"
	"termboids/braille"
	"time"

	"golang.org/x/term"
)

const numBoids = 1000

func main() {
	// Get terminal size
	termWidth, termHeight, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		// Fallback to default size if we can't get terminal size
		termWidth = 80
		termHeight = 24
	}

	// Calculate pixel dimensions
	// Each braille character is 2 pixels wide and 4 pixels tall
	// Reserve 1 line for status text at the bottom
	width := termWidth * 2
	height := (termHeight - 1) * 4

	// Create boid simulation with default config
	config := boids.Config{
		MaxSpeed:         100.0,
		MaxForce:         80.0,
		SeparationRadius: 5.0,
		AlignmentRadius:  45.0,
		CohesionRadius:   45.0,
		SeparationWeight: 1.8,
		AlignmentWeight:  1.2,
		CohesionWeight:   1.0,
		RenderRadius:     1,
		ColorMode:        boids.ColorModeDistance, // Use distance-based coloring
	}
	simulation := boids.NewSimulation(numBoids, width, height, config)

	canvas := braille.NewCanvas(width, height)

	// Hide cursor and clear screen once at startup
	fmt.Print("\033[?25l\033[2J\033[H")

	// Setup signal handler to restore cursor on exit
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Print("\033[?25h") // Show cursor
		os.Exit(0)
	}()

	// Buffer for building the frame
	var buffer bytes.Buffer

	// FPS tracking
	var frameCount int
	var fps float64
	fpsUpdateInterval := time.Second
	lastFPSUpdate := time.Now()

	// Delta time tracking
	lastFrameTime := time.Now()

	// Animation loop
	for {
		frameStart := time.Now()

		// Calculate delta time in seconds
		deltaTime := frameStart.Sub(lastFrameTime).Seconds()
		lastFrameTime = frameStart

		// Cap delta time to prevent huge jumps (e.g., when debugging)
		if deltaTime > 0.1 {
			deltaTime = 0.1
		}

		buffer.Reset()
		canvas.Clear()

		// Update simulation with delta time
		simulation.Update(deltaTime)

		// Draw boids using simulation's Draw method
		simulation.Draw(canvas)

		// Update FPS counter
		frameCount++
		if time.Since(lastFPSUpdate) >= fpsUpdateInterval {
			fps = float64(frameCount) / time.Since(lastFPSUpdate).Seconds()
			frameCount = 0
			lastFPSUpdate = time.Now()
		}

		// Render to buffer
		buffer.WriteString("\033[H") // Move cursor to home position
		buffer.WriteString(canvas.Render())

		// Position cursor at last line and write status
		fmt.Fprintf(&buffer, "\033[%d;1H", termHeight) // Move to last line, first column
		fmt.Fprintf(&buffer, "\033[K")                 // Clear line

		// Display status based on color mode
		if simulation.Config.ColorMode == boids.ColorModeDistance {
			fmt.Fprintf(&buffer, "FPS: %.1f | Boids: %d | Mode: Distance | \033[31mCenter\033[0m \033[33mClose\033[0m \033[32mFar\033[0m \033[37mOutskirts\033[0m | Ctrl+C to exit", fps, numBoids)
		} else {
			fmt.Fprintf(&buffer, "FPS: %.1f | Boids: %d | Mode: Force | \033[31mSeparation\033[0m \033[32mAlignment\033[0m \033[34mCohesion\033[0m \033[37mBalanced\033[0m | Ctrl+C to exit", fps, numBoids)
		}

		// Write entire frame at once
		fmt.Print(buffer.String())
	}
}
