package main

const (
	EPSILON = 0.05
)

func approx(a, b float64) bool {
	return (a - b) < EPSILON && (b - a) < EPSILON
}
