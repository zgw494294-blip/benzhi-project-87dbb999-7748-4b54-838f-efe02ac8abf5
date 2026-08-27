package domain

import (
	"strings"
	"time"
)

type NewApplication struct {
	ID                string
	CaveName          string
	SegmentName       string
	ResearchPurpose   string
	ApplicantID       string
	ProtectionOwnerID string
	Now               time.Time
}

func CreateApplication(in NewApplication) (*Application, error) {
	var problems []Violation
	required := []struct{ field, value string }{
		{"cave_name", in.CaveName}, {"segment_name", in.SegmentName},
		{"research_purpose", in.ResearchPurpose}, {"applicant_id", in.ApplicantID},
		{"protection_owner_id", in.ProtectionOwnerID},
	}
	for _, item := range required {
		if strings.TrimSpace(item.value) == "" {
			problems = append(problems, violation(item.field, "required", "该字段不能为空"))
		}
	}
	if strings.TrimSpace(in.ID) == "" {
		problems = append(problems, violation("application_id", "required", "申请编号不能为空"))
	}
	if in.ApplicantID == in.ProtectionOwnerID && in.ApplicantID != "" {
		problems = append(problems, violation("protection_owner_id", "participant_conflict", "保护责任人不能与申请人相同"))
	}
	if len(problems) > 0 {
		return nil, &ValidationError{Violations: problems}
	}
	now := in.Now.UTC()
	return &Application{ID: in.ID, CaveName: strings.TrimSpace(in.CaveName), SegmentName: strings.TrimSpace(in.SegmentName), ResearchPurpose: strings.TrimSpace(in.ResearchPurpose), ApplicantID: in.ApplicantID, ProtectionOwnerID: in.ProtectionOwnerID, State: StateDraft, Version: 1, CreatedAt: now, UpdatedAt: now}, nil
}

func (a *Application) RequireState(states ...ApplicationState) error {
	for _, allowed := range states {
		if a.State == allowed {
			return nil
		}
	}
	return &StateError{Current: a.State, Expected: joinStates(states)}
}

func joinStates(states []ApplicationState) string {
	parts := make([]string, len(states))
	for i := range states {
		parts[i] = string(states[i])
	}
	return strings.Join(parts, " or ")
}

func (a *Application) Touch(now time.Time) { a.UpdatedAt = now.UTC() }
