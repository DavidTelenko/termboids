package boids

import (
	_ "embed"
	"fmt"
	"unsafe"

	"github.com/rajveermalviya/go-webgpu/wgpu"
)

//go:embed shaders/boids.wgsl
var boidShaderCode string

// GPUBoid represents the GPU-side boid data structure (must match shader layout)
type GPUBoid struct {
	PosX float32
	PosY float32
	VelX float32
	VelY float32
}

// GPUConfig represents the uniform buffer data (must match shader layout)
type GPUConfig struct {
	MaxSpeed         float32
	MaxForce         float32
	SeparationRadius float32
	AlignmentRadius  float32
	CohesionRadius   float32
	SeparationWeight float32
	AlignmentWeight  float32
	CohesionWeight   float32
	RandomWeight     float32
	Width            float32
	Height           float32
	DeltaTime        float32
	NumBoids         uint32
	FrameCount       uint32
	_padding         [2]uint32 // Align to 16 bytes for uniform buffers
}

// GPUCompute handles GPU-accelerated boid computation
type GPUCompute struct {
	device          *wgpu.Device
	queue           *wgpu.Queue
	pipeline        *wgpu.ComputePipeline
	bindGroup0      *wgpu.BindGroup // Bind group for buffer config 0 (in=0, out=1)
	bindGroup1      *wgpu.BindGroup // Bind group for buffer config 1 (in=1, out=0)
	bufferIn        *wgpu.Buffer
	bufferOut       *wgpu.Buffer
	bufferStaging   *wgpu.Buffer
	configBuffer    *wgpu.Buffer
	numBoids        int
	frameCount      uint32
	workgroupSize   int
	bindGroupLayout *wgpu.BindGroupLayout
	useBindGroup0   bool // Toggle between bind groups
}

// NewGPUCompute creates a new GPU compute context for boid simulation
func NewGPUCompute(numBoids int) (*GPUCompute, error) {
	// Initialize WebGPU instance
	instance := wgpu.CreateInstance(nil)
	if instance == nil {
		return nil, fmt.Errorf("failed to create WebGPU instance")
	}
	defer instance.Release()

	// Request adapter (GPU)
	adapter, err := instance.RequestAdapter(&wgpu.RequestAdapterOptions{
		PowerPreference: wgpu.PowerPreference_HighPerformance,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to request adapter: %w", err)
	}
	defer adapter.Release()

	// Get adapter info (silently - don't interfere with terminal rendering)
	_ = adapter.GetProperties()

	// Request device
	device, err := adapter.RequestDevice(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to request device: %w", err)
	}

	// Get queue
	queue := device.GetQueue()

	// Create shader module
	shaderModule, err := device.CreateShaderModule(&wgpu.ShaderModuleDescriptor{
		Label: "Boid Compute Shader",
		WGSLDescriptor: &wgpu.ShaderModuleWGSLDescriptor{
			Code: boidShaderCode,
		},
	})
	if err != nil {
		device.Release()
		return nil, fmt.Errorf("failed to create shader module: %w", err)
	}
	defer shaderModule.Release()

	// Calculate buffer sizes
	boidSize := uint64(unsafe.Sizeof(GPUBoid{}))
	bufferSize := boidSize * uint64(numBoids)
	configSize := uint64(unsafe.Sizeof(GPUConfig{}))

	// Create buffers
	bufferIn, err := device.CreateBuffer(&wgpu.BufferDescriptor{
		Label: "Boid Input Buffer",
		Size:  bufferSize,
		Usage: wgpu.BufferUsage_Storage | wgpu.BufferUsage_CopyDst | wgpu.BufferUsage_CopySrc,
	})
	if err != nil {
		device.Release()
		return nil, fmt.Errorf("failed to create input buffer: %w", err)
	}

	bufferOut, err := device.CreateBuffer(&wgpu.BufferDescriptor{
		Label: "Boid Output Buffer",
		Size:  bufferSize,
		Usage: wgpu.BufferUsage_Storage | wgpu.BufferUsage_CopyDst | wgpu.BufferUsage_CopySrc,
	})
	if err != nil {
		bufferIn.Release()
		device.Release()
		return nil, fmt.Errorf("failed to create output buffer: %w", err)
	}

	bufferStaging, err := device.CreateBuffer(&wgpu.BufferDescriptor{
		Label: "Boid Staging Buffer",
		Size:  bufferSize,
		Usage: wgpu.BufferUsage_MapRead | wgpu.BufferUsage_CopyDst,
	})
	if err != nil {
		bufferIn.Release()
		bufferOut.Release()
		device.Release()
		return nil, fmt.Errorf("failed to create staging buffer: %w", err)
	}

	configBuffer, err := device.CreateBuffer(&wgpu.BufferDescriptor{
		Label: "Config Uniform Buffer",
		Size:  configSize,
		Usage: wgpu.BufferUsage_Uniform | wgpu.BufferUsage_CopyDst,
	})
	if err != nil {
		bufferIn.Release()
		bufferOut.Release()
		bufferStaging.Release()
		device.Release()
		return nil, fmt.Errorf("failed to create config buffer: %w", err)
	}

	// Create bind group layout
	bindGroupLayout, err := device.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Label: "Boid Compute Bind Group Layout",
		Entries: []wgpu.BindGroupLayoutEntry{
			{
				Binding:    0,
				Visibility: wgpu.ShaderStage_Compute,
				Buffer: wgpu.BufferBindingLayout{
					Type:             wgpu.BufferBindingType_ReadOnlyStorage,
					HasDynamicOffset: false,
				},
			},
			{
				Binding:    1,
				Visibility: wgpu.ShaderStage_Compute,
				Buffer: wgpu.BufferBindingLayout{
					Type:             wgpu.BufferBindingType_Storage,
					HasDynamicOffset: false,
				},
			},
			{
				Binding:    2,
				Visibility: wgpu.ShaderStage_Compute,
				Buffer: wgpu.BufferBindingLayout{
					Type:             wgpu.BufferBindingType_Uniform,
					HasDynamicOffset: false,
				},
			},
		},
	})
	if err != nil {
		bufferIn.Release()
		bufferOut.Release()
		bufferStaging.Release()
		configBuffer.Release()
		device.Release()
		return nil, fmt.Errorf("failed to create bind group layout: %w", err)
	}

	// Create bind group 0 (in=bufferIn, out=bufferOut)
	bindGroup0, err := device.CreateBindGroup(&wgpu.BindGroupDescriptor{
		Label:  "Boid Compute Bind Group 0",
		Layout: bindGroupLayout,
		Entries: []wgpu.BindGroupEntry{
			{
				Binding: 0,
				Buffer:  bufferIn,
				Offset:  0,
				Size:    bufferSize,
			},
			{
				Binding: 1,
				Buffer:  bufferOut,
				Offset:  0,
				Size:    bufferSize,
			},
			{
				Binding: 2,
				Buffer:  configBuffer,
				Offset:  0,
				Size:    configSize,
			},
		},
	})
	if err != nil {
		bufferIn.Release()
		bufferOut.Release()
		bufferStaging.Release()
		configBuffer.Release()
		bindGroupLayout.Release()
		device.Release()
		return nil, fmt.Errorf("failed to create bind group 0: %w", err)
	}

	// Create bind group 1 (in=bufferOut, out=bufferIn) for ping-pong
	bindGroup1, err := device.CreateBindGroup(&wgpu.BindGroupDescriptor{
		Label:  "Boid Compute Bind Group 1",
		Layout: bindGroupLayout,
		Entries: []wgpu.BindGroupEntry{
			{
				Binding: 0,
				Buffer:  bufferOut,
				Offset:  0,
				Size:    bufferSize,
			},
			{
				Binding: 1,
				Buffer:  bufferIn,
				Offset:  0,
				Size:    bufferSize,
			},
			{
				Binding: 2,
				Buffer:  configBuffer,
				Offset:  0,
				Size:    configSize,
			},
		},
	})
	if err != nil {
		bufferIn.Release()
		bufferOut.Release()
		bufferStaging.Release()
		configBuffer.Release()
		bindGroupLayout.Release()
		bindGroup0.Release()
		device.Release()
		return nil, fmt.Errorf("failed to create bind group 1: %w", err)
	}

	// Create pipeline layout
	pipelineLayout, err := device.CreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{
		Label:            "Boid Compute Pipeline Layout",
		BindGroupLayouts: []*wgpu.BindGroupLayout{bindGroupLayout},
	})
	if err != nil {
		bufferIn.Release()
		bufferOut.Release()
		bufferStaging.Release()
		configBuffer.Release()
		bindGroupLayout.Release()
		bindGroup0.Release()
		bindGroup1.Release()
		device.Release()
		return nil, fmt.Errorf("failed to create pipeline layout: %w", err)
	}
	defer pipelineLayout.Release()

	// Create compute pipeline
	pipeline, err := device.CreateComputePipeline(&wgpu.ComputePipelineDescriptor{
		Label:  "Boid Compute Pipeline",
		Layout: pipelineLayout,
		Compute: wgpu.ProgrammableStageDescriptor{
			Module:     shaderModule,
			EntryPoint: "main",
		},
	})
	if err != nil {
		bufferIn.Release()
		bufferOut.Release()
		bufferStaging.Release()
		configBuffer.Release()
		bindGroupLayout.Release()
		bindGroup0.Release()
		bindGroup1.Release()
		device.Release()
		return nil, fmt.Errorf("failed to create compute pipeline: %w", err)
	}

	return &GPUCompute{
		device:          device,
		queue:           queue,
		pipeline:        pipeline,
		bindGroup0:      bindGroup0,
		bindGroup1:      bindGroup1,
		bufferIn:        bufferIn,
		bufferOut:       bufferOut,
		bufferStaging:   bufferStaging,
		configBuffer:    configBuffer,
		numBoids:        numBoids,
		workgroupSize:   64, // Must match shader @workgroup_size
		bindGroupLayout: bindGroupLayout,
		useBindGroup0:   true,
	}, nil
}

// UploadBoids uploads boid data to GPU
func (g *GPUCompute) UploadBoids(boids []*Boid) error {
	// Convert to GPU format
	gpuBoids := make([]GPUBoid, len(boids))
	for i, boid := range boids {
		gpuBoids[i] = GPUBoid{
			PosX: float32(boid.Position.X),
			PosY: float32(boid.Position.Y),
			VelX: float32(boid.Velocity.X),
			VelY: float32(boid.Velocity.Y),
		}
	}

	// Write to GPU buffer
	data := unsafe.Slice((*byte)(unsafe.Pointer(&gpuBoids[0])), len(gpuBoids)*int(unsafe.Sizeof(GPUBoid{})))
	g.queue.WriteBuffer(g.bufferIn, 0, data)

	return nil
}

// UploadConfig uploads simulation config to GPU
func (g *GPUCompute) UploadConfig(config Config, width, height int, deltaTime float64) error {
	gpuConfig := GPUConfig{
		MaxSpeed:         float32(config.MaxSpeed),
		MaxForce:         float32(config.MaxForce),
		SeparationRadius: float32(config.SeparationRadius),
		AlignmentRadius:  float32(config.AlignmentRadius),
		CohesionRadius:   float32(config.CohesionRadius),
		SeparationWeight: float32(config.SeparationWeight),
		AlignmentWeight:  float32(config.AlignmentWeight),
		CohesionWeight:   float32(config.CohesionWeight),
		RandomWeight:     float32(config.RandomWeight),
		Width:            float32(width),
		Height:           float32(height),
		DeltaTime:        float32(deltaTime),
		NumBoids:         uint32(g.numBoids),
		FrameCount:       g.frameCount,
	}

	g.frameCount++

	data := unsafe.Slice((*byte)(unsafe.Pointer(&gpuConfig)), int(unsafe.Sizeof(GPUConfig{})))
	g.queue.WriteBuffer(g.configBuffer, 0, data)

	return nil
}

// Compute runs the compute shader
func (g *GPUCompute) Compute() error {
	// Create command encoder
	encoder, err := g.device.CreateCommandEncoder(nil)
	if err != nil {
		return fmt.Errorf("failed to create command encoder: %w", err)
	}
	defer encoder.Release()

	// Create compute pass
	computePass := encoder.BeginComputePass(nil)
	computePass.SetPipeline(g.pipeline)
	
	// Use the appropriate bind group for ping-pong buffering
	if g.useBindGroup0 {
		computePass.SetBindGroup(0, g.bindGroup0, nil)
	} else {
		computePass.SetBindGroup(0, g.bindGroup1, nil)
	}

	// Dispatch compute shader
	// Calculate number of workgroups needed (round up)
	numWorkgroups := (g.numBoids + g.workgroupSize - 1) / g.workgroupSize
	computePass.DispatchWorkgroups(uint32(numWorkgroups), 1, 1)

	computePass.End()

	// Submit commands
	commandBuffer, err := encoder.Finish(nil)
	if err != nil {
		return fmt.Errorf("failed to finish command encoder: %w", err)
	}
	defer commandBuffer.Release()

	g.queue.Submit(commandBuffer)
	
	// Ensure compute shader completes before returning
	g.device.Poll(true, nil)

	return nil
}

// DownloadBoids downloads boid data from GPU
func (g *GPUCompute) DownloadBoids(boids []*Boid) error {
	// Determine which buffer has the output based on current bind group
	var outputBuffer *wgpu.Buffer
	if g.useBindGroup0 {
		outputBuffer = g.bufferOut
	} else {
		outputBuffer = g.bufferIn
	}

	// Copy output buffer to staging buffer
	encoder, err := g.device.CreateCommandEncoder(nil)
	if err != nil {
		return fmt.Errorf("failed to create command encoder: %w", err)
	}
	defer encoder.Release()

	bufferSize := uint64(unsafe.Sizeof(GPUBoid{})) * uint64(g.numBoids)
	encoder.CopyBufferToBuffer(outputBuffer, 0, g.bufferStaging, 0, bufferSize)

	commandBuffer, err := encoder.Finish(nil)
	if err != nil {
		return fmt.Errorf("failed to finish command encoder: %w", err)
	}
	defer commandBuffer.Release()

	g.queue.Submit(commandBuffer)

	// Map staging buffer for reading
	var status wgpu.BufferMapAsyncStatus
	callback := func(s wgpu.BufferMapAsyncStatus) {
		status = s
	}

	g.bufferStaging.MapAsync(wgpu.MapMode_Read, 0, bufferSize, callback)

	// Wait for mapping to complete
	g.device.Poll(true, nil)

	if status != wgpu.BufferMapAsyncStatus_Success {
		return fmt.Errorf("failed to map staging buffer")
	}

	// Read data
	mappedData := g.bufferStaging.GetMappedRange(0, uint(bufferSize))
	gpuBoids := unsafe.Slice((*GPUBoid)(unsafe.Pointer(&mappedData[0])), g.numBoids)

	// Copy to CPU boids
	for i := 0; i < g.numBoids && i < len(boids); i++ {
		boids[i].Position.X = float64(gpuBoids[i].PosX)
		boids[i].Position.Y = float64(gpuBoids[i].PosY)
		boids[i].Velocity.X = float64(gpuBoids[i].VelX)
		boids[i].Velocity.Y = float64(gpuBoids[i].VelY)
	}

	// Unmap buffer
	g.bufferStaging.Unmap()

	// Toggle bind group for next frame (ping-pong)
	g.useBindGroup0 = !g.useBindGroup0

	return nil
}

// Release frees GPU resources
func (g *GPUCompute) Release() {
	if g.bindGroup0 != nil {
		g.bindGroup0.Release()
	}
	if g.bindGroup1 != nil {
		g.bindGroup1.Release()
	}
	if g.bindGroupLayout != nil {
		g.bindGroupLayout.Release()
	}
	if g.pipeline != nil {
		g.pipeline.Release()
	}
	if g.configBuffer != nil {
		g.configBuffer.Release()
	}
	if g.bufferStaging != nil {
		g.bufferStaging.Release()
	}
	if g.bufferOut != nil {
		g.bufferOut.Release()
	}
	if g.bufferIn != nil {
		g.bufferIn.Release()
	}
	if g.queue != nil {
		g.queue.Release()
	}
	if g.device != nil {
		g.device.Release()
	}
}
