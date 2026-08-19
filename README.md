# termboids

Boids flocking simulation in your terminal using Braille characters.

## Features

- Real-time flocking simulation with separation, alignment, and cohesion behaviors
- **GPU-accelerated compute shaders** for high-performance boid calculations
- Multiple color modes (distance-based, force-based, or monochrome)
- Spatial grid optimization for efficient neighbor queries
- Frame-rate independent physics simulation
- Configurable parameters via TOML config file

## Performance

GPU acceleration provides significant performance improvements:

- **CPU**: ~5.7ms per update (1000 boids)
- **GPU**: ~0.26ms per update (1000 boids)
- **Speedup**: ~21x faster with GPU compute shaders

The GPU implementation uses WebGPU compute shaders (WGSL) to parallelize boid force calculations across all boids simultaneously.

## Install

```bash
go build
```

## Run

```bash
./termboids
```

## Controls

- `c` - cycle color modes (distance/force/none)
- `d` - toggle spatial grid debug view
- `q` - quit

## Config

Edit `config.toml` to customize boid behavior and settings:

- `use_gpu` - Enable/disable GPU compute acceleration (default: true)
- Boid behavior parameters (speed, forces, radii, weights)
- Number of boids
- Keybindings

## GPU Requirements

GPU acceleration requires:

- WebGPU-compatible GPU and drivers
- Vulkan, Metal, or DirectX 12 support

The simulation automatically falls back to CPU if GPU initialization fails.
