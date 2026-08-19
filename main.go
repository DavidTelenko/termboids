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

	// Track whether config debug window is visible
	showConfigDebug := false

	// Track whether simulation is paused
	isPaused := false

	// Track current preset name
	currentPreset := "Default"

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
		BoidsConfig:     cfg.Boids,
		RepellantConfig: cfg.System.Repellant,
		ColorMode:       boids.ColorModeDistance, // Start with distance-based coloring
		UseGPU:          cfg.System.Rendering.UseGPU,
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

	// Switch to alternate screen buffer, hide cursor, enable mouse tracking and clear screen once at startup
	// \033[?1049h - alternate screen buffer
	// \033[?25l - hide cursor
	// \033[?1000h - enable mouse button press tracking
	// \033[?1002h - enable mouse button press and release tracking
	// \033[?1006h - enable SGR extended mouse mode (better coordinate handling)
	// \033[2J - clear screen
	// \033[H - move cursor to home
	fmt.Print("\033[?1049h\033[?25l\033[?1000h\033[?1002h\033[?1006h\033[2J\033[H")

	// Setup signal handler to restore terminal on exit
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		simulation.Release()
		inputHandler.Close()
		// Disable mouse tracking, show cursor and restore normal screen buffer
		fmt.Print("\033[?1006l\033[?1002l\033[?1000l\033[?25h\033[?1049l")
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
	targetFrameTime := time.Second / time.Duration(cfg.System.Rendering.FPS)

	// Preload all preset configs at startup
	presetConfigs := make(map[string]config.BoidsConfig)
	presetNameMap := make(map[string]string)
	for _, preset := range cfg.System.Presets {
		presetConfig, err := config.LoadBoidsConfig(preset.Path)
		if err != nil {
			// Log warning but continue - preset won't be available
			fmt.Fprintf(os.Stderr, "Warning: Failed to load preset '%s' from %s: %v\n",
				preset.Name, preset.Path, err)
			continue
		}
		presetConfigs[preset.Key] = presetConfig
		presetNameMap[preset.Key] = preset.Name
	}

	// Animation loop
	for {
		frameStart := time.Now()

		// Poll for input
		key := inputHandler.Poll()
		if key != 0 {
			keyStr := strings.ToLower(string(key))

			// Check for quit key
			if keyStr == strings.ToLower(cfg.System.KeyBindings.Quit) {
				simulation.Release()
				inputHandler.Close()
				// Disable mouse tracking, show cursor and restore normal screen buffer
				fmt.Print("\033[?1006l\033[?1002l\033[?1000l\033[?25h\033[?1049l")
				os.Exit(0)
			}

			// Check for color mode cycle key
			if keyStr == strings.ToLower(cfg.System.KeyBindings.CycleColorMode) {
				// Cycle through color modes
				switch simulation.Config.ColorMode {
				case boids.ColorModeNone:
					simulation.Config.ColorMode = boids.ColorModeDistance
				case boids.ColorModeDistance:
					simulation.Config.ColorMode = boids.ColorModeNone
				}
			}

			// Check for debug grid toggle key
			if keyStr == strings.ToLower(cfg.System.KeyBindings.DebugGrid) {
				simulation.Config.ShowSpatialGrid = !simulation.Config.ShowSpatialGrid
			}

			// Check for toggle bottom line key
			if keyStr == strings.ToLower(cfg.System.KeyBindings.ToggleHelp) {
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

			// Check for show config debug key
			if keyStr == strings.ToLower(cfg.System.KeyBindings.ShowConfig) {
				showConfigDebug = !showConfigDebug
			}

			// Check for pause key
			if keyStr == strings.ToLower(cfg.System.KeyBindings.Pause) {
				isPaused = !isPaused
			}

			// Check for preset loading keys (1-0)
			if presetConfig, ok := presetConfigs[keyStr]; ok {
				// Update current preset name
				currentPreset = presetNameMap[keyStr]

				// Update current config
				cfg.Boids = presetConfig

				// Update simulation config with new boids parameters
				simulation.Config.BoidsConfig = presetConfig

				// Recreate boids with new count if it changed
				if presetConfig.NumBoids != len(simulation.Boids) {
					simulation.Release()
					simulation = boids.NewSimulation(presetConfig.NumBoids, width, height, simulation.Config)
				}
			}
		}
		
		// Poll for mouse events
		mouseEvent := inputHandler.PollMouse()
		if mouseEvent != nil {
			// Convert terminal coordinates to pixel coordinates
			// Terminal coordinates are character-based, need to convert to pixel space
			// Each character is 2 pixels wide and 4 pixels tall (braille characters)
			pixelX := float64(mouseEvent.X * 2)
			pixelY := float64(mouseEvent.Y * 4)
			
			// Set repellant point with configured duration
			simulation.SetRepellant(pixelX, pixelY, cfg.System.Repellant.Duration)
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

		// Update simulation with delta time (only if not paused)
		if !isPaused {
			simulation.Update(deltaTime)
		}

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

			// Display status with current preset
			fmt.Fprintf(&buffer, "FPS: %.1f | Boids: %d | Preset: \033[33m%s\033[0m",
				fps, cfg.Boids.NumBoids, currentPreset)

			// Add pause indicator
			if isPaused {
				fmt.Fprintf(&buffer, " | \033[31m⏸ PAUSED\033[0m")
			}

			// Add GPU indicator
			if simulation.Config.UseGPU && simulation.IsUsingGPU() {
				fmt.Fprintf(&buffer, " | \033[35mGPU\033[0m")
			}

			// Add debug grid indicator if enabled
			if simulation.Config.ShowSpatialGrid {
				fmt.Fprintf(&buffer, " | \033[36mDEBUG: Grid ON\033[0m")
			}
			
			// Add repellant indicator if active
			if simulation.GetRepellant() != nil {
				fmt.Fprintf(&buffer, " | \033[31m🔴 REPELLANT\033[0m")
			}
		}

		// Display floating config debug window if enabled
		if showConfigDebug {
			// Position at bottom left corner
			startX := 2
			startY := max(termHeight-15, 1)

			// Draw window border and content using rounded single-line box drawing
			lines := []string{
				"╭──────────────────────────────────────────────╮",
				"│        BOID CONFIGURATION DEBUG              │",
				"├──────────────────────────────────────────────┤",
				fmt.Sprintf("│ Num Boids:         %-25d │", cfg.Boids.NumBoids),
				fmt.Sprintf("│ Max Speed:         %-25.2f │", cfg.Boids.MaxSpeed),
				fmt.Sprintf("│ Max Force:         %-25.2f │", cfg.Boids.MaxForce),
				fmt.Sprintf("│ Separation Radius: %-25.2f │", cfg.Boids.SeparationRadius),
				fmt.Sprintf("│ Alignment Radius:  %-25.2f │", cfg.Boids.AlignmentRadius),
				fmt.Sprintf("│ Cohesion Radius:   %-25.2f │", cfg.Boids.CohesionRadius),
				fmt.Sprintf("│ Separation Weight: %-25.2f │", cfg.Boids.SeparationWeight),
				fmt.Sprintf("│ Alignment Weight:  %-25.2f │", cfg.Boids.AlignmentWeight),
				fmt.Sprintf("│ Cohesion Weight:   %-25.2f │", cfg.Boids.CohesionWeight),
				fmt.Sprintf("│ Random Weight:     %-25.2f │", cfg.Boids.RandomWeight),
				"╰──────────────────────────────────────────────╯",
			}

			for i, line := range lines {
				fmt.Fprintf(&buffer, "\033[%d;%dH%s", startY+i, startX, line)
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
