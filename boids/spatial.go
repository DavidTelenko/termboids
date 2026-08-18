package boids

// SpatialGrid provides efficient spatial partitioning for boid lookups
type SpatialGrid struct {
	cellSize    int
	width       int
	height      int
	cols        int
	rows        int
	cells       map[int][]*Boid
	resultPool  []*Boid // Reusable slice for query results
	maxCapacity int     // Maximum expected neighbors
}

// NewSpatialGrid creates a new spatial partitioning grid
func NewSpatialGrid(width, height, cellSize int) *SpatialGrid {
	cols := (width / cellSize) + 1
	rows := (height / cellSize) + 1

	return &SpatialGrid{
		cellSize:    cellSize,
		width:       width,
		height:      height,
		cols:        cols,
		rows:        rows,
		cells:       make(map[int][]*Boid, cols*rows),
		resultPool:  make([]*Boid, 0, 100), // Pre-allocate for typical query size
		maxCapacity: 100,
	}
}

// Clear removes all boids from the grid
func (g *SpatialGrid) Clear() {
	// Reuse existing slices instead of deleting
	for key := range g.cells {
		g.cells[key] = g.cells[key][:0]
	}
}

// Insert adds a boid to the grid
func (g *SpatialGrid) Insert(boid *Boid) {
	cellX := int(boid.Position.X) / g.cellSize
	cellY := int(boid.Position.Y) / g.cellSize

	// Clamp to grid bounds
	if cellX < 0 {
		cellX = 0
	}
	if cellX >= g.cols {
		cellX = g.cols - 1
	}
	if cellY < 0 {
		cellY = 0
	}
	if cellY >= g.rows {
		cellY = g.rows - 1
	}

	key := cellY*g.cols + cellX

	// Pre-allocate slice with reasonable capacity if not exists
	if g.cells[key] == nil {
		g.cells[key] = make([]*Boid, 0, 16)
	}

	g.cells[key] = append(g.cells[key], boid)
}

// QueryRadius returns all boids within a given radius of a position
func (g *SpatialGrid) QueryRadius(pos Vector2D, radius float64) []*Boid {
	radiusSquared := radius * radius

	// Reuse the result pool
	g.resultPool = g.resultPool[:0]

	// Calculate cell range to check
	minCellX := int(pos.X-radius) / g.cellSize
	maxCellX := int(pos.X+radius) / g.cellSize
	minCellY := int(pos.Y-radius) / g.cellSize
	maxCellY := int(pos.Y+radius) / g.cellSize

	// Clamp to grid bounds
	if minCellX < 0 {
		minCellX = 0
	}
	if maxCellX >= g.cols {
		maxCellX = g.cols - 1
	}
	if minCellY < 0 {
		minCellY = 0
	}
	if maxCellY >= g.rows {
		maxCellY = g.rows - 1
	}

	// Check all cells in range
	for cy := minCellY; cy <= maxCellY; cy++ {
		for cx := minCellX; cx <= maxCellX; cx++ {
			key := cy*g.cols + cx
			if cell := g.cells[key]; len(cell) > 0 {
				for _, boid := range cell {
					if pos.DistanceSquared(boid.Position) <= radiusSquared {
						g.resultPool = append(g.resultPool, boid)
					}
				}
			}
		}
	}

	return g.resultPool
}
