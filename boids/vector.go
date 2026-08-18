package boids

import "math"

// Vector2D represents a 2D vector
type Vector2D struct {
	X, Y float64
}

// Add returns the sum of two vectors
func (v Vector2D) Add(other Vector2D) Vector2D {
	return Vector2D{v.X + other.X, v.Y + other.Y}
}

// Sub returns the difference of two vectors
func (v Vector2D) Sub(other Vector2D) Vector2D {
	return Vector2D{v.X - other.X, v.Y - other.Y}
}

// Scale returns the vector scaled by a scalar
func (v Vector2D) Scale(s float64) Vector2D {
	return Vector2D{v.X * s, v.Y * s}
}

// Length returns the magnitude of the vector
func (v Vector2D) Length() float64 {
	return math.Sqrt(v.X*v.X + v.Y*v.Y)
}

// LengthSquared returns the squared magnitude (faster than Length)
func (v Vector2D) LengthSquared() float64 {
	return v.X*v.X + v.Y*v.Y
}

// Normalize returns a unit vector in the same direction
func (v Vector2D) Normalize() Vector2D {
	l := v.Length()
	if l == 0 {
		return Vector2D{0, 0}
	}
	return Vector2D{v.X / l, v.Y / l}
}

// Limit returns the vector limited to a maximum magnitude
func (v Vector2D) Limit(max float64) Vector2D {
	if v.Length() > max {
		return v.Normalize().Scale(max)
	}
	return v
}

// Distance returns the distance between two points
func (v Vector2D) Distance(other Vector2D) float64 {
	return v.Sub(other).Length()
}

// DistanceSquared returns the squared distance (faster than Distance)
func (v Vector2D) DistanceSquared(other Vector2D) float64 {
	return v.Sub(other).LengthSquared()
}
