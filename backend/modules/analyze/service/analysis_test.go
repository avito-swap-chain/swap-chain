package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"swap-chain/modules/analyze/model"

	"go.uber.org/zap"
)

type analysisRepoStub struct {
	item     model.AnalysisItem
	wishes   []model.AnalysisWish
	updated  bool
	complete model.AnalysisResult
}

func (r *analysisRepoStub) GetItemForAnalysis(context.Context, int64) (model.AnalysisItem, error) {
	return r.item, nil
}

func (r *analysisRepoStub) GetItemWishesForAnalysis(context.Context, int64) ([]model.AnalysisWish, error) {
	return r.wishes, nil
}

func (r *analysisRepoStub) CompleteItemAnalysis(_ context.Context, result model.AnalysisResult) (bool, error) {
	r.complete = result
	return r.updated, nil
}

type scoreStub struct{ value float64 }

func (s scoreStub) EvaluateDescription(context.Context, string) (*model.DescriptionScore, error) {
	return &model.DescriptionScore{ParamRichness: s.value}, nil
}

type tagStub struct {
	mu     sync.Mutex
	byText map[string]*model.CategoryMatch
	called []string
}

func (s *tagStub) DefineTag(_ context.Context, text string, _, _ []float32) (*model.CategoryMatch, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.called = append(s.called, text)
	return s.byText[text], nil
}

type vectorStub struct{}

func (vectorStub) EnrichAndVectorize(_ context.Context, text string) ([]float32, []float32, error) {
	return []float32{float32(len(text))}, []float32{1}, nil
}

type notifierStub struct{ calls int }

func (n *notifierStub) NotifyCategoryActionRequired(context.Context, int64, string, int64) {
	n.calls++
}

func TestAnalysisPreservesManualOfferCategory(t *testing.T) {
	offerCategoryID := int32(4)
	repo := &analysisRepoStub{
		item: model.AnalysisItem{
			ID:                    7,
			UserID:                9,
			AnalysisVersion:       3,
			OfferTitle:            "Велосипед",
			OfferDescription:      "Городской велосипед",
			OfferCategoryID:       &offerCategoryID,
			OfferCategoryIsManual: true,
		},
		wishes:  []model.AnalysisWish{{ID: 10, Description: "Сноуборд"}},
		updated: true,
	}
	tagging := &tagStub{byText: map[string]*model.CategoryMatch{
		"Сноуборд": {CategoryID: 4},
	}}
	notifier := &notifierStub{}
	analysis, err := NewAnalysis(repo, scoreStub{value: 0.75}, tagging, vectorStub{}, notifier, AnalysisConfig{Concurrency: 3})
	if err != nil {
		t.Fatalf("NewAnalysis() error = %v", err)
	}

	if err := analysis.AnalyzeItem(context.Background(), 7); err != nil {
		t.Fatalf("AnalyzeItem() error = %v", err)
	}
	if repo.complete.OfferCategoryID == nil || *repo.complete.OfferCategoryID != offerCategoryID {
		t.Fatalf("offer category = %v, want %d", repo.complete.OfferCategoryID, offerCategoryID)
	}
	if len(tagging.called) != 1 || tagging.called[0] != "Сноуборд" {
		t.Fatalf("category classifier calls = %v, manual offer must be skipped", tagging.called)
	}
	if repo.complete.RequiresCategoryInput || notifier.calls != 0 {
		t.Fatalf("unexpected action-required result: %+v, notifications=%d", repo.complete, notifier.calls)
	}
}

func TestAnalysisRequestsActionWhenWishCategoryIsUnresolved(t *testing.T) {
	offerCategoryID := int32(1)
	repo := &analysisRepoStub{
		item: model.AnalysisItem{
			ID:                    7,
			UserID:                9,
			AnalysisVersion:       1,
			OfferTitle:            "Телефон",
			OfferDescription:      "Смартфон",
			OfferCategoryID:       &offerCategoryID,
			OfferCategoryIsManual: true,
		},
		wishes:  []model.AnalysisWish{{ID: 10, Description: "Что-то для поездок"}},
		updated: true,
	}
	tagging := &tagStub{byText: map[string]*model.CategoryMatch{
		"Что-то для поездок": {
			RequiresInput: true,
		},
	}}
	notifier := &notifierStub{}
	analysis, err := NewAnalysis(repo, scoreStub{value: 0.5}, tagging, vectorStub{}, notifier, AnalysisConfig{Concurrency: 3})
	if err != nil {
		t.Fatalf("NewAnalysis() error = %v", err)
	}

	if err := analysis.AnalyzeItem(context.Background(), 7); err != nil {
		t.Fatalf("AnalyzeItem() error = %v", err)
	}
	if !repo.complete.RequiresCategoryInput || notifier.calls != 1 {
		t.Fatalf("action-required result = %+v, notifications=%d", repo.complete, notifier.calls)
	}
}

func TestAnalysisRejectsInvalidRichness(t *testing.T) {
	offerCategoryID := int32(1)
	repo := &analysisRepoStub{
		item: model.AnalysisItem{
			ID:                    7,
			UserID:                9,
			OfferTitle:            "Вещь",
			OfferDescription:      "Описание",
			OfferCategoryID:       &offerCategoryID,
			OfferCategoryIsManual: true,
		},
		wishes: []model.AnalysisWish{{ID: 10, Description: "Другая вещь"}},
	}
	tagging := &tagStub{byText: map[string]*model.CategoryMatch{
		"Другая вещь": {CategoryID: 2},
	}}
	analysis, err := NewAnalysis(repo, scoreStub{value: 1.1}, tagging, vectorStub{}, &notifierStub{}, AnalysisConfig{Concurrency: 3})
	if err != nil {
		t.Fatalf("NewAnalysis() error = %v", err)
	}
	if err := analysis.AnalyzeItem(context.Background(), 7); err == nil {
		t.Fatal("AnalyzeItem() error = nil, want invalid richness error")
	}
}

func TestAnalysisRejectsOfferWithoutManualCategory(t *testing.T) {
	repo := &analysisRepoStub{
		item:    model.AnalysisItem{ID: 7, OfferTitle: "Вещь", OfferDescription: "Описание"},
		wishes:  []model.AnalysisWish{{ID: 10, Description: "Другая вещь"}},
		updated: true,
	}
	tagging := &tagStub{byText: map[string]*model.CategoryMatch{}}
	analysis, err := NewAnalysis(repo, scoreStub{value: 0.5}, tagging, vectorStub{}, &notifierStub{}, AnalysisConfig{Concurrency: 3})
	if err != nil {
		t.Fatalf("NewAnalysis() error = %v", err)
	}

	err = analysis.AnalyzeItem(context.Background(), 7)
	if !errors.Is(err, model.ErrOfferCategoryRequired) {
		t.Fatalf("AnalyzeItem() error = %v, want ErrOfferCategoryRequired", err)
	}
	if len(tagging.called) != 0 {
		t.Fatalf("category classifier calls = %v, want none", tagging.called)
	}
}

type generatorStub struct {
	response string
	err      error
}

func (s generatorStub) GenerateJSON(context.Context, string) (string, error) {
	return s.response, s.err
}

func TestScoringParsesJSON(t *testing.T) {
	scoring, err := NewScoring(generatorStub{response: `{"param_richness":0.625}`})
	if err != nil {
		t.Fatalf("NewScoring() error = %v", err)
	}
	score, err := scoring.EvaluateDescription(context.Background(), "description")
	if err != nil {
		t.Fatalf("EvaluateDescription() error = %v", err)
	}
	if score.ParamRichness != 0.625 {
		t.Fatalf("ParamRichness = %v", score.ParamRichness)
	}
}

type visionStub struct{ response string }

func (s visionStub) AnalyzePhoto(context.Context, []byte, string) (string, error) {
	return s.response, nil
}

func TestVisionValidatesModelResponse(t *testing.T) {
	vision, err := NewVision(visionStub{response: `{"marketplace_description":"Новый велосипед","suggested_category":"Спорт и отдых","visual_quality":"NEW","quality_score":0.9}`})
	if err != nil {
		t.Fatalf("NewVision() error = %v", err)
	}
	result, err := vision.DescribeImage(context.Background(), []byte("image"))
	if err != nil {
		t.Fatalf("DescribeImage() error = %v", err)
	}
	if result.VisualQuality != model.New || result.QualityScore != 0.9 {
		t.Fatalf("DescribeImage() = %+v", result)
	}
}

type recoveryRepoStub struct{ ids []int64 }

func (r recoveryRepoStub) ClaimStaleAnalyzingItems(context.Context, time.Time, int32) ([]int64, error) {
	return r.ids, nil
}

type analyzerStub struct {
	ids []int64
	err error
}

func (a *analyzerStub) AnalyzeItem(_ context.Context, itemID int64) error {
	a.ids = append(a.ids, itemID)
	return a.err
}

func TestAnalysisRecoveryWorkerProcessesClaimedItems(t *testing.T) {
	analyzer := &analyzerStub{}
	worker, err := NewAnalysisRecoveryWorker(recoveryRepoStub{ids: []int64{4, 5}}, analyzer, zap.NewNop(), AnalysisRecoveryWorkerConfig{
		PollInterval: time.Second,
		StaleAfter:   time.Minute,
		BatchSize:    10,
	})
	if err != nil {
		t.Fatalf("NewAnalysisRecoveryWorker() error = %v", err)
	}
	worker.now = func() time.Time { return time.Unix(1000, 0) }
	if err := worker.recoverOnce(context.Background()); err != nil {
		t.Fatalf("recoverOnce() error = %v", err)
	}
	if len(analyzer.ids) != 2 || analyzer.ids[0] != 4 || analyzer.ids[1] != 5 {
		t.Fatalf("analyzed IDs = %v", analyzer.ids)
	}
}

func TestAnalysisRecoveryWorkerJoinsErrors(t *testing.T) {
	wantErr := errors.New("analysis failed")
	analyzer := &analyzerStub{err: wantErr}
	worker, err := NewAnalysisRecoveryWorker(recoveryRepoStub{ids: []int64{4}}, analyzer, zap.NewNop(), DefaultAnalysisRecoveryWorkerConfig())
	if err != nil {
		t.Fatalf("NewAnalysisRecoveryWorker() error = %v", err)
	}
	if err := worker.recoverOnce(context.Background()); !errors.Is(err, wantErr) {
		t.Fatalf("recoverOnce() error = %v, want wrapped %v", err, wantErr)
	}
}
