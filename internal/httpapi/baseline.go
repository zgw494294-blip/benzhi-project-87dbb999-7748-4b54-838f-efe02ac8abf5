package httpapi

import (
	"net/http"

	"cave-sampling-permit/internal/application"
	"cave-sampling-permit/internal/domain"
)

type freezeBaselineRequest struct {
	mutationMeta
	CoordinateSystem  string           `json:"coordinate_system"`
	SegmentPolygon    polygonRequest   `json:"segment_polygon"`
	ExclusionZones    []polygonRequest `json:"exclusion_zones"`
	ReferencePoints   []pointRequest   `json:"reference_points"`
	BufferDistanceMM  float64          `json:"buffer_distance_mm"`
	MinimumSpacingMM  float64          `json:"minimum_spacing_mm"`
	MaxCoreVolumeMM3  float64          `json:"max_core_volume_mm3"`
	MaxTotalVolumeMM3 float64          `json:"max_total_volume_mm3"`
}

func (a *API) HandleFreezeBaseline(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	var body freezeBaselineRequest
	if err := decodeJSON(w, r, &body); err != nil {
		writeDecodeError(w, err)
		return
	}
	zones := make([]domain.Polygon, len(body.ExclusionZones))
	for i := range body.ExclusionZones {
		zones[i] = body.ExclusionZones[i].domain()
	}
	points := make([]domain.Point, len(body.ReferencePoints))
	for i := range body.ReferencePoints {
		points[i] = body.ReferencePoints[i].domain()
	}
	app, err := a.service.FreezeBaseline(r.Context(), application.FreezeBaselineCommand{ApplicationID: r.PathValue("application_id"), ExpectedVersion: body.ExpectedVersion, ActorID: actor, IdempotencyKey: body.IdempotencyKey, CorrelationID: correlationID(r), CoordinateSystem: body.CoordinateSystem, SegmentPolygon: body.SegmentPolygon.domain(), ExclusionZones: zones, ReferencePoints: points, BufferDistanceMM: body.BufferDistanceMM, MinimumSpacingMM: body.MinimumSpacingMM, MaxCoreVolumeMM3: body.MaxCoreVolumeMM3, MaxTotalVolumeMM3: body.MaxTotalVolumeMM3})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, app)
}
