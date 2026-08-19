package boids

import (
	"termboids/config"
	"testing"
)

func TestGPUCompute(t *testing.T) {
	// Create small simulation
	numBoids := 100
	width := 800
	height := 600

	config := Config{
		BoidsConfig: config.BoidsConfig{
			NumBoids:         numBoids,
			MaxSpeed:         50.0,
			MaxForce:         80.0,
			SeparationRadius: 5.0,
			AlignmentRadius:  35.0,
			CohesionRadius:   45.0,
			SeparationWeight: 1.8,
			AlignmentWeight:  1.2,
			CohesionWeight:   1.0,
			RandomWeight:     0.15,
			RenderRadius:     1,
		},
		ColorMode: ColorModeDistance,
		UseGPU:    true,
	}

	// Create simulation with GPU
	simGPU := NewSimulation(numBoids, width, height, config)
	if simGPU.gpuCompute == nil {
		t.Skip("GPU not available, skipping test")
	}
	defer simGPU.Release()

	// Save initial positions
	initialPositions := make([]Vector2D, numBoids)
	for i, boid := range simGPU.Boids {
		initialPositions[i] = boid.Position
	}

	// Update simulation
	deltaTime := 0.016 // ~60 FPS
	simGPU.Update(deltaTime)

	// Verify boids have moved
	movedCount := 0
	for i, boid := range simGPU.Boids {
		if boid.Position.X != initialPositions[i].X || boid.Position.Y != initialPositions[i].Y {
			movedCount++
		}
	}

	if movedCount == 0 {
		t.Error("GPU compute did not update any boid positions")
	}

	t.Logf("GPU compute updated %d/%d boids", movedCount, numBoids)
}

func BenchmarkGPUCompute(b *testing.B) {
	numBoids := 1000
	width := 800
	height := 600

	config := Config{
		BoidsConfig: config.BoidsConfig{
			NumBoids:         numBoids,
			MaxSpeed:         50.0,
			MaxForce:         80.0,
			SeparationRadius: 5.0,
			AlignmentRadius:  35.0,
			CohesionRadius:   45.0,
			SeparationWeight: 1.8,
			AlignmentWeight:  1.2,
			CohesionWeight:   1.0,
			RandomWeight:     0.15,
			RenderRadius:     1,
		},
		ColorMode: ColorModeDistance,
		UseGPU:    true,
	}

	sim := NewSimulation(numBoids, width, height, config)
	if sim.gpuCompute == nil {
		b.Skip("GPU not available")
	}
	defer sim.Release()

	deltaTime := 0.016

	for b.Loop() {
		sim.Update(deltaTime)
	}
}

func BenchmarkCPUCompute(b *testing.B) {
	numBoids := 1000
	width := 800
	height := 600

	config := Config{
		BoidsConfig: config.BoidsConfig{
			NumBoids:         numBoids,
			MaxSpeed:         50.0,
			MaxForce:         80.0,
			SeparationRadius: 5.0,
			AlignmentRadius:  35.0,
			CohesionRadius:   45.0,
			SeparationWeight: 1.8,
			AlignmentWeight:  1.2,
			CohesionWeight:   1.0,
			RandomWeight:     0.15,
			RenderRadius:     1,
		},
		ColorMode: ColorModeDistance,
		UseGPU:    false,
	}

	sim := NewSimulation(numBoids, width, height, config)
	defer sim.Release()

	deltaTime := 0.016

	for b.Loop() {
		sim.Update(deltaTime)
	}
}
