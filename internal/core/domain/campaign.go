package domain

import "time"

// Campaign represents an advertising campaign.
// Budgets are stored in integer units (e.g. cents).
type Campaign struct {
	ID                   int64
	Name                 string
	StartDate            time.Time
	EndDate              time.Time
	DailyBudget          int64
	TotalBudget          int64
	RemainingDailyBudget int64
	RemainingTotalBudget int64
	CPMBid               int64  // cost per thousand impressions
	CPCBid               int64  // cost per click
	Status               string // active, paused, ended
	CreatedAt            time.Time
	UpdatedAt            time.Time
}
