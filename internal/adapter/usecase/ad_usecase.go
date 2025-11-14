package usecase

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"mesa-ads/internal/core/domain"
	"mesa-ads/internal/core/port"
)

// AdUseCase provides business logic for ad selection and event processing.
// It orchestrates domain and repositories to implement the AdUseCase interface.
type AdUseCase struct {
	repo port.AdRepository

	// defaultCTR is the estimated click‑through rate used for eCPM
	// calculations when no prior data exists. It is expressed as a
	// fraction in the range [0,1].
	defaultCTR float64
}

// NewAdUseCase creates a new usecase with the provided repository. The
// defaultCTR is set to a reasonable small value.
func NewAdUseCase(repo port.AdRepository) *AdUseCase {
	return &AdUseCase{repo: repo, defaultCTR: 0.01}
}

// RequestAd selects a suitable ad for the given user context, creates an
// impression and deducts CPM budget. It returns nil when no creative
// matches the targeting or budgets are exhausted. An error is returned on
// repository failures.
func (u *AdUseCase) RequestAd(ctx context.Context, user domain.UserContext) (*port.AdResponse, error) {
	candidates, err := u.repo.GetEligibleCreatives(ctx, user)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return nil, nil
	}
	// compute scores and choose best candidate
	bestIndex := -1
	bestScore := float64(-1)
	for i := range candidates {
		score := u.computeScore(&candidates[i].Campaign)
		candidates[i].Score = score
		if score > bestScore {
			bestScore = score
			bestIndex = i
		}
	}
	if bestIndex < 0 {
		return nil, nil
	}
	chosen := candidates[bestIndex]
	// generate unique token for impression
	token := uuid.NewString()
	imp := &domain.Impression{
		Token:      token,
		CreativeID: chosen.Creative.ID,
		CampaignID: chosen.Campaign.ID,
		UserID:     user.UserID,
	}
	cpmBid := chosen.Campaign.CPMBid
	if err = u.repo.CreateImpressionAndDeductBudget(ctx, imp, cpmBid); err != nil {
		return nil, err
	}
	clickURL := fmt.Sprintf("/ad/click/%s", token)
	return &port.AdResponse{
		CreativeID: chosen.Creative.ID,
		Duration:   chosen.Creative.Duration,
		VideoURL:   chosen.Creative.VideoURL,
		ClickURL:   clickURL,
	}, nil
}

// RegisterClick records a click event by token and deducts CPC budget if
// applicable. It returns the landing URL for redirection. Duplicate
// inserts are treated as idempotent and return the same URL without
// additional charges.
func (u *AdUseCase) RegisterClick(ctx context.Context, token string) (string, error) {
	if token == "" {
		return "", errors.New("empty token")
	}
	imp, err := u.repo.FindImpressionByToken(ctx, token)
	if err != nil {
		return "", err
	}
	if imp == nil {
		return "", errors.New("impression not found")
	}
	cr, err := u.repo.GetCreative(ctx, imp.CreativeID)
	if err != nil {
		return "", err
	}
	if cr == nil {
		return "", errors.New("creative not found")
	}
	camp, err := u.repo.GetCampaign(ctx, imp.CampaignID)
	if err != nil {
		return "", err
	}
	if camp == nil {
		return "", errors.New("campaign not found")
	}

	cpcBid := camp.CPCBid
	click := &domain.Click{
		Token:        token,
		CreativeID:   imp.CreativeID,
		CampaignID:   imp.CampaignID,
		UserID:       imp.UserID,
		ImpressionID: &imp.ID,
	}

	if err = u.repo.CreateClickAndDeductBudget(ctx, click, cpcBid); err != nil {
		// treat duplicate insert as idempotent – return landing URL
		return cr.LandingURL, nil
	}
	return cr.LandingURL, nil
}

// GetStats returns aggregated stats for campaigns in a period.
func (u *AdUseCase) GetStats(ctx context.Context, req port.StatsReq) (*port.StatsResp, error) {
	return u.repo.GetStats(ctx, req)
}

// computeScore returns a floating score for ranking candidate. For CPM
// campaigns the score is simply the bid. For CPC campaigns the bid is
// converted into an eCPM by multiplying with an estimated CTR and 1000.
func (u *AdUseCase) computeScore(c *domain.Campaign) float64 {
	var score float64
	if c.CPMBid > 0 {
		score = float64(c.CPMBid)
	}
	if c.CPCBid > 0 {
		cpcScore := float64(c.CPCBid) * u.defaultCTR * 1000.0
		if cpcScore > score {
			score = cpcScore
		}
	}
	return score
}
