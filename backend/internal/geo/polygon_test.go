package geo

import "testing"

func TestValidatePolygon(t *testing.T) {
	closed := [][]float64{{0, 0}, {0, 2}, {2, 2}, {2, 0}, {0, 0}}
	if err := ValidatePolygon(closed); err != nil {
		t.Fatal(err)
	}
	open := [][]float64{{0, 0}, {0, 2}, {2, 2}, {2, 0}}
	if err := ValidatePolygon(open); err == nil {
		t.Fatal("expected open ring to fail")
	}
	flat := [][]float64{{0, 0}, {1, 0}, {2, 0}, {0, 0}}
	if err := ValidatePolygon(flat); err == nil {
		t.Fatal("expected collinear ring to fail")
	}
	high := [][]float64{{91, 0}, {91, 1}, {92, 1}, {91, 0}}
	if err := ValidatePolygon(high); err == nil {
		t.Fatal("expected latitude to fail")
	}
}

func TestPointInPolygonSquare(t *testing.T) {
	ring := [][]float64{{0, 0}, {0, 2}, {2, 2}, {2, 0}, {0, 0}}
	if !PointInPolygon(1, 1, ring) {
		t.Fatal("center should be inside")
	}
	if PointInPolygon(1, 3, ring) {
		t.Fatal("east point should be outside")
	}
	if PointInPolygon(-0.1, 1, ring) {
		t.Fatal("south point should be outside")
	}
}

func TestDowntownSample(t *testing.T) {
	ring := [][]float64{
		{37.7749, -122.4194},
		{37.7849, -122.4194},
		{37.7849, -122.4094},
		{37.7749, -122.4094},
		{37.7749, -122.4194},
	}
	if !PointInPolygon(37.7799, -122.4144, ring) {
		t.Fatal("downtown center should be inside")
	}
	if PointInPolygon(37.7799, -122.4280, ring) {
		t.Fatal("point west of downtown should be outside")
	}
}

func TestConcaveMouthIsOutside(t *testing.T) {
	// A block letter C opening east. The bounding box contains the mouth,
	// so this fails if the bbox check is used without the ray cast.
	ring := [][]float64{
		{0, 0},
		{4, 0},
		{4, 3},
		{3, 3},
		{3, 1},
		{1, 1},
		{1, 3},
		{0, 3},
		{0, 0},
	}
	if err := ValidatePolygon(ring); err != nil {
		t.Fatal(err)
	}
	if !PointInPolygon(2, 0.5, ring) {
		t.Fatal("left bar should be inside")
	}
	if PointInPolygon(2, 2, ring) {
		t.Fatal("mouth should be outside")
	}
}
