package service

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	analyzemodel "swap-chain/analyze/model"
	"swap-chain/shared/db"

	"go.uber.org/zap"
)

type analysisRepoStub struct {
	item     db.GetItemForAnalysisRow
	updated  int64
	complete db.CompleteItemAnalysisParams
}

func (r *analysisRepoStub) GetItemForAnalysis(context.Context, int64) (db.GetItemForAnalysisRow, error) {
	return r.item, nil
}

func (r *analysisRepoStub) CompleteItemAnalysis(_ context.Context, arg db.CompleteItemAnalysisParams) (int64, error) {
	r.complete = arg
	return r.updated, nil
}

type scoreStub struct{ value float64 }

func (s scoreStub) EvaluateDescription(context.Context, string) (*analyzemodel.DescriptionScore, error) {
	return &analyzemodel.DescriptionScore{ParamRichness: s.value}, nil
}

type tagStub struct {
	matches []*analyzemodel.CategoryMatch
	calls   int
}

func (s *tagStub) DefineTag(context.Context, string, string) (*analyzemodel.CategoryMatch, error) {
	match := s.matches[s.calls]
	s.calls++
	return match, nil
}

type vectorStub struct{ vectors [][]float32 }

func (s *vectorStub) Vectorize(context.Context, string) ([]float32, error) {
	result := s.vectors[0]
	s.vectors = s.vectors[1:]
	return result, nil
}

func TestAnalysisCompletesItem(t *testing.T) {
	repo := &analysisRepoStub{
		item: db.GetItemForAnalysisRow{
			ID:               7,
			OfferTitle:       "Велосипед",
			OfferDescription: sql.NullString{String: "Городской велосипед", Valid: true},
			WantDescription:  sql.NullString{String: "Сноуборд", Valid: true},
		},
		updated: 1,
	}
	tagging := &tagStub{matches: []*analyzemodel.CategoryMatch{
		{CategoryID: 2, Confidence: 0.9},
		{CategoryID: 3, Confidence: 0.8},
	}}
	analysis, err := NewAnalysis(repo, scoreStub{value: 0.75}, tagging, &vectorStub{
		vectors: [][]float32{{1, 2}, {3, 4}},
	})
	if err != nil {
		t.Fatalf("NewAnalysis() error = %v", err)
	}

	if err := analysis.AnalyzeItem(context.Background(), 7); err != nil {
		t.Fatalf("AnalyzeItem() error = %v", err)
	}
	if !repo.complete.OfferCategoryID.Valid || repo.complete.OfferCategoryID.Int32 != 2 {
		t.Fatalf("offer category = %+v", repo.complete.OfferCategoryID)
	}
	if repo.complete.ParamRichness.String != "0.75" {
		t.Fatalf("ParamRichness = %q", repo.complete.ParamRichness.String)
	}
	if len(repo.complete.OfferEmbeddingLocal.Slice()) != 2 || len(repo.complete.WantEmbeddingLocal.Slice()) != 2 {
		t.Fatal("embeddings were not passed to repository")
	}
}

func TestAnalysisRequiresManualCategory(t *testing.T) {
	repo := &analysisRepoStub{
		item: db.GetItemForAnalysisRow{
			ID:               7,
			OfferTitle:       "Вещь",
			OfferDescription: sql.NullString{String: "Описание", Valid: true},
			WantDescription:  sql.NullString{String: "Другая вещь", Valid: true},
		},
		updated: 1,
	}
	manual := &analyzemodel.CategoryMatch{IsManual: true}
	analysis, err := NewAnalysis(repo, scoreStub{value: 0.5}, &tagStub{matches: []*analyzemodel.CategoryMatch{manual, manual}}, &vectorStub{
		vectors: [][]float32{{1}, {2}},
	})
	if err != nil {
		t.Fatalf("NewAnalysis() error = %v", err)
	}

	if err := analysis.AnalyzeItem(context.Background(), 7); !errors.Is(err, ErrManualCategoryRequired) {
		t.Fatalf("AnalyzeItem() error = %v, want %v", err, ErrManualCategoryRequired)
	}
	if repo.complete.OfferCategoryID.Valid || repo.complete.WantCategoryID.Valid {
		t.Fatalf("manual category should not update database, got %+v", repo.complete)
	}
}

func TestAnalysisRejectsInvalidRichness(t *testing.T) {
	repo := &analysisRepoStub{item: db.GetItemForAnalysisRow{
		ID:               7,
		OfferTitle:       "Вещь",
		OfferDescription: sql.NullString{String: "Описание", Valid: true},
		WantDescription:  sql.NullString{String: "Другая вещь", Valid: true},
	}}
	analysis, err := NewAnalysis(repo, scoreStub{value: 1.1}, &tagStub{}, &vectorStub{})
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
	if result.VisualQuality != analyzemodel.New || result.QualityScore != 0.9 {
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

func (r recoveryRepoStub) ClaimStaleAnalyzingItems(context.Context, db.ClaimStaleAnalyzingItemsParams) ([]int64, error) {
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
