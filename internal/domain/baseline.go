package domain

import (
	"fmt"
	"strings"
	"time"
)

type BaselineInput struct {
	ID, CoordinateSystem                string
	SegmentPolygon                      Polygon
	ExclusionZones                      []Polygon
	ReferencePoints                     []Point
	BufferDistanceMM, MinimumSpacingMM  float64
	MaxCoreVolumeMM3, MaxTotalVolumeMM3 float64
	Now                                 time.Time
}

func (a *Application) FreezeBaseline(in BaselineInput) error {
	if err := a.RequireState(StateDraft); err != nil {
		return err
	}
	var problems []Violation
	if strings.TrimSpace(in.ID) == "" {
		problems = append(problems, violation("baseline_id", "required", "基线编号不能为空"))
	}
	if strings.TrimSpace(in.CoordinateSystem) == "" {
		problems = append(problems, violation("coordinate_system", "required", "坐标系不能为空"))
	}
	problems = append(problems, validatePolygon("segment_polygon", in.SegmentPolygon)...)
	for i, zone := range in.ExclusionZones {
		field := fmt.Sprintf("exclusion_zones[%d]", i)
		zoneProblems := validatePolygon(field, zone)
		problems = append(problems, zoneProblems...)
		if len(zoneProblems) > 0 || len(in.SegmentPolygon.Points) < 3 {
			continue
		}
		for pointIndex, p := range zone.Points {
			if !PointInPolygon(p, in.SegmentPolygon) {
				problems = append(problems, violation(fmt.Sprintf("%s.points[%d]", field, pointIndex), "outside_segment", "禁采区必须完全位于洞段边界内"))
				break
			}
		}
		for zoneEdge := range zone.Points {
			for segmentEdge := range in.SegmentPolygon.Points {
				if SegmentIntersection(zone.Points[zoneEdge], zone.Points[(zoneEdge+1)%len(zone.Points)], in.SegmentPolygon.Points[segmentEdge], in.SegmentPolygon.Points[(segmentEdge+1)%len(in.SegmentPolygon.Points)]) {
					problems = append(problems, violation(fmt.Sprintf("%s.edges[%d]", field, zoneEdge), "crosses_segment_boundary", fmt.Sprintf("禁采区边界与洞段边界边 %d 相交", segmentEdge)))
					break
				}
			}
		}
	}
	for i := 0; i < len(in.ExclusionZones); i++ {
		for j := i + 1; j < len(in.ExclusionZones); j++ {
			if len(in.ExclusionZones[i].Points) >= 3 && len(in.ExclusionZones[j].Points) >= 3 && PolygonsOverlap(in.ExclusionZones[i], in.ExclusionZones[j]) {
				problems = append(problems, violation(fmt.Sprintf("exclusion_zones[%d]", j), "overlaps_exclusion_zone", fmt.Sprintf("禁采区与 exclusion_zones[%d] 重叠", i)))
			}
		}
	}
	if len(in.ReferencePoints) == 0 {
		problems = append(problems, violation("reference_points", "required", "至少登记一个基准点"))
	}
	for i, point := range in.ReferencePoints {
		field := fmt.Sprintf("reference_points[%d]", i)
		if !IsFinitePoint(point) {
			problems = append(problems, violation(field, "non_finite_coordinate", "基准点坐标必须为有限数值"))
			continue
		}
		if !PointInPolygon(point, in.SegmentPolygon) {
			problems = append(problems, violation(field, "outside_segment", "基准点必须位于洞段内"))
		}
		for j := 0; j < i; j++ {
			if pointsEqual(point, in.ReferencePoints[j]) {
				problems = append(problems, violation(field, "duplicate_reference_point", fmt.Sprintf("基准点与 reference_points[%d] 重复", j)))
			}
		}
		for zoneIndex, zone := range in.ExclusionZones {
			if PointInPolygon(point, zone) {
				problems = append(problems, violation(field, "inside_exclusion_zone", fmt.Sprintf("基准点位于 exclusion_zones[%d] 内", zoneIndex)))
				break
			}
		}
	}
	if !isFinite(in.BufferDistanceMM) || in.BufferDistanceMM < 0 {
		problems = append(problems, violation("buffer_distance_mm", "invalid_limit", "缓冲距离不能为负数"))
	}
	if !isFinite(in.MinimumSpacingMM) || in.MinimumSpacingMM <= 0 {
		problems = append(problems, violation("minimum_spacing_mm", "invalid_limit", "最小孔距必须大于零"))
	}
	if !isFinite(in.MaxCoreVolumeMM3) || !isFinite(in.MaxTotalVolumeMM3) || in.MaxCoreVolumeMM3 <= 0 || in.MaxTotalVolumeMM3 <= 0 {
		problems = append(problems, violation("volume_limits", "invalid_limit", "取样限额必须大于零"))
	}
	if in.MaxTotalVolumeMM3 < in.MaxCoreVolumeMM3 {
		problems = append(problems, violation("max_total_volume_mm3", "invalid_limit", "累计限额不能小于单孔限额"))
	}
	if len(problems) == 0 && !HasSamplingArea(in.SegmentPolygon, in.ExclusionZones, in.BufferDistanceMM) {
		problems = append(problems, violation("buffer_distance_mm", "no_sampling_area", "缓冲距离与保护范围组合使洞段内不存在可采区域"))
	}
	if len(problems) > 0 {
		return &ValidationError{Violations: problems}
	}
	a.Baseline = &ProtectionBaseline{ID: in.ID, ApplicationID: a.ID, CoordinateSystem: strings.TrimSpace(in.CoordinateSystem), SegmentPolygon: clonePolygon(in.SegmentPolygon), ExclusionZones: clonePolygons(in.ExclusionZones), ReferencePoints: append([]Point(nil), in.ReferencePoints...), BufferDistanceMM: in.BufferDistanceMM, MinimumSpacingMM: in.MinimumSpacingMM, MaxCoreVolumeMM3: in.MaxCoreVolumeMM3, MaxTotalVolumeMM3: in.MaxTotalVolumeMM3, FrozenAt: in.Now.UTC()}
	a.State = StateBaselineFrozen
	a.Touch(in.Now)
	return nil
}

func validatePolygon(field string, polygon Polygon) []Violation {
	var problems []Violation
	if len(polygon.Points) < 3 {
		return []Violation{violation(field, "invalid_polygon", "多边形至少需要三个点")}
	}
	for i, point := range polygon.Points {
		if !IsFinitePoint(point) {
			problems = append(problems, violation(fmt.Sprintf("%s.points[%d]", field, i), "non_finite_coordinate", "坐标必须为有限数值"))
		}
		next := polygon.Points[(i+1)%len(polygon.Points)]
		if pointsEqual(point, next) {
			problems = append(problems, violation(fmt.Sprintf("%s.edges[%d]", field, i), "zero_length_edge", "相邻点重复并形成零长度边"))
		}
	}
	if !isFinite(PolygonArea(polygon)) || PolygonArea(polygon) == 0 {
		problems = append(problems, violation(field, "degenerate_polygon", "多边形面积必须大于零"))
	}
	for i := range polygon.Points {
		for j := i + 1; j < len(polygon.Points); j++ {
			if j == i+1 || (i == 0 && j == len(polygon.Points)-1) {
				continue
			}
			if SegmentIntersection(polygon.Points[i], polygon.Points[(i+1)%len(polygon.Points)], polygon.Points[j], polygon.Points[(j+1)%len(polygon.Points)]) {
				problems = append(problems, violation(fmt.Sprintf("%s.edges[%d]", field, i), "self_intersection", fmt.Sprintf("边 %d 与边 %d 自相交", i, j)))
			}
		}
	}
	return problems
}
