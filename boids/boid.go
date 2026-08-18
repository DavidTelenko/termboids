package boids

import (
	"math/rand"
	"termboids/braille"
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
	ColorModeForce    ColorMode = 1 // Color by dominant behavioral force
	ColorModeDistance ColorMode = 2 // Color by distance to flock center
)

// Config holds simulation parameters
type Config struct {
	MaxSpeed         float64
	MaxForce         float64
	SeparationRadius float64
	AlignmentRadius  float64
	CohesionRadius   float64
	SeparationWeight float64
	AlignmentWeight  float64
	CohesionWeight   float64
	RandomWeight     float64
	RenderRadius     int
	ColorMode        ColorMode
	ShowSpatialGrid  bool
}

// DefaultConfig returns the default boid simulation parameters
// Tuned for fast-moving, tight clustering behavior
// Note: speeds are in pixels per second (frame-rate independent)
func DefaultConfig() Config {
	return Config{
		MaxSpeed:         50.0,              // pixels per second - fast movement
		MaxForce:         80.0,              // acceleration per second - responsive turning
		SeparationRadius: 5.0,               // Small radius = tight packing
		AlignmentRadius:  35.0,              // Medium radius = group coordination
		CohesionRadius:   45.0,              // Larger radius = strong clustering
		SeparationWeight: 1.8,               // Strong separation to prevent overlap
		AlignmentWeight:  1.2,               // Strong alignment for coordinated movement
		CohesionWeight:   1.0,               // Moderate cohesion for clustering
		RandomWeight:     0.15,              // Small random force to prevent stabilization
		RenderRadius:     1,                 // Single dot
		ColorMode:        ColorModeDistance, // Default to distance-based coloring
	}
}

// Simulation manages a flock of boids
type Simulation struct {
	Boids           []*Boid
	Config          Config
	Width           int
	Height          int
	grid            *SpatialGrid
	smoothedMaxDist float64 // Smoothed max distance to prevent flickering
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

	return &Simulation{
		Boids:           boids,
		Config:          config,
		Width:           width,
		Height:          height,
		grid:            NewSpatialGrid(width, height, cellSize),
		smoothedMaxDist: 100.0, // Initial value
	}
}

// Update advances the simulation by deltaTime seconds
func (s *Simulation) Update(deltaTime float64) {
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

		// Track force magnitudes to determine dominant behavior
		var sepMagnitude, alignMagnitude, cohMagnitude float64

		if separationCount > 0 {
			separation = separation.Scale(1.0 / float64(separationCount))
			if separation.Length() > 0 {
				separation = separation.Normalize().Scale(s.Config.MaxSpeed)
				separation = separation.Sub(boid.Velocity)
				separation = separation.Limit(s.Config.MaxForce)
				sepForce := separation.Scale(s.Config.SeparationWeight)
				sepMagnitude = sepForce.Length()
				acceleration = acceleration.Add(sepForce)
			}
		}

		if alignmentCount > 0 {
			alignment = alignment.Scale(1.0 / float64(alignmentCount))
			alignment = alignment.Normalize().Scale(s.Config.MaxSpeed)
			alignment = alignment.Sub(boid.Velocity)
			alignment = alignment.Limit(s.Config.MaxForce)
			alignForce := alignment.Scale(s.Config.AlignmentWeight)
			alignMagnitude = alignForce.Length()
			acceleration = acceleration.Add(alignForce)
		}

		if cohesionCount > 0 {
			cohesion = cohesion.Scale(1.0 / float64(cohesionCount))
			desired := cohesion.Sub(boid.Position)
			desired = desired.Normalize().Scale(s.Config.MaxSpeed)
			desired = desired.Sub(boid.Velocity)
			desired = desired.Limit(s.Config.MaxForce)
			cohForce := desired.Scale(s.Config.CohesionWeight)
			cohMagnitude = cohForce.Length()
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
		case ColorModeForce:
			s.colorByForce(boid, sepMagnitude, alignMagnitude, cohMagnitude)
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

	// Apply distance-based coloring if in distance mode
	if s.Config.ColorMode == ColorModeDistance {
		s.colorByDistance()
	}
}

// colorByForce assigns colors based on dominant behavioral force
func (s *Simulation) colorByForce(boid *Boid, sepMagnitude, alignMagnitude, cohMagnitude float64) {
	// Red = Separation, Green = Alignment, Blue = Cohesion, White = Balanced
	totalMagnitude := sepMagnitude + alignMagnitude + cohMagnitude
	if totalMagnitude < 0.1 {
		// Very weak forces - balanced/neutral
		boid.Color = braille.ColorWhite
	} else if sepMagnitude > alignMagnitude && sepMagnitude > cohMagnitude {
		// Separation dominant - avoiding crowding
		boid.Color = braille.ColorRed
	} else if alignMagnitude > cohMagnitude {
		// Alignment dominant - matching neighbors
		boid.Color = braille.ColorGreen
	} else {
		// Cohesion dominant - seeking group
		boid.Color = braille.ColorBlue
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
