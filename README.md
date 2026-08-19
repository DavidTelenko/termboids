# termboids

Boids flocking simulation in your terminal using Braille characters.

## Features

- Real-time flocking simulation with separation, alignment, and cohesion behaviors
- **GPU-accelerated compute shaders** for high-performance boid calculations
- **Interactive mouse controls** - left click to repel, right click to attract boids
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
- `h` - toggle help/status bar
- `i` - show configuration info
- `p` - pause/unpause simulation
- `1-0` - load preset configurations
- `q` - quit
- **Left mouse click** - create temporary repellant point (boids steer away)
- **Right mouse click** - create temporary attractor point (boids move towards)

## Config

Configuration is split across multiple files:

### `system.toml` - System Settings

- `fps` - Target framerate
- `use_gpu` - Enable/disable GPU compute acceleration (default: true)
- Keybindings
- Preset configurations
- **Repellant settings (left click):**
  - `radius` - Area of effect when clicking (pixels)
  - `strength` - Force multiplier (higher = stronger push)
  - `duration` - How long the effect lasts (seconds)
- **Attractor settings (right click):**
  - `radius` - Area of effect when clicking (pixels)
  - `strength` - Force multiplier (higher = stronger pull)
  - `duration` - How long the effect lasts (seconds)

Example mouse interaction configuration in `system.toml`:

```toml
[repellant]
radius = 200.0      # 200 pixel radius
strength = 10.0     # 10x normal force
duration = 2.0      # lasts 2 seconds

[attractor]
radius = 200.0      # 200 pixel radius
strength = 10.0     # 10x normal force
duration = 2.0      # lasts 2 seconds
```

### `configs/*.toml` - Boid Behavior Presets

- Boid behavior parameters (speed, forces, radii, weights)
- Number of boids
- Render settings

## GPU Requirements

GPU acceleration requires:

- WebGPU-compatible GPU and drivers
- Vulkan, Metal, or DirectX 12 support

The simulation automatically falls back to CPU if GPU initialization fails.
