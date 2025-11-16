package db

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Seed inserts demo data into the mesa-ads database.
func Seed(ctx context.Context, db *pgxpool.Pool) error {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	// create campaigns
	for i := 1; i <= 5; i++ {
		name := fmt.Sprintf("Campaign %d", i)
		start := time.Now().AddDate(0, 0, -1)
		end := time.Now().AddDate(0, 1, 0)
		dailyBudget := int64(100000) // 1000.00 units
		totalBudget := int64(500000) // 5000.00 units
		remainingDaily := dailyBudget
		remainingTotal := totalBudget
		cpmBid := int64(500) // 0.50 per thousand
		cpcBid := int64(50)  // 0.50 per click
		status := "active"
		_, err := db.Exec(ctx, `INSERT INTO campaigns
    (id, name, start_date, end_date, daily_budget, total_budget, remaining_daily_budget,
     remaining_total_budget, cpm_bid, cpc_bid, status, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,now(),now()) ON CONFLICT DO NOTHING`,
			i, name, start, end, dailyBudget, totalBudget, remainingDaily, remainingTotal, cpmBid, cpcBid, status)
		if err != nil {
			return err
		}
		// insert targeting
		targeting := map[string]interface{}{
			"languages":  []string{"ru", "en"},
			"geos":       []string{"Armenia", "Russia"},
			"categories": []string{"music", "tech", "sports"},
			"interests":  []string{"coding", "gaming"},
			"placements": []string{"pre-roll", "mid-roll"},
		}
		tgtJSON, _ := json.Marshal(targeting)
		_, err = db.Exec(ctx, `INSERT INTO campaign_targeting (campaign_id, data)
VALUES ($1, $2) ON CONFLICT DO NOTHING`, i, tgtJSON)
		if err != nil {
			return err
		}
		// create creatives for campaign
		for j := 1; j <= 10; j++ {
			crID := (i-1)*10 + j
			title := fmt.Sprintf("Creative %d for campaign %d", j, i)
			videoURL := fmt.Sprintf("https://example.com/video/%d.mp4", crID)
			landingURL := fmt.Sprintf("https://example.com/landing/%d", crID)
			duration := 30 + r.Intn(30)
			language := []string{"ru", "en"}[r.Intn(2)]
			category := []string{"music", "tech", "sports"}[r.Intn(3)]
			placement := []string{"pre-roll", "mid-roll", "post-roll"}[r.Intn(3)]
			_, err = db.Exec(ctx, `INSERT INTO creatives
(id, campaign_id, title, video_url, landing_url, duration, language, category, placement, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,now(),now()) ON CONFLICT DO NOTHING`,
				crID, i, title, videoURL, landingURL, duration, language, category, placement)
			if err != nil {
				return err
			}
		}
	}
	// generate impressions and clicks
	impCount := 1000
	clickPerImp := 10
	for i := 0; i < impCount; i++ {
		creativeID := int64(r.Intn(50) + 1)
		campaignID := (creativeID-1)/10 + 1
		token := uuid.NewString()
		userID := fmt.Sprintf("user-%d", r.Intn(100)+1)
		cost := int64(500) // approximate cost per impression (0.50)
		var impID int64
		err := db.QueryRow(ctx, `INSERT INTO impressions
(token, creative_id, campaign_id, user_id, cost, created_at)
VALUES ($1,$2,$3,$4,$5,now()) ON CONFLICT DO NOTHING RETURNING id`,
			token, creativeID, campaignID, userID, cost).Scan(&impID)
		if err != nil {
			return err
		}
		// generate clicks
		for j := 0; j < clickPerImp; j++ {
			clickToken := uuid.NewString()
			clickCost := int64(50) // 0.50 per click
			_, err = db.Exec(ctx, `INSERT INTO clicks
(token, impression_id, creative_id, campaign_id, user_id, cost, created_at)
VALUES ($1,$2,$3,$4,$5,$6,now()) ON CONFLICT DO NOTHING`,
				clickToken, impID, creativeID, campaignID, userID, clickCost)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
