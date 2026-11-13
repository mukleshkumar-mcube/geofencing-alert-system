package geo

import (
	"fmt"
	"math"
)

// Categories accepted by POST /geofences.
var categories = map[string]struct{}{
	"delivery_zone":   {},
	"restricted_zone": {},
	"toll_zone":       {},
	"customer_area":   {},
}

// ValidCategory reports whether category is one of the four zone types.
func ValidCategory(category string) bool {
	_, ok := categories[category]
	return ok
}

// ValidatePoint checks a single latitude/longitude pair.
func ValidatePoint(lat, lng float64) error {
	if math.IsNaN(lat) || math.IsInf(lat, 0) || lat < -90 || lat > 90 {
		return fmt.Errorf("latitude must be between -90 and 90")
	}
	if math.IsNaN(lng) || math.IsInf(lng, 0) || lng < -180 || lng > 180 {
		return fmt.Errorf("longitude must be between -180 and 180")
	}
	return nil
}

// ValidatePolygon checks a closed [latitude, longitude] ring.
// The ring needs at least four positions: three corners and a repeated closing point.
func ValidatePolygon(coords [][]float64) error {
	if len(coords) < 4 {
		return fmt.Errorf("a polygon needs at least 4 points (3 corners plus the closing point)")
	}
	if len(coords) > 500 {
		return fmt.Errorf("a polygon can have at most 500 points")
	}
	for i, p := range coords {
		if len(p) != 2 {
			return fmt.Errorf("point %d must be a [latitude, longitude] pair", i+1)
		}
		if err := ValidatePoint(p[0], p[1]); err != nil {
			return fmt.Errorf("point %d: %w", i+1, err)
		}
	}
	first, last := coords[0], coords[len(coords)-1]
	if !near(first[0], last[0]) || !near(first[1], last[1]) {
		return fmt.Errorf("coordinates must form a closed polygon: the first and last points must match")
	}
	if Area(coords) < 1e-12 {
		return fmt.Errorf("polygon area is too small; the corners look collinear")
	}
	return nil
}

// Bounds is the latitude/longitude envelope of a ring.
type Bounds struct {
	MinLat, MaxLat float64
	MinLng, MaxLng float64
}

// BoundsOf returns the envelope used as a SQL prefilter before the precise point test.
func BoundsOf(coords [][]float64) Bounds {
	b := Bounds{
		MinLat: coords[0][0], MaxLat: coords[0][0],
		MinLng: coords[0][1], MaxLng: coords[0][1],
	}
	for _, p := range coords[1:] {
		b.MinLat = math.Min(b.MinLat, p[0])
		b.MaxLat = math.Max(b.MaxLat, p[0])
		b.MinLng = math.Min(b.MinLng, p[1])
		b.MaxLng = math.Max(b.MaxLng, p[1])
	}
	return b
}

// Area is the absolute shoelace area in degree-space. It rejects collapsed rings.
func Area(coords [][]float64) float64 {
	pts := openRing(coords)
	var sum float64
	n := len(pts)
	for i := 0; i < n; i++ {
		j := (i + 1) % n
		sum += pts[i][1]*pts[j][0] - pts[j][1]*pts[i][0]
	}
	return math.Abs(sum) / 2
}

// PointInPolygon reports whether (lat, lng) is inside the ring using the even-odd
// ray cast. Horizontal edges are skipped. Points that sit exactly on a boundary
// are not treated as a special case; callers that need boundary inclusion should
// keep a small margin inside the fence.
func PointInPolygon(lat, lng float64, ring [][]float64) bool {
	pts := openRing(ring)
	n := len(pts)
	if n < 3 {
		return false
	}
	inside := false
	j := n - 1
	for i := 0; i < n; i++ {
		yi, xi := pts[i][0], pts[i][1]
		yj, xj := pts[j][0], pts[j][1]
		if (yi > lat) != (yj > lat) {
			xint := (xj-xi)*(lat-yi)/(yj-yi) + xi
			if lng < xint {
				inside = !inside
			}
		}
		j = i
	}
	return inside
}

func openRing(coords [][]float64) [][]float64 {
	n := len(coords)
	if n > 1 && near(coords[0][0], coords[n-1][0]) && near(coords[0][1], coords[n-1][1]) {
		return coords[:n-1]
	}
	return coords
}

func near(a, b float64) bool {
	return math.Abs(a-b) <= 1e-9
}
