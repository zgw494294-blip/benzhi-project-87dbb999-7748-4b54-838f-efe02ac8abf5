package application

import (
	"context"

	"cave-sampling-permit/internal/domain"
)

type CheckBatchFilter struct {
	SiteID string
	Rule   domain.RuleCode
	Passed *bool
	Type   string
}

func (s *Service) ListCheckBatches(ctx context.Context, applicationID string, filter CheckBatchFilter) ([]domain.CheckBatch, error) {
	if err := validateCheckBatchFilter(filter); err != nil {
		return nil, err
	}
	app, err := s.repo.Get(ctx, applicationID)
	if err != nil {
		return nil, err
	}
	result := make([]domain.CheckBatch, 0)
	for _, batch := range app.CheckBatches {
		filtered, matches := filterBatch(batch, app.Version, filter)
		if matches {
			result = append(result, filtered)
		}
	}
	return result, nil
}

func (s *Service) GetCheckBatch(ctx context.Context, applicationID, batchID string, filter CheckBatchFilter) (*domain.CheckBatch, error) {
	if err := validateCheckBatchFilter(filter); err != nil {
		return nil, err
	}
	app, err := s.repo.Get(ctx, applicationID)
	if err != nil {
		return nil, err
	}
	for _, batch := range app.CheckBatches {
		if batch.ID == batchID {
			filtered, _ := filterBatch(batch, app.Version, filter)
			return &filtered, nil
		}
	}
	return nil, ErrNotFound
}

func validateCheckBatchFilter(filter CheckBatchFilter) error {
	if filter.Type != "" && filter.Type != "full" && filter.Type != "targeted" {
		return validation("type", "invalid_type", "检查批次类型必须为 full 或 targeted")
	}
	if filter.Rule != "" {
		allowed := map[domain.RuleCode]bool{domain.RuleBoundary: true, domain.RuleBuffer: true, domain.RuleSpacing: true, domain.RuleCoreVolume: true, domain.RuleTotal: true}
		if !allowed[filter.Rule] {
			return validation("rule", "unknown_rule", "包含未知检查规则")
		}
	}
	return nil
}

func filterBatch(batch domain.CheckBatch, currentVersion int64, filter CheckBatchFilter) (domain.CheckBatch, bool) {
	batch.HistoricalSnapshot = batch.ApplicationVersion != currentVersion
	batch.Current = batch.Current && !batch.HistoricalSnapshot
	if filter.Type != "" && batch.Type != filter.Type {
		batch.Results = []domain.CheckResult{}
		return batch, false
	}
	filtered := make([]domain.CheckResult, 0)
	for _, result := range batch.Results {
		if filter.SiteID != "" && result.SiteID != filter.SiteID {
			continue
		}
		if filter.Rule != "" && result.Rule != filter.Rule {
			continue
		}
		if filter.Passed != nil && result.Passed != *filter.Passed {
			continue
		}
		filtered = append(filtered, result)
	}
	batch.Results = filtered
	hasResultFilter := filter.SiteID != "" || filter.Rule != "" || filter.Passed != nil
	return batch, !hasResultFilter || len(filtered) > 0
}
