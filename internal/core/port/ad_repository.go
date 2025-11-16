package port

import (
	"context"
	"errors"

	"mesa-ads/internal/core/domain"
)

var ErrInsufficientBudget = errors.New("insufficient budget")

// AdRepository defines the persistence layer for the ad engine. It is an
// outbound port in hexagonal architecture. Implementations must be
// concurrency-safe and handle budget deductions atomically.
type AdRepository interface {
	// GetEligibleCreatives returns creatives that match targeting and have
	// available budget.
	GetEligibleCreatives(ctx context.Context, user domain.UserContext) ([]CreativeCandidate, error)
	// CreateImpressionAndDeductBudget stores an impression event and decrements
	// campaign budget (CPM).
	CreateImpressionAndDeductBudget(ctx context.Context, imp domain.Impression, cpmBid int64) error
	// CreateClickAndDeductBudget stores a click event and decrements campaign
	// budget (CPC).
	CreateClickAndDeductBudget(ctx context.Context, click domain.Click, cpcBid int64) error
	// GetStats returns aggregated statistics for campaigns in a period.
	GetStats(ctx context.Context, req StatsReq) (*StatsResp, error)

	// FindImpressionByToken finds an impression by its token.
	FindImpressionByToken(ctx context.Context, token string) (*domain.Impression, error)
	// GetCreative returns a creative by id.
	GetCreative(ctx context.Context, id int64) (*domain.Creative, error)
	// GetCampaign returns a campaign by id.
	GetCampaign(ctx context.Context, id int64) (*domain.Campaign, error)
}

type CreativeCandidate struct {
	Creative domain.Creative
	Campaign domain.Campaign
	Target   domain.Targeting
	Score    float64
}
