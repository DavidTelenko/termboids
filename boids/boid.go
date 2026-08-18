package boids

import "math/rand"

// Drawable interface for objects that can be drawn on a canvas
type Drawable interface {
	Set(x, y int)
	FillCircle(cx, cy, r int)
}

// Boid represents a single flocking entity
type Boid struct {
	Position Vector2D
	Velocity Vector2D
}

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
	RenderRadius     int
}

// DefaultConfig returns the default boid simulation parameters
// Tuned for fast-moving, tight clustering behavior
// Note: speeds are in pixels per second (frame-rate independent)
func DefaultConfig() Config {
	return Config{
		MaxSpeed:         50.0, // pixels per second - fast movement
		MaxForce:         80.0, // acceleration per second - responsive turning
		SeparationRadius: 5.0,  // Small radius = tight packing
		AlignmentRadius:  35.0, // Medium radius = group coordination
		CohesionRadius:   45.0, // Larger radius = strong clustering
		SeparationWeight: 1.8,  // Strong separation to prevent overlap
		AlignmentWeight:  1.2,  // Strong alignment for coordinated movement
		CohesionWeight:   1.0,  // Moderate cohesion for clustering
		RenderRadius:     1,    // Single dot
	}
}

// Simulation manages a flock of boids
type Simulation struct {
	Boids  []*Boid
	Config Config
	Width  int
	Height int
	grid   *SpatialGrid
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

	// Use cell size of the largest interaction radius
	cellSize := int(config.CohesionRadius)
	if config.AlignmentRadius > float64(cellSize) {
		cellSize = int(config.AlignmentRadius)
	}
	if config.SeparationRadius > float64(cellSize) {
		cellSize = int(config.SeparationRadius)
	}

	return &Simulation{
		Boids:  boids,
		Config: config,
		Width:  width,
		Height: height,
		grid:   NewSpatialGrid(width, height, cellSize),
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

		if separationCount > 0 {
			separation = separation.Scale(1.0 / float64(separationCount))
			if separation.Length() > 0 {
				separation = separation.Normalize().Scale(s.Config.MaxSpeed)
				separation = separation.Sub(boid.Velocity)
				separation = separation.Limit(s.Config.MaxForce)
				acceleration = acceleration.Add(separation.Scale(s.Config.SeparationWeight))
			}
		}

		if alignmentCount > 0 {
			alignment = alignment.Scale(1.0 / float64(alignmentCount))
			alignment = alignment.Normalize().Scale(s.Config.MaxSpeed)
			alignment = alignment.Sub(boid.Velocity)
			alignment = alignment.Limit(s.Config.MaxForce)
			acceleration = acceleration.Add(alignment.Scale(s.Config.AlignmentWeight))
		}

		if cohesionCount > 0 {
			cohesion = cohesion.Scale(1.0 / float64(cohesionCount))
			desired := cohesion.Sub(boid.Position)
			desired = desired.Normalize().Scale(s.Config.MaxSpeed)
			desired = desired.Sub(boid.Velocity)
			desired = desired.Limit(s.Config.MaxForce)
			acceleration = acceleration.Add(desired.Scale(s.Config.CohesionWeight))
		}

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

// Draw renders all boids to the given canvas
func (s *Simulation) Draw(canvas Drawable) {
	for _, boid := range s.Boids {
		x := int(boid.Position.X)
		y := int(boid.Position.Y)

		if s.Config.RenderRadius == 1 {
			// Single dot - just set the pixel
			canvas.Set(x, y)
		} else {
			// Use FillCircle for radius > 1
			// Adjust radius: user's "radius 2" becomes actual radius 1
			canvas.FillCircle(x, y, s.Config.RenderRadius-1)
		}
	}
}
