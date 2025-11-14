package domain

import "time"

// Creative represents an individual advertisement video.
type Creative struct {
	ID         int64
	CampaignID int64
	Title      string
	VideoURL   string
	LandingURL string
	Duration   int // in seconds
	Language   string
	Category   string
	Placement  string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
