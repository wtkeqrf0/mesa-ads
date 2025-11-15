package usecase

import (
	"context"
	"sync"
	"testing"

	"mesa-ads/internal/core/domain"
	"mesa-ads/internal/core/port"
	"mesa-ads/internal/core/port/mocks"

	"github.com/stretchr/testify/mock"
)

// TestAdSelection ensures the usecase picks the highest eCPM creative.
func TestAdSelection(t *testing.T) {
	repo := mocks.NewMockAdRepository(t)

	user := domain.UserContext{UserID: "u1"}
	creatives := []port.CreativeCandidate{
		{
			Creative: domain.Creative{ID: 1, Duration: 30, VideoURL: "v1", LandingURL: "l1"},
			Campaign: domain.Campaign{ID: 1, CPMBid: 1000, CPCBid: 0},
		},
		{
			Creative: domain.Creative{ID: 2, Duration: 30, VideoURL: "v2", LandingURL: "l2"},
			Campaign: domain.Campaign{ID: 2, CPMBid: 0, CPCBid: 100},
		},
	}

	// repo.GetEligibleCreatives возвращает наших кандидатов
	repo.EXPECT().
		GetEligibleCreatives(mock.Anything, user).
		Return(creatives, nil)

	// Импрессию мы не проверяем детально, просто разрешаем её создавать
	repo.EXPECT().
		CreateImpressionAndDeductBudget(
			mock.Anything,
			mock.AnythingOfType("*domain.Impression"),
			int64(1000),
		).
		Return(nil)

	svc := NewAdUseCase(repo)

	resp, err := svc.RequestAd(context.Background(), user)
	if err != nil {
		t.Fatalf("RequestAd error: %v", err)
	}
	if resp == nil {
		t.Fatalf("expected ad, got nil")
	}
	if resp.CreativeID != 1 {
		t.Fatalf("expected creative 1, got %d", resp.CreativeID)
	}
}

// TestConcurrentBudget ensures concurrent impressions decrement budget correctly without double spending.
func TestConcurrentBudget(t *testing.T) {
	repo := mocks.NewMockAdRepository(t)

	user := domain.UserContext{UserID: "u"}
	candidates := []port.CreativeCandidate{{
		Creative: domain.Creative{ID: 1, Duration: 30, VideoURL: "v", LandingURL: "l"},
		Campaign: domain.Campaign{ID: 1, CPMBid: 1000},
	}}

	// "Глобальный" бюджет и мьютекс для симуляции конкурентного доступа
	var (
		mu     sync.Mutex
		budget int64 = 100
	)

	// Всегда отдаём один и тот же список кандидатов
	repo.EXPECT().
		GetEligibleCreatives(mock.Anything, user).
		Return(candidates, nil)

	// В CreateImpressionAndDeductBudget эмулируем списание бюджета, как раньше в mockRepo
	repo.EXPECT().
		CreateImpressionAndDeductBudget(
			mock.Anything,
			mock.AnythingOfType("*domain.Impression"),
			int64(1000),
		).
		Run(func(ctx context.Context, imp *domain.Impression, cpmBid int64) {
			mu.Lock()
			defer mu.Unlock()

			var cost int64
			if cpmBid > 0 {
				cost = (cpmBid + 999) / 1000
			}
			if budget < cost {
				// бюджета нет — просто не списываем, как и раньше
				return
			}
			budget -= cost
			imp.Cost = cost
		}).
		Return(nil)

	svc := NewAdUseCase(repo)

	wg := sync.WaitGroup{}
	count := 10
	wg.Add(count)
	for i := 0; i < count; i++ {
		go func() {
			defer wg.Done()
			_, _ = svc.RequestAd(context.Background(), user)
		}()
	}
	wg.Wait()

	// each impression costs (1000+999)/1000 = 1 unit; 10 impressions -> budget 100 - 10 = 90
	if budget != 90 {
		t.Fatalf("unexpected budget after concurrency: got %d, want 90", budget)
	}
}
