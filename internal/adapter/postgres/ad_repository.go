package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mesa-ads/internal/core/domain"
	"mesa-ads/internal/core/port"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AdRepository implements port.AdRepository using pgxpool for PostgreSQL.
type AdRepository struct {
	pool *pgxpool.Pool
}

// NewAdRepository returns a new repository instance.
func NewAdRepository(pool *pgxpool.Pool) *AdRepository {
	return &AdRepository{pool: pool}
}

// GetEligibleCreatives returns creatives matching the user context.
func (r *AdRepository) GetEligibleCreatives(ctx context.Context, user domain.UserContext) ([]port.CreativeCandidate, error) {
	query := `
        SELECT
            c.id,
            c.name,
            c.start_date,
            c.end_date,
            c.daily_budget,
            c.total_budget,
            c.remaining_daily_budget,
            c.remaining_total_budget,
            c.cpm_bid,
            c.cpc_bid,
            c.status,
            c.created_at,
            c.updated_at,
            cr.id,
            cr.campaign_id,
            cr.title,
            cr.video_url,
            cr.landing_url,
            cr.duration,
            cr.language,
            cr.category,
            cr.placement,
            cr.created_at,
            cr.updated_at,
            t.data
        FROM creatives cr
        JOIN campaigns c ON cr.campaign_id = c.id
        JOIN campaign_targeting t ON t.campaign_id = c.id
        WHERE c.status = 'active'
          AND now() BETWEEN c.start_date AND c.end_date
          AND c.remaining_daily_budget > 0 AND c.remaining_total_budget > 0`
	// Execute the query
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	// Use CollectRows to scan each row into a temporary struct.
	type rawCandidate struct {
		Camp         domain.Campaign
		Cr           domain.Creative
		TargetingRaw []byte
	}
	raw, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (rawCandidate, error) {
		var rc rawCandidate
		// Scan campaign fields
		err = row.Scan(
			&rc.Camp.ID,
			&rc.Camp.Name,
			&rc.Camp.StartDate,
			&rc.Camp.EndDate,
			&rc.Camp.DailyBudget,
			&rc.Camp.TotalBudget,
			&rc.Camp.RemainingDailyBudget,
			&rc.Camp.RemainingTotalBudget,
			&rc.Camp.CPMBid,
			&rc.Camp.CPCBid,
			&rc.Camp.Status,
			&rc.Camp.CreatedAt,
			&rc.Camp.UpdatedAt,
			&rc.Cr.ID,
			&rc.Cr.CampaignID,
			&rc.Cr.Title,
			&rc.Cr.VideoURL,
			&rc.Cr.LandingURL,
			&rc.Cr.Duration,
			&rc.Cr.Language,
			&rc.Cr.Category,
			&rc.Cr.Placement,
			&rc.Cr.CreatedAt,
			&rc.Cr.UpdatedAt,
			&rc.TargetingRaw,
		)
		return rc, err
	})
	if err != nil {
		return nil, err
	}
	// Filter and convert raw results into CreativeCandidate
	candidates := make([]port.CreativeCandidate, 0, len(raw))
	for _, rc := range raw {
		var tgt domain.Targeting
		if err = json.Unmarshal(rc.TargetingRaw, &tgt); err != nil {
			// skip malformed targeting
			continue
		}
		// apply targeting filters using slices.Contains from golang.org/x/exp/slices.
		if len(tgt.Languages) > 0 && !slices.Contains(tgt.Languages, user.Language) {
			continue
		}
		if len(tgt.Geos) > 0 && !slices.Contains(tgt.Geos, user.Geo) {
			continue
		}
		if len(tgt.Categories) > 0 && !slices.Contains(tgt.Categories, user.Category) {
			continue
		}
		if len(tgt.Placements) > 0 && !slices.Contains(tgt.Placements, user.Placement) {
			continue
		}
		if len(tgt.Interests) > 0 {
			match := false
			for _, v := range tgt.Interests {
				if slices.Contains(user.Interests, v) {
					match = true
					break
				}
			}
			if !match {
				continue
			}
		}
		candidates = append(candidates, port.CreativeCandidate{
			Creative: rc.Cr,
			Campaign: rc.Camp,
			Target:   tgt,
		})
	}
	return candidates, nil
}

// CreateImpressionAndDeductBudget inserts impression and deducts budget for CPM campaigns.
func (r *AdRepository) CreateImpressionAndDeductBudget(ctx context.Context, imp *domain.Impression, cpmBid int64) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		} else {
			_ = tx.Commit(ctx)
		}
	}()
	// lock campaign
	var remainingDaily, remainingTotal int64
	err = tx.QueryRow(ctx, `SELECT remaining_daily_budget, remaining_total_budget FROM campaigns WHERE id = $1 FOR UPDATE`, imp.CampaignID).Scan(&remainingDaily, &remainingTotal)
	if err != nil {
		return err
	}
	// compute cost per impression
	cost := int64(0)
	if cpmBid > 0 {
		cost = (cpmBid + 999) / 1000
	}
	if cost > 0 && (remainingDaily < cost || remainingTotal < cost) {
		return errors.New("insufficient budget")
	}
	if cost > 0 {
		_, err = tx.Exec(ctx, `UPDATE campaigns SET remaining_daily_budget = remaining_daily_budget - $1, remaining_total_budget = remaining_total_budget - $1 WHERE id = $2`, cost, imp.CampaignID)
		if err != nil {
			return err
		}
	}
	imp.Cost = cost
	// insert impression with explicit created_at
	imp.CreatedAt = time.Now().UTC()
	_, err = tx.Exec(ctx, `INSERT INTO impressions (token, creative_id, campaign_id, user_id, cost, created_at) VALUES ($1,$2,$3,$4,$5,$6)`, imp.Token, imp.CreativeID, imp.CampaignID, imp.UserID, imp.Cost, imp.CreatedAt)
	return err
}

// CreateClickAndDeductBudget inserts click event and deducts budget for CPC campaigns.
func (r *AdRepository) CreateClickAndDeductBudget(ctx context.Context, click *domain.Click, cpcBid int64) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		} else {
			_ = tx.Commit(ctx)
		}
	}()
	// lock campaign
	var remainingDaily, remainingTotal int64
	err = tx.QueryRow(ctx, `SELECT remaining_daily_budget, remaining_total_budget FROM campaigns WHERE id = $1 FOR UPDATE`, click.CampaignID).Scan(&remainingDaily, &remainingTotal)
	if err != nil {
		return err
	}
	cost := cpcBid
	if cost > 0 && (remainingDaily < cost || remainingTotal < cost) {
		return errors.New("insufficient budget")
	}
	if cost > 0 {
		_, err = tx.Exec(ctx, `UPDATE campaigns SET remaining_daily_budget = remaining_daily_budget - $1, remaining_total_budget = remaining_total_budget - $1 WHERE id = $2`, cost, click.CampaignID)
		if err != nil {
			return err
		}
	}
	click.Cost = cost
	// set created_at explicitly
	click.CreatedAt = time.Now().UTC()
	_, err = tx.Exec(ctx, `INSERT INTO clicks (token, impression_id, creative_id, campaign_id, user_id, cost, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7)`, click.Token, click.ImpressionID, click.CreativeID, click.CampaignID, click.UserID, click.Cost, click.CreatedAt)
	return err
}

// GetStats returns aggregated events for campaigns.
func (r *AdRepository) GetStats(ctx context.Context, req port.StatsReq) (*port.StatsResp, error) {
	args := []interface{}{req.From, req.To}
	whereCampaign := ""
	if req.CampaignID != nil {
		whereCampaign = "AND campaign_id = $3"
		args = append(args, *req.CampaignID)
	}
	impQuery := fmt.Sprintf(`SELECT COALESCE(count(*),0), COALESCE(sum(cost),0) FROM impressions WHERE created_at >= $1 AND created_at <= $2 %s`, whereCampaign)
	var impCount, impCost int64
	err := r.pool.QueryRow(ctx, impQuery, args...).Scan(&impCount, &impCost)
	if err != nil {
		return nil, err
	}
	clickQuery := fmt.Sprintf(`SELECT COALESCE(count(*),0), COALESCE(sum(cost),0) FROM clicks WHERE created_at >= $1 AND created_at <= $2 %s`, whereCampaign)
	var clickCount, clickCost int64
	err = r.pool.QueryRow(ctx, clickQuery, args...).Scan(&clickCount, &clickCost)
	if err != nil {
		return nil, err
	}
	return &port.StatsResp{
		Impressions: impCount,
		Clicks:      clickCount,
		Cost:        impCost + clickCost,
	}, nil
}

// FindImpressionByToken returns impression by token.
func (r *AdRepository) FindImpressionByToken(ctx context.Context, token string) (*domain.Impression, error) {
	var imp domain.Impression
	err := r.pool.QueryRow(ctx, `SELECT id, token, creative_id, campaign_id, user_id, cost, created_at FROM impressions WHERE token = $1`, token).Scan(&imp.ID, &imp.Token, &imp.CreativeID, &imp.CampaignID, &imp.UserID, &imp.Cost, &imp.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &imp, nil
}

// GetCreative returns a creative by id.
func (r *AdRepository) GetCreative(ctx context.Context, id int64) (*domain.Creative, error) {
	var cr domain.Creative
	err := r.pool.QueryRow(ctx, `SELECT id, campaign_id, title, video_url, landing_url, duration, language, category, placement, created_at, updated_at FROM creatives WHERE id = $1`, id).
		Scan(&cr.ID, &cr.CampaignID, &cr.Title, &cr.VideoURL, &cr.LandingURL, &cr.Duration, &cr.Language, &cr.Category, &cr.Placement, &cr.CreatedAt, &cr.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &cr, nil
}

// GetCampaign returns a campaign by id.
func (r *AdRepository) GetCampaign(ctx context.Context, id int64) (*domain.Campaign, error) {
	var c domain.Campaign
	err := r.pool.QueryRow(ctx, `SELECT id, name, start_date, end_date, daily_budget, total_budget, remaining_daily_budget, remaining_total_budget, cpm_bid, cpc_bid, status, created_at, updated_at FROM campaigns WHERE id = $1`, id).
		Scan(&c.ID, &c.Name, &c.StartDate, &c.EndDate, &c.DailyBudget, &c.TotalBudget, &c.RemainingDailyBudget, &c.RemainingTotalBudget, &c.CPMBid, &c.CPCBid, &c.Status, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}
