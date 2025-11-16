package port

import (
	"context"
	"time"

	"mesa-ads/internal/core/domain"
)

// AdUseCase defines the business operations exposed by the ad engine. This
// interface represents the primary port into the application domain. Mock
// implementations can be generated from this interface for testing.
type AdUseCase interface {
	// RequestAd selects a suitable creative for the provided user context,
	// records an impression and deducts CPM budget if applicable. It
	// returns nil when no creative matches the targeting or budgets are
	// exhausted. An error is returned on internal failures.
	RequestAd(ctx context.Context, user domain.UserContext) (*AdResponse, error)

	// RegisterClick records a click event by token and deducts CPC budget
	// when configured. It returns the landing URL for redirection. If the
	// token is unknown or invalid, an error is returned. Duplicate clicks
	// are treated idempotently and return the same URL without additional
	// charges.
	RegisterClick(ctx context.Context, token string) (string, error)

	// GetStats returns aggregated impressions, clicks and cost for the
	// specified campaign (optional) and time period. When campaignID is
	// nil the stats across all campaigns are returned.
	GetStats(ctx context.Context, req StatsReq) (*StatsResp, error)
}

// AdResponse represents the selected ad details returned to the client.
// It is a DTO used by the HTTP layer and does not contain domain behaviour.
type AdResponse struct {
	CreativeID int64
	Duration   int
	VideoURL   string
	ClickURL   string
}

// StatsResp contains aggregated event counts and cost for campaigns. It is
// returned by repository and usecase methods when requesting statistics.
// Impressions and Clicks count the number of respective events. Cost
// sums the cost of those events in integer currency units.
type StatsResp struct {
	Impressions int64
	Clicks      int64
	Cost        int64
}

type StatsReq struct {
	From       time.Time
	To         time.Time
	CampaignID *int64
}
