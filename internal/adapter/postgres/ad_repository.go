package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"mesa-ads/internal/core/domain"
	"mesa-ads/internal/core/port"
)

const maxImpressionsPerUserPerCreative = 3

// AdRepository implements port.AdRepository using pgxpool for PostgreSQL.
type AdRepository struct {
	pool *pgxpool.Pool
}

// NewAdRepository returns a new repository instance.
func NewAdRepository(pool *pgxpool.Pool) *AdRepository {
	return &AdRepository{pool: pool}
}

// GetEligibleCreatives returns creatives matching the user context.
func (r *AdRepository) GetEligibleCreatives(
	ctx context.Context,
	user domain.UserContext,
) ([]port.CreativeCandidate, error) {
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

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	candidates := make([]port.CreativeCandidate, 0)

	for rows.Next() {
		var (
			camp         domain.Campaign
			cr           domain.Creative
			targetingRaw []byte
		)

		if err = rows.Scan(
			&camp.ID,
			&camp.Name,
			&camp.StartDate,
			&camp.EndDate,
			&camp.DailyBudget,
			&camp.TotalBudget,
			&camp.RemainingDailyBudget,
			&camp.RemainingTotalBudget,
			&camp.CPMBid,
			&camp.CPCBid,
			&camp.Status,
			&camp.CreatedAt,
			&camp.UpdatedAt,
			&cr.ID,
			&cr.CampaignID,
			&cr.Title,
			&cr.VideoURL,
			&cr.LandingURL,
			&cr.Duration,
			&cr.Language,
			&cr.Category,
			&cr.Placement,
			&cr.CreatedAt,
			&cr.UpdatedAt,
			&targetingRaw,
		); err != nil {
			return nil, err
		}

		var tgt domain.Targeting
		if err = json.Unmarshal(targetingRaw, &tgt); err != nil {
			continue
		}

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

		// ограничиваем повторы креатива для одного пользователя
		if user.UserID != "" {
			var cnt int
			err = r.pool.QueryRow(
				ctx,
				`SELECT COUNT(*) 
                   FROM impressions 
                  WHERE creative_id = $1 
                    AND user_id = $2 
                    AND created_at > now() - INTERVAL '1 hour'`,
				cr.ID,
				user.UserID,
			).Scan(&cnt)
			if err != nil {
				return nil, err
			}

			if cnt >= maxImpressionsPerUserPerCreative {
				// пользователь уже достаточно часто видел этот ролик недавно
				continue
			}
		}

		candidates = append(candidates, port.CreativeCandidate{
			Creative: cr,
			Campaign: camp,
			Target:   tgt,
		})
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return candidates, nil
}

// CreateImpressionAndDeductBudget inserts impression and deducts budget for CPM campaigns.
func (r *AdRepository) CreateImpressionAndDeductBudget(ctx context.Context, imp domain.Impression, cpmBid int64) error {
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
	const selectQuery = `SELECT remaining_daily_budget, remaining_total_budget FROM campaigns WHERE id = $1 FOR UPDATE`

	var remainingDaily, remainingTotal int64
	err = tx.QueryRow(ctx, selectQuery, imp.CampaignID).Scan(&remainingDaily, &remainingTotal)
	if err != nil {
		return err
	}
	cost := int64(0)
	if cpmBid > 0 {
		cost = (cpmBid + 999) / 1000
	}
	if cost > 0 && (remainingDaily < cost || remainingTotal < cost) {
		return port.ErrInsufficientBudget
	}
	if cost > 0 {
		const updateQuery = `UPDATE campaigns SET
remaining_daily_budget = remaining_daily_budget - $1,
remaining_total_budget = remaining_total_budget - $1
	WHERE id = $2`
		_, err = tx.Exec(ctx, updateQuery, cost, imp.CampaignID)
		if err != nil {
			return err
		}
	}

	const insertQuery = `INSERT INTO impressions
    (token, creative_id, campaign_id, user_id, cost, created_at) VALUES ($1,$2,$3,$4,$5,$6)`

	imp.Cost = cost
	imp.CreatedAt = time.Now().UTC()
	_, err = tx.Exec(ctx, insertQuery, imp.Token, imp.CreativeID,
		imp.CampaignID, imp.UserID, imp.Cost, imp.CreatedAt)
	return err
}

// CreateClickAndDeductBudget inserts click event and deducts budget for CPC campaigns.
// Operation is idempotent by token: repeated calls with the same token do not
// create a new click and do not charge the budget again.
func (r *AdRepository) CreateClickAndDeductBudget(ctx context.Context, click domain.Click, cpcBid int64) (err error) {
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

	cost := cpcBid

	const (
		insertQuery = `
INSERT INTO clicks (token, impression_id, creative_id, campaign_id, user_id, cost, created_at)
VALUES ($1,$2,$3,$4,$5,$6,$7) ON CONFLICT (token) DO NOTHING`
		selectQuery = `SELECT remaining_daily_budget, remaining_total_budget 
FROM campaigns WHERE id = $1 FOR UPDATE`
	)

	// если ставка CPC не задана, просто записываем клик (без списания бюджета)
	if cost <= 0 {
		click.Cost = 0
		click.CreatedAt = time.Now().UTC()

		_, err = tx.Exec(ctx, insertQuery,
			click.Token,
			click.ImpressionID,
			click.CreativeID,
			click.CampaignID,
			click.UserID,
			click.Cost,
			click.CreatedAt,
		)
		return err
	}

	// 1. проверяем бюджет и блокируем строку кампании
	var remainingDaily, remainingTotal int64
	err = tx.QueryRow(
		ctx,
		selectQuery,
		click.CampaignID,
	).Scan(&remainingDaily, &remainingTotal)
	if err != nil {
		return err
	}

	if remainingDaily < cost || remainingTotal < cost {
		return port.ErrInsufficientBudget
	}

	// 2. Пытаемся вставить клик. Если дубликат токена — строка не вставится.
	click.Cost = cost
	click.CreatedAt = time.Now().UTC()

	tag, err := tx.Exec(ctx, insertQuery,
		click.Token,
		click.ImpressionID,
		click.CreativeID,
		click.CampaignID,
		click.UserID,
		click.Cost,
		click.CreatedAt,
	)
	if err != nil {
		return err
	}

	// Если строка не вставлена — это повторный клик с тем же токеном.
	// Считаем это идемпотентным вызовом: бюджет не списываем.
	if tag.RowsAffected() == 0 {
		return nil
	}

	const updateQuery = `UPDATE campaigns SET
	remaining_daily_budget = remaining_daily_budget - $1,
	remaining_total_budget = remaining_total_budget - $1
WHERE id = $2`

	// 3. Списываем бюджет ровно один раз для нового клика.
	_, err = tx.Exec(ctx, updateQuery,
		cost,
		click.CampaignID,
	)
	return err
}

// GetStats returns aggregated events for campaigns.
func (r *AdRepository) GetStats(ctx context.Context, req port.StatsReq) (*port.StatsResp, error) {
	args := []interface{}{req.From, req.To}
	whereClause := "WHERE created_at >= $1 AND created_at <= $2"

	if req.CampaignID != nil {
		whereClause += " AND campaign_id = $3"
		args = append(args, *req.CampaignID)
	}

	impQuery := `SELECT COALESCE(count(*),0), COALESCE(sum(cost),0) FROM impressions ` + whereClause
	var impCount, impCost int64
	err := r.pool.QueryRow(ctx, impQuery, args...).Scan(&impCount, &impCost)
	if err != nil {
		return nil, err
	}

	clickQuery := `SELECT COALESCE(count(*),0), COALESCE(sum(cost),0) FROM clicks ` + whereClause
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
	const query = `SELECT
id, token, creative_id, campaign_id, user_id, cost, created_at
FROM impressions WHERE token = $1`
	var imp domain.Impression
	err := r.pool.QueryRow(ctx, query, token).Scan(
		&imp.ID, &imp.Token, &imp.CreativeID, &imp.CampaignID, &imp.UserID, &imp.Cost, &imp.CreatedAt,
	)
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
	const query = `SELECT id, campaign_id, title, video_url, landing_url,
duration, language, category, placement, created_at, updated_at FROM creatives WHERE id = $1`

	var cr domain.Creative
	err := r.pool.QueryRow(ctx, query, id).
		Scan(&cr.ID, &cr.CampaignID, &cr.Title, &cr.VideoURL, &cr.LandingURL,
			&cr.Duration, &cr.Language, &cr.Category, &cr.Placement, &cr.CreatedAt, &cr.UpdatedAt)
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
	const query = `SELECT
id, name, start_date, end_date, daily_budget, total_budget,
remaining_daily_budget, remaining_total_budget, cpm_bid,
cpc_bid, status, created_at, updated_at FROM campaigns WHERE id = $1`

	var c domain.Campaign
	err := r.pool.QueryRow(ctx, query, id).
		Scan(&c.ID, &c.Name, &c.StartDate, &c.EndDate, &c.DailyBudget,
			&c.TotalBudget, &c.RemainingDailyBudget, &c.RemainingTotalBudget,
			&c.CPMBid, &c.CPCBid, &c.Status, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}
