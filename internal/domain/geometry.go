package domain

import "math"

func Distance(a, b Point) float64 { return math.Hypot(a.X-b.X, a.Y-b.Y) }

func PointInPolygon(p Point, polygon Polygon) bool {
	n := len(polygon.Points)
	if n < 3 {
		return false
	}
	inside := false
	for i, j := 0, n-1; i < n; j, i = i, i+1 {
		a, b := polygon.Points[i], polygon.Points[j]
		if pointOnSegment(p, a, b) {
			return true
		}
		crosses := (a.Y > p.Y) != (b.Y > p.Y)
		if crosses && p.X < (b.X-a.X)*(p.Y-a.Y)/(b.Y-a.Y)+a.X {
			inside = !inside
		}
	}
	return inside
}

func DistanceToPolygonBoundary(p Point, polygon Polygon) float64 {
	if len(polygon.Points) == 0 {
		return math.Inf(1)
	}
	minimum := math.Inf(1)
	for i := range polygon.Points {
		a := polygon.Points[i]
		b := polygon.Points[(i+1)%len(polygon.Points)]
		if d := pointSegmentDistance(p, a, b); d < minimum {
			minimum = d
		}
	}
	return minimum
}

func pointOnSegment(p, a, b Point) bool {
	cross := (p.Y-a.Y)*(b.X-a.X) - (p.X-a.X)*(b.Y-a.Y)
	if math.Abs(cross) > 1e-8 {
		return false
	}
	dot := (p.X-a.X)*(b.X-a.X) + (p.Y-a.Y)*(b.Y-a.Y)
	return dot >= 0 && dot <= math.Pow(Distance(a, b), 2)
}

func pointSegmentDistance(p, a, b Point) float64 {
	dx, dy := b.X-a.X, b.Y-a.Y
	lengthSquared := dx*dx + dy*dy
	if lengthSquared == 0 {
		return Distance(p, a)
	}
	t := ((p.X-a.X)*dx + (p.Y-a.Y)*dy) / lengthSquared
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	return Distance(p, Point{X: a.X + t*dx, Y: a.Y + t*dy})
}

func PolygonArea(p Polygon) float64 {
	area := 0.0
	for i := range p.Points {
		a, b := p.Points[i], p.Points[(i+1)%len(p.Points)]
		area += a.X*b.Y - b.X*a.Y
	}
	return math.Abs(area) / 2
}

func IsFinitePoint(p Point) bool {
	return !math.IsNaN(p.X) && !math.IsNaN(p.Y) && !math.IsInf(p.X, 0) && !math.IsInf(p.Y, 0)
}

func pointsEqual(a, b Point) bool { return a.X == b.X && a.Y == b.Y }

// SegmentIntersection 判断两条闭合线段是否相交。
func SegmentIntersection(a, b, c, d Point) bool {
	o1, o2 := orientation(a, b, c), orientation(a, b, d)
	o3, o4 := orientation(c, d, a), orientation(c, d, b)
	if o1*o2 < 0 && o3*o4 < 0 {
		return true
	}
	return (o1 == 0 && pointOnSegment(c, a, b)) || (o2 == 0 && pointOnSegment(d, a, b)) ||
		(o3 == 0 && pointOnSegment(a, c, d)) || (o4 == 0 && pointOnSegment(b, c, d))
}

func orientation(a, b, c Point) int {
	v := (b.X-a.X)*(c.Y-a.Y) - (b.Y-a.Y)*(c.X-a.X)
	if math.Abs(v) <= 1e-8 {
		return 0
	}
	if v > 0 {
		return 1
	}
	return -1
}

func PolygonsOverlap(a, b Polygon) bool {
	for i := range a.Points {
		for j := range b.Points {
			if SegmentIntersection(a.Points[i], a.Points[(i+1)%len(a.Points)], b.Points[j], b.Points[(j+1)%len(b.Points)]) {
				return true
			}
		}
	}
	return len(a.Points) > 0 && len(b.Points) > 0 && (PointInPolygon(a.Points[0], b) || PointInPolygon(b.Points[0], a))
}

func polygonBounds(p Polygon) (float64, float64, float64, float64) {
	minX, minY, maxX, maxY := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, point := range p.Points {
		minX, minY = math.Min(minX, point.X), math.Min(minY, point.Y)
		maxX, maxY = math.Max(maxX, point.X), math.Max(maxY, point.Y)
	}
	return minX, minY, maxX, maxY
}

// HasSamplingArea 在精确边界和拓扑校验通过后，以确定性密集采样预检保护缓冲区外是否存在可采区域。
func HasSamplingArea(segment Polygon, zones []Polygon, buffer float64) bool {
	minX, minY, maxX, maxY := polygonBounds(segment)
	if !isFinite(minX) || maxX <= minX || maxY <= minY {
		return false
	}
	const divisions = 80
	for xi := 0; xi <= divisions; xi++ {
		for yi := 0; yi <= divisions; yi++ {
			p := Point{X: minX + (maxX-minX)*float64(xi)/divisions, Y: minY + (maxY-minY)*float64(yi)/divisions}
			if !PointInPolygon(p, segment) || DistanceToPolygonBoundary(p, segment) < buffer {
				continue
			}
			protected := false
			for _, zone := range zones {
				if PointInPolygon(p, zone) || DistanceToPolygonBoundary(p, zone) < buffer {
					protected = true
					break
				}
			}
			if !protected {
				return true
			}
		}
	}
	return false
}

func isFinite(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }

func clonePolygon(polygon Polygon) Polygon {
	return Polygon{Points: append([]Point(nil), polygon.Points...)}
}

func clonePolygons(polygons []Polygon) []Polygon {
	result := make([]Polygon, len(polygons))
	for i := range polygons {
		result[i] = clonePolygon(polygons[i])
	}
	return result
}
