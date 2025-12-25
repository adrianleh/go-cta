package cta

type Position struct {
	Latitude  float64
	Longitude float64
}

type DirectedPosition struct {
	Position Position
	Heading  float64
}
