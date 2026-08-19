package boids

import (
	"math/rand"
	"termboids/braille"
	"termboids/config"
)

// Drawable interface for objects that can be drawn on a canvas
type Drawable interface {
	Set(x, y int)
	SetWithColor(x, y int, color braille.Color)
	FillCircle(cx, cy, r int)
	FillCircleWithColor(cx, cy, r int, color braille.Color)
	DrawLineWithColor(x0, y0, x1, y1 int, color braille.Color)
}

// Boid represents a single flocking entity
type Boid struct {
	Position Vector2D
	Velocity Vector2D
	Color    braille.Color // Color based on dominant force
}

// ColorMode determines how boids are colored
type ColorMode int

const (
	ColorModeNone     ColorMode = 0 // No coloring (white)
	ColorModeDistance ColorMode = 1 // Color by distance to flock center
)

// Config holds simulation parameters
type Config struct {
	config.BoidsConfig                        // Embed the shared config
	RepellantConfig    config.RepellantConfig // Repellant interaction config
	AttractorConfig    config.AttractorConfig // Attractor interaction config
	ColorMode          ColorMode
	ShowSpatialGrid    bool
	UseGPU             bool
}

// DefaultConfig returns the default boid simulation parameters
// Tuned for fast-moving, tight clustering behavior
// Note: speeds are in pixels per second (frame-rate independent)
func DefaultConfig() Config {
	return Config{
		BoidsConfig: config.BoidsConfig{
			NumBoids:         1000,
			MaxSpeed:         50.0, // pixels per second - fast movement
			MaxForce:         80.0, // acceleration per second - responsive turning
			SeparationRadius: 5.0,  // Small radius = tight packing
			AlignmentRadius:  35.0, // Medium radius = group coordination
			CohesionRadius:   45.0, // Larger radius = strong clustering
			SeparationWeight: 1.8,  // Strong separation to prevent overlap
			AlignmentWeight:  1.2,  // Strong alignment for coordinated movement
			CohesionWeight:   1.0,  // Moderate cohesion for clustering
			RandomWeight:     0.15, // Small random force to prevent stabilization
			RenderRadius:     1,    // Single dot
		},
		ColorMode: ColorModeDistance, // Default to distance-based coloring
	}
}

// Simulation manages a flock of boids
type Simulation struct {
	Boids            []*Boid
	Config           Config
	Width            int
	Height           int
	grid             *SpatialGrid
	smoothedMaxDist  float64 // Smoothed max distance to prevent flickering
	gpuCompute       *GPUCompute
	repellantPoint   *Vector2D // Active repellant point (nil if none)
	repellantExpires float64   // Time when repellant expires (seconds since start)
	attractorPoint   *Vector2D // Active attractor point (nil if none)
	attractorExpires float64   // Time when attractor expires (seconds since start)
	simulationTime   float64   // Total simulation time in seconds
}

// NewSimulation creates a new boid simulation
func NewSimulation(numBoids, width, height int, config Config) *Simulation {
	boids := make([]*Boid, numBoids)
	for i := range boids {
		boids[i] = &Boid{
			Position: Vector2D{
				X: rand.Float64() * float64(width),
				Y: rand.Float64() * float64(height),
			},
			Velocity: Vector2D{
				X: rand.Float64()*2 - 1,
				Y: rand.Float64()*2 - 1,
			}.Normalize().Scale(config.MaxSpeed * 0.5),
		}
	}

	// Use cell size of the smallest interaction radius
	cellSize := int(config.CohesionRadius)
	if config.AlignmentRadius < float64(cellSize) {
		cellSize = int(config.AlignmentRadius)
	}
	if config.SeparationRadius < float64(cellSize) {
		cellSize = int(config.SeparationRadius)
	}

	sim := &Simulation{
		Boids:           boids,
		Config:          config,
		Width:           width,
		Height:          height,
		grid:            NewSpatialGrid(width, height, cellSize),
		smoothedMaxDist: 100.0, // Initial value
	}

	// Initialize GPU compute if enabled
	if config.UseGPU {
		gpuCompute, err := NewGPUCompute(numBoids)
		if err != nil {
			// Silently fallback to CPU - don't interfere with terminal rendering
		} else {
			sim.gpuCompute = gpuCompute
		}
	}

	return sim
}

// Update advances the simulation by deltaTime seconds
func (s *Simulation) Update(deltaTime float64) {
	// Update simulation time
	s.simulationTime += deltaTime

	// Check if repellant has expired
	if s.repellantPoint != nil && s.simulationTime >= s.repellantExpires {
		s.repellantPoint = nil
	}

	// Check if attractor has expired
	if s.attractorPoint != nil && s.simulationTime >= s.attractorExpires {
		s.attractorPoint = nil
	}

	// Use GPU compute if available
	if s.gpuCompute != nil {
		s.updateGPU(deltaTime)
		// Apply color modes for GPU path (GPU shader doesn't compute colors)
		s.applyColorMode()
	} else {
		s.updateCPU(deltaTime)
	}

	// Apply distance-based coloring if in distance mode
	if s.Config.ColorMode == ColorModeDistance {
	}
}

// updateGPU uses GPU compute shader for boid updates
func (s *Simulation) updateGPU(deltaTime float64) {
	// Upload boids to GPU
	if err := s.gpuCompute.UploadBoids(s.Boids); err != nil {
		return
	}

	// Upload config with attractor/repellant data
	if err := s.gpuCompute.UploadConfig(s.Config, s.Width, s.Height, deltaTime, s.repellantPoint, s.attractorPoint); err != nil {
		return
	}

	// Run compute shader
	if err := s.gpuCompute.Compute(); err != nil {
		return
	}

	// Download results
	if err := s.gpuCompute.DownloadBoids(s.Boids); err != nil {
		return
	}
}

// updateCPU uses CPU-based boid updates (original implementation)
func (s *Simulation) updateCPU(deltaTime float64) {
	// Rebuild spatial grid
	s.grid.Clear()
	for _, boid := range s.Boids {
		s.grid.Insert(boid)
	}

	// Pre-calculate squared radii (avoid recalculating in inner loop)
	alignRadiusSquared := s.Config.AlignmentRadius * s.Config.AlignmentRadius
	cohRadiusSquared := s.Config.CohesionRadius * s.Config.CohesionRadius

	// Get maximum query radius once
	maxRadius := s.Config.CohesionRadius
	if s.Config.AlignmentRadius > maxRadius {
		maxRadius = s.Config.AlignmentRadius
	}
	if s.Config.SeparationRadius > maxRadius {
		maxRadius = s.Config.SeparationRadius
	}

	// Calculate new velocities for all boids
	newVelocities := make([]Vector2D, len(s.Boids))

	for i, boid := range s.Boids {
		separation := Vector2D{0, 0}
		alignment := Vector2D{0, 0}
		cohesion := Vector2D{0, 0}
		separationCount := 0
		alignmentCount := 0
		cohesionCount := 0

		// Query nearby boids using spatial grid
		nearby := s.grid.QueryRadius(boid.Position, maxRadius)

		for _, other := range nearby {
			if boid == other {
				continue
			}

			distSquared := boid.Position.DistanceSquared(other.Position)
			dist := boid.Position.Distance(other.Position)

			// Separation: steer away from nearby boids
			if dist < s.Config.SeparationRadius && dist > 0 {
				diff := boid.Position.Sub(other.Position)
				// Weight by inverse distance (closer = stronger push)
				diff = diff.Normalize().Scale(1.0 / dist)
				separation = separation.Add(diff)
				separationCount++
			}

			// Alignment: steer towards average heading of nearby boids
			if distSquared < alignRadiusSquared {
				alignment = alignment.Add(other.Velocity)
				alignmentCount++
			}

			// Cohesion: steer towards average position of nearby boids
			if distSquared < cohRadiusSquared {
				cohesion = cohesion.Add(other.Position)
				cohesionCount++
			}
		}

		// Calculate steering forces
		acceleration := Vector2D{0, 0}

		// Add repellant force if active (this should dominate other forces)
		if s.repellantPoint != nil {
			dist := boid.Position.Distance(*s.repellantPoint)

			if dist < s.Config.RepellantConfig.Radius && dist > 0 {
				// Steer away from repellant point
				diff := boid.Position.Sub(*s.repellantPoint)
				// Stronger force when closer
				strength := (s.Config.RepellantConfig.Radius - dist) / s.Config.RepellantConfig.Radius
				// Apply configured repellant strength multiplier
				diff = diff.Normalize().Scale(s.Config.MaxForce * strength * s.Config.RepellantConfig.Strength)
				acceleration = acceleration.Add(diff)
			}
		}

		// Add attractor force if active (this should dominate other forces)
		if s.attractorPoint != nil {
			dist := boid.Position.Distance(*s.attractorPoint)

			if dist < s.Config.AttractorConfig.Radius && dist > 0 {
				// Steer towards attractor point
				diff := s.attractorPoint.Sub(boid.Position)
				// Stronger force when closer
				strength := (s.Config.AttractorConfig.Radius - dist) / s.Config.AttractorConfig.Radius
				// Apply configured attractor strength multiplier
				diff = diff.Normalize().Scale(s.Config.MaxForce * strength * s.Config.AttractorConfig.Strength)
				acceleration = acceleration.Add(diff)
			}
		}

		// Track force magnitudes to determine dominant behavior
		if separationCount > 0 {
			separation = separation.Scale(1.0 / float64(separationCount))
			if separation.Length() > 0 {
				separation = separation.Normalize().Scale(s.Config.MaxSpeed)
				separation = separation.Sub(boid.Velocity)
				separation = separation.Limit(s.Config.MaxForce)
				sepForce := separation.Scale(s.Config.SeparationWeight)
				acceleration = acceleration.Add(sepForce)
			}
		}

		if alignmentCount > 0 {
			alignment = alignment.Scale(1.0 / float64(alignmentCount))
			alignment = alignment.Normalize().Scale(s.Config.MaxSpeed)
			alignment = alignment.Sub(boid.Velocity)
			alignment = alignment.Limit(s.Config.MaxForce)
			alignForce := alignment.Scale(s.Config.AlignmentWeight)
			acceleration = acceleration.Add(alignForce)
		}

		if cohesionCount > 0 {
			cohesion = cohesion.Scale(1.0 / float64(cohesionCount))
			desired := cohesion.Sub(boid.Position)
			desired = desired.Normalize().Scale(s.Config.MaxSpeed)
			desired = desired.Sub(boid.Velocity)
			desired = desired.Limit(s.Config.MaxForce)
			cohForce := desired.Scale(s.Config.CohesionWeight)
			acceleration = acceleration.Add(cohForce)
		}

		// Add random force to prevent stabilization
		// Apply random impulses with weighted probability
		if s.Config.RandomWeight > 0 && rand.Float64() < 0.1 { // 10% chance per frame
			randomForce := Vector2D{
				X: rand.Float64()*2 - 1,
				Y: rand.Float64()*2 - 1,
			}.Normalize().Scale(s.Config.MaxForce * s.Config.RandomWeight)
			acceleration = acceleration.Add(randomForce)
		}

		// Determine color based on selected mode
		switch s.Config.ColorMode {
		case ColorModeNone:
			boid.Color = braille.ColorWhite
		}
		// Distance-based coloring is done after all positions are updated

		// Update velocity (scaled by deltaTime for frame-rate independence)
		acceleration = acceleration.Scale(deltaTime)
		newVelocities[i] = boid.Velocity.Add(acceleration).Limit(s.Config.MaxSpeed)
	}

	// Apply new velocities and update positions
	for i, boid := range s.Boids {
		boid.Velocity = newVelocities[i]
		// Scale position update by deltaTime
		boid.Position = boid.Position.Add(boid.Velocity.Scale(deltaTime))

		// Wrap around edges - appear at opposite side
		if boid.Position.X < 0 {
			boid.Position.X += float64(s.Width)
		} else if boid.Position.X >= float64(s.Width) {
			boid.Position.X -= float64(s.Width)
		}

		if boid.Position.Y < 0 {
			boid.Position.Y += float64(s.Height)
		} else if boid.Position.Y >= float64(s.Height) {
			boid.Position.Y -= float64(s.Height)
		}
	}
}

// applyColorMode applies the current color mode to all boids
// This is used for GPU path where colors aren't computed in the shader
func (s *Simulation) applyColorMode() {
	switch s.Config.ColorMode {
	case ColorModeNone:
		// Set all boids to white
		for _, boid := range s.Boids {
			boid.Color = braille.ColorWhite
		}
	case ColorModeDistance:
		s.colorByDistance()
	}
}

// colorByDistance assigns colors based on distance to flock center
// Uses smooth interpolation for fluid color transitions
func (s *Simulation) colorByDistance() {
	// Calculate center of mass of entire flock
	centerX := 0.0
	centerY := 0.0
	for _, boid := range s.Boids {
		centerX += boid.Position.X
		centerY += boid.Position.Y
	}
	centerX /= float64(len(s.Boids))
	centerY /= float64(len(s.Boids))
	center := Vector2D{X: centerX, Y: centerY}

	// Find max distance for normalization
	maxDist := 0.0
	for _, boid := range s.Boids {
		dist := boid.Position.Distance(center)
		if dist > maxDist {
			maxDist = dist
		}
	}

	// Smooth the max distance heavily to prevent flickering
	// Use exponential moving average with strong smoothing
	smoothingFactor := 0.05 // Lower = more smoothing
	s.smoothedMaxDist = s.smoothedMaxDist*(1-smoothingFactor) + maxDist*smoothingFactor

	// Prevent division by zero
	if s.smoothedMaxDist < 1.0 {
		s.smoothedMaxDist = 1.0
	}

	// Assign colors based on normalized distance with smooth, fluid transitions
	for _, boid := range s.Boids {
		dist := boid.Position.Distance(center)
		normalizedDist := dist / s.smoothedMaxDist // 0.0 = center, 1.0 = outskirts

		// Clamp to [0, 1] range
		if normalizedDist > 1.0 {
			normalizedDist = 1.0
		}

		// Use smooth probabilistic color assignment for fluid transitions
		// Create smooth gradient: Red (center) -> Yellow -> Green -> White (outskirts)
		// Use velocity and fine-grained position hash to reduce visible patterns

		// Calculate a stable hash with finer granularity and better distribution
		// Mix position with velocity for more randomness
		hashVal := (int(boid.Position.X*13.7) + int(boid.Position.Y*17.3)*2531 +
			int(boid.Velocity.X*100)*97 + int(boid.Velocity.Y*100)*127) % 1000
		threshold := float64(hashVal) / 1000.0

		// Define color transition points with smooth probabilistic blending
		if normalizedDist < 0.2 {
			// Very center - mostly red with gradual yellow blend
			redProbability := 1.0 - (normalizedDist/0.2)*0.3
			if threshold < redProbability {
				boid.Color = braille.ColorRed
			} else {
				boid.Color = braille.ColorYellow
			}
		} else if normalizedDist < 0.5 {
			// Red to Yellow transition zone
			redProbability := (0.5 - normalizedDist) / 0.3 // 1.0 at 0.2, 0.0 at 0.5
			if threshold < redProbability {
				boid.Color = braille.ColorRed
			} else {
				boid.Color = braille.ColorYellow
			}
		} else if normalizedDist < 0.75 {
			// Yellow to Green transition zone
			yellowProbability := (0.75 - normalizedDist) / 0.25 // 1.0 at 0.5, 0.0 at 0.75
			if threshold < yellowProbability {
				boid.Color = braille.ColorYellow
			} else {
				boid.Color = braille.ColorGreen
			}
		} else {
			// Green to White transition zone (outskirts)
			greenProbability := (1.0 - normalizedDist) / 0.25 // 1.0 at 0.75, 0.0 at 1.0
			if threshold < greenProbability {
				boid.Color = braille.ColorGreen
			} else {
				boid.Color = braille.ColorWhite
			}
		}
	}
}

// Draw renders all boids to the given canvas
func (s *Simulation) Draw(canvas Drawable) {
	// Draw spatial grid if debug mode is enabled
	if s.Config.ShowSpatialGrid {
		s.drawSpatialGrid(canvas)
	}

	for _, boid := range s.Boids {
		x := int(boid.Position.X)
		y := int(boid.Position.Y)

		if s.Config.RenderRadius == 1 {
			// Single dot - just set the pixel with color
			canvas.SetWithColor(x, y, boid.Color)
		} else {
			// Use FillCircle for radius > 1
			// Adjust radius: user's "radius 2" becomes actual radius 1
			canvas.FillCircleWithColor(x, y, s.Config.RenderRadius-1, boid.Color)
		}
	}
}

// drawSpatialGrid renders the spatial subdivision grid for debugging
func (s *Simulation) drawSpatialGrid(canvas Drawable) {
	cellSize := s.grid.cellSize
	gridColor := braille.ColorCyan

	// Draw vertical lines
	for x := cellSize; x < s.Width; x += cellSize {
		canvas.DrawLineWithColor(x, 0, x, s.Height-1, gridColor)
	}

	// Draw horizontal lines
	for y := cellSize; y < s.Height; y += cellSize {
		canvas.DrawLineWithColor(0, y, s.Width-1, y, gridColor)
	}
}

// Release frees simulation resources (especially GPU resources)
func (s *Simulation) Release() {
	if s.gpuCompute != nil {
		s.gpuCompute.Release()
		s.gpuCompute = nil
	}
}

// SetBounds updates the simulation bounds
func (s *Simulation) SetBounds(width, height int) {
	s.Width = width
	s.Height = height
	// Update spatial grid with new bounds (keep the same cell size)
	cellSize := s.grid.cellSize
	s.grid = NewSpatialGrid(width, height, cellSize)
}

// IsUsingGPU returns true if GPU compute is active
func (s *Simulation) IsUsingGPU() bool {
	return s.gpuCompute != nil
}

// SetRepellant sets a repellant point that boids will avoid for the specified duration
func (s *Simulation) SetRepellant(x, y float64, duration float64) {
	s.repellantPoint = &Vector2D{X: x, Y: y}
	s.repellantExpires = s.simulationTime + duration
}

// GetRepellant returns the current repellant point if active, nil otherwise
func (s *Simulation) GetRepellant() *Vector2D {
	return s.repellantPoint
}

// SetAttractor sets an attractor point that boids will move towards for the specified duration
func (s *Simulation) SetAttractor(x, y float64, duration float64) {
	s.attractorPoint = &Vector2D{X: x, Y: y}
	s.attractorExpires = s.simulationTime + duration
}

// GetAttractor returns the current attractor point if active, nil otherwise
func (s *Simulation) GetAttractor() *Vector2D {
	return s.attractorPoint
}
