package domain

import (
	"time"
)

// Impression is a record of an ad being shown.
type Impression struct {
	ID         int64
	Token      string
	CreativeID int64
	CampaignID int64
	UserID     string
	Cost       int64
	CreatedAt  time.Time
}

// Click is a record of a click event.
type Click struct {
	ID           int64
	Token        string
	ImpressionID *int64
	CreativeID   int64
	CampaignID   int64
	UserID       string
	Cost         int64
	CreatedAt    time.Time
}
