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
	updated  bool
	complete model.AnalysisResult
}

func (r *analysisRepoStub) GetItemForAnalysis(context.Context, int64) (model.AnalysisItem, error) {
	return r.item, nil
}

func (r *analysisRepoStub) CompleteItemAnalysis(_ context.Context, arg model.AnalysisResult) (bool, error) {
	r.complete = arg
	return r.updated, nil
}

type scoreStub struct{ value float64 }

func (s scoreStub) EvaluateDescription(context.Context, string) (*model.DescriptionScore, error) {
	return &model.DescriptionScore{ParamRichness: s.value}, nil
}

type tagStub struct {
	mu          sync.Mutex
	matches     []*model.CategoryMatch
	byEmbedding map[float32]*model.CategoryMatch
	calls       int
}

func (s *tagStub) DefineTag(_ context.Context, embedding []float32) (*model.CategoryMatch, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(embedding) > 0 && s.byEmbedding != nil {
		return s.byEmbedding[embedding[0]], nil
	}

	match := s.matches[s.calls]
	s.calls++
	return match, nil
}

type vectorStub struct {
	mu      sync.Mutex
	vectors [][]float32
	byText  map[string][]float32
}

func (s *vectorStub) EnrichAndVectorize(_ context.Context, text string) ([]float32, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.byText != nil {
		return s.byText[text], nil
	}

	result := s.vectors[0]
	s.vectors = s.vectors[1:]
	return result, nil
}

func TestAnalysisCompletesItem(t *testing.T) {
	repo := &analysisRepoStub{
		item: model.AnalysisItem{
			ID:               7,
			AnalysisVersion:  3,
			OfferTitle:       "Велосипед",
			OfferDescription: "Городской велосипед",
			WantDescription:  "Сноуборд",
		},
		updated: true,
	}
	tagging := &tagStub{byEmbedding: map[float32]*model.CategoryMatch{
		1: {CategoryID: 2, Confidence: 0.9},
		3: {CategoryID: 3, Confidence: 0.8},
	}}
	analysis, err := NewAnalysis(repo, scoreStub{value: 0.75}, tagging, &vectorStub{
		byText: map[string][]float32{
			"Велосипед Городской велосипед": {1, 2},
			"Сноуборд": {3, 4},
		},
	})
	if err != nil {
		t.Fatalf("NewAnalysis() error = %v", err)
	}

	if err := analysis.AnalyzeItem(context.Background(), 7); err != nil {
		t.Fatalf("AnalyzeItem() error = %v", err)
	}
	if repo.complete.OfferCategoryID != 2 {
		t.Fatalf("offer category = %+v", repo.complete.OfferCategoryID)
	}
	if repo.complete.ParamRichness != 0.75 {
		t.Fatalf("ParamRichness = %v", repo.complete.ParamRichness)
	}
	if repo.complete.AnalysisVersion != 3 {
		t.Fatalf("analysis version = %d, want 3", repo.complete.AnalysisVersion)
	}
	if len(repo.complete.OfferEmbedding) != 2 || len(repo.complete.WantEmbedding) != 2 {
		t.Fatal("embeddings were not passed to repository")
	}
}

func TestAnalysisStoresUndefinedCategoryForManualDecision(t *testing.T) {
	repo := &analysisRepoStub{
		item: model.AnalysisItem{
			ID:               7,
			OfferTitle:       "Вещь",
			OfferDescription: "Описание",
			WantDescription:  "Другая вещь",
		},
		updated: true,
	}
	manual := &model.CategoryMatch{CategoryID: 99, IsManual: true}
	analysis, err := NewAnalysis(repo, scoreStub{value: 0.5}, &tagStub{matches: []*model.CategoryMatch{manual, manual}}, &vectorStub{
		vectors: [][]float32{{1}, {2}},
	})
	if err != nil {
		t.Fatalf("NewAnalysis() error = %v", err)
	}

	if err := analysis.AnalyzeItem(context.Background(), 7); err != nil {
		t.Fatalf("AnalyzeItem() error = %v", err)
	}
	if repo.complete.OfferCategoryID != 99 || repo.complete.WantCategoryID != 99 || !repo.complete.IsCategoryManual {
		t.Fatalf("undefined categories were not stored: %+v", repo.complete)
	}
}

func TestAnalysisRejectsInvalidRichness(t *testing.T) {
	repo := &analysisRepoStub{item: model.AnalysisItem{
		ID:               7,
		OfferTitle:       "Вещь",
		OfferDescription: "Описание",
		WantDescription:  "Другая вещь",
	}}
	analysis, err := NewAnalysis(
		repo,
		scoreStub{value: 1.1},
		&tagStub{matches: []*model.CategoryMatch{{CategoryID: 1}, {CategoryID: 2}}},
		&vectorStub{vectors: [][]float32{{1}, {2}}},
	)
	if err != nil {
		t.Fatalf("NewAnalysis() error = %v", err)
	}

	if err := analysis.AnalyzeItem(context.Background(), 7); err == nil {
		t.Fatal("AnalyzeItem() error = nil, want invalid richness error")
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
	vision, err := NewVision(visionStub{response: `{"marketplace_description":"Новый велосипед","visual_quality":"NEW","quality_score":0.9}`})
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

func TestVisionRejectsOutOfRangeScore(t *testing.T) {
	vision, err := NewVision(visionStub{response: `{"marketplace_description":"Вещь","visual_quality":"GOOD","quality_score":1.5}`})
	if err != nil {
		t.Fatalf("NewVision() error = %v", err)
	}
	if _, err := vision.DescribeImage(context.Background(), []byte("image")); err == nil {
		t.Fatal("DescribeImage() error = nil, want range error")
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
