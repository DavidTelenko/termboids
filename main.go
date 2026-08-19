package main

import (
	"bytes"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"termboids/boids"
	"termboids/braille"
	"termboids/config"
	"termboids/input"
	"time"

	"golang.org/x/term"
)

func main() {
	// Load configuration
	cfg, err := config.LoadOrDefault()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Get terminal size
	termWidth, termHeight, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		// Fallback to default size if we can't get terminal size
		termWidth = 80
		termHeight = 24
	}

	// Track whether bottom line is visible
	showHelp := true

	// Calculate pixel dimensions
	// Each braille character is 2 pixels wide and 4 pixels tall
	// Reserve 1 line for status text at the bottom (when visible)
	width := termWidth * 2
	var height int
	if showHelp {
		height = (termHeight - 1) * 4
	} else {
		height = termHeight * 4
	}

	// Create boid simulation from config
	boidConfig := boids.Config{
		MaxSpeed:         cfg.Boids.MaxSpeed,
		MaxForce:         cfg.Boids.MaxForce,
		SeparationRadius: cfg.Boids.SeparationRadius,
		AlignmentRadius:  cfg.Boids.AlignmentRadius,
		CohesionRadius:   cfg.Boids.CohesionRadius,
		SeparationWeight: cfg.Boids.SeparationWeight,
		AlignmentWeight:  cfg.Boids.AlignmentWeight,
		CohesionWeight:   cfg.Boids.CohesionWeight,
		RandomWeight:     cfg.Boids.RandomWeight,
		RenderRadius:     cfg.Boids.RenderRadius,
		ColorMode:        boids.ColorModeDistance, // Start with distance-based coloring
		UseGPU:           cfg.Rendering.UseGPU,
	}
	simulation := boids.NewSimulation(cfg.Boids.NumBoids, width, height, boidConfig)
	defer simulation.Release() // Clean up GPU resources on exit

	canvas := braille.NewCanvas(width, height)

	// Initialize input handler
	inputHandler, err := input.NewHandler()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing input handler: %v\n", err)
		os.Exit(1)
	}
	defer inputHandler.Close()

	// Hide cursor and clear screen once at startup
	fmt.Print("\033[?25l\033[2J\033[H")

	// Setup signal handler to restore cursor on exit
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		simulation.Release()
		inputHandler.Close()
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

	// Target frame time for 60 FPS
	targetFrameTime := time.Second / time.Duration(cfg.Rendering.FPS)

	// Animation loop
	for {
		frameStart := time.Now()

		// Poll for input
		key := inputHandler.Poll()
		if key != 0 {
			keyStr := strings.ToLower(string(key))

			// Check for quit key
			if keyStr == strings.ToLower(cfg.KeyBindings.Quit) {
				simulation.Release()
				inputHandler.Close()
				fmt.Print("\033[?25h") // Show cursor
				os.Exit(0)
			}

			// Check for color mode cycle key
			if keyStr == strings.ToLower(cfg.KeyBindings.CycleColorMode) {
				// Cycle through color modes
				switch simulation.Config.ColorMode {
				case boids.ColorModeNone:
					simulation.Config.ColorMode = boids.ColorModeForce
				case boids.ColorModeForce:
					simulation.Config.ColorMode = boids.ColorModeDistance
				case boids.ColorModeDistance:
					simulation.Config.ColorMode = boids.ColorModeNone
				}
			}

			// Check for debug grid toggle key
			if keyStr == strings.ToLower(cfg.KeyBindings.DebugGrid) {
				simulation.Config.ShowSpatialGrid = !simulation.Config.ShowSpatialGrid
			}

			// Check for toggle bottom line key
			if keyStr == strings.ToLower(cfg.KeyBindings.ToggleHelp) {
				showHelp = !showHelp
				// Recalculate height and resize canvas
				if showHelp {
					height = (termHeight - 1) * 4
				} else {
					height = termHeight * 4
				}
				simulation.SetBounds(width, height)
				canvas = braille.NewCanvas(width, height)
			}
		}

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

		// Display bottom line if enabled
		if showHelp {
			// Position cursor at last line and write status
			fmt.Fprintf(&buffer, "\033[%d;1H", termHeight) // Move to last line, first column
			fmt.Fprintf(&buffer, "\033[K")                 // Clear line

			// Display status based on color mode
			switch simulation.Config.ColorMode {
			case boids.ColorModeDistance:
				fmt.Fprintf(&buffer, "FPS: %.1f | Boids: %d | Mode: Distance | \033[31mCenter\033[0m \033[33mClose\033[0m \033[32mFar\033[0m \033[37mOutskirts\033[0m",
					fps, cfg.Boids.NumBoids)
			case boids.ColorModeForce:
				fmt.Fprintf(&buffer, "FPS: %.1f | Boids: %d | Mode: Force | \033[31mSeparation\033[0m \033[32mAlignment\033[0m \033[34mCohesion\033[0m \033[37mBalanced\033[0m",
					fps, cfg.Boids.NumBoids)
			default:
				fmt.Fprintf(&buffer, "FPS: %.1f | Boids: %d | Mode: None",
					fps, cfg.Boids.NumBoids)
			}

			// Add GPU indicator
			if simulation.Config.UseGPU && simulation.IsUsingGPU() {
				fmt.Fprintf(&buffer, " | \033[35mGPU\033[0m")
			}

			// Add debug grid indicator if enabled
			if simulation.Config.ShowSpatialGrid {
				fmt.Fprintf(&buffer, " | \033[36mDEBUG: Grid ON\033[0m")
			}
		}

		// Write entire frame at once
		fmt.Print(buffer.String())

		// Frame rate limiting to prevent flickering
		frameTime := time.Since(frameStart)
		if frameTime < targetFrameTime {
			time.Sleep(targetFrameTime - frameTime)
		}
	}
}
