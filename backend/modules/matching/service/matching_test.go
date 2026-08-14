package service

import (
	"context"
	"errors"
	"math"
	"testing"

	"swap-chain/modules/matching/model"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestNewMatchingValidatesDependenciesAndConfig(t *testing.T) {
	valid := MatchingConfig{
		SimilarItemsAmount:     5,
		CompatibilityThreshold: 0.5,
		ChainLen:               3,
		PenaltyFactor:          0.2,
		ChainRatingThreshold:   0.4,
	}
	repository := &repoStub{matchable: true}
	scorer := scorerStub{score: 1}
	tests := []struct {
		name   string
		logger *zap.Logger
		repo   MatchingRepo
		scorer Scorer
		cfg    MatchingConfig
	}{
		{name: "nil logger", repo: repository, scorer: scorer, cfg: valid},
		{name: "nil repository", logger: zap.NewNop(), scorer: scorer, cfg: valid},
		{name: "nil scorer", logger: zap.NewNop(), repo: repository, cfg: valid},
		{name: "invalid chain length", logger: zap.NewNop(), repo: repository, scorer: scorer, cfg: func() MatchingConfig { c := valid; c.ChainLen = 4; return c }()},
		{name: "invalid amount", logger: zap.NewNop(), repo: repository, scorer: scorer, cfg: func() MatchingConfig { c := valid; c.SimilarItemsAmount = 0; return c }()},
		{name: "invalid compatibility", logger: zap.NewNop(), repo: repository, scorer: scorer, cfg: func() MatchingConfig { c := valid; c.CompatibilityThreshold = math.NaN(); return c }()},
		{name: "invalid rating", logger: zap.NewNop(), repo: repository, scorer: scorer, cfg: func() MatchingConfig { c := valid; c.ChainRatingThreshold = 2; return c }()},
		{name: "invalid penalty", logger: zap.NewNop(), repo: repository, scorer: scorer, cfg: func() MatchingConfig { c := valid; c.PenaltyFactor = -1; return c }()},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NewMatching(test.logger, test.repo, test.scorer, test.cfg); err == nil {
				t.Fatal("NewMatching() error = nil")
			}
		})
	}
	if _, err := NewMatching(zap.NewNop(), repository, scorer, valid); err != nil {
		t.Fatalf("NewMatching(valid) error = %v", err)
	}
}

type repoCall struct {
	itemID int64
	limit  int
}

type repoStub struct {
	matchesByItem map[int64][]model.ItemMatch
	errByItem     map[int64]error
	matchable     bool
	matchableErr  error
	calls         []repoCall
}

func (r *repoStub) FindSimilarItems(_ context.Context, itemID int64, limit int) ([]model.ItemMatch, error) {
	r.calls = append(r.calls, repoCall{itemID: itemID, limit: limit})
	if err := r.errByItem[itemID]; err != nil {
		return nil, err
	}
	return r.matchesByItem[itemID], nil
}

func (r *repoStub) ValidateSourceItem(context.Context, int64) error {
	if r.matchableErr != nil {
		return r.matchableErr
	}
	if !r.matchable {
		return model.ErrItemNotMatchable
	}

	return nil
}

type scorerStub struct{ score float64 }

func (s scorerStub) CalculateScore(model.ItemMatch) float64 { return s.score }

func TestMatchingFindCyclesRejectsNonMatchingSource(t *testing.T) {
	repo := &repoStub{}
	matching := newTestMatching(t, repo, 3)

	_, err := matching.FindCycles(context.Background(), 42)
	if !errors.Is(err, model.ErrItemNotMatchable) {
		t.Fatalf("FindCycles() error = %v, want %v", err, model.ErrItemNotMatchable)
	}
}

func TestMatchingFindCyclesReturnsEmptyResultWhenNoCandidatesExist(t *testing.T) {
	repo := &repoStub{matchable: true, matchesByItem: make(map[int64][]model.ItemMatch)}
	matching := newTestMatching(t, repo, 3)

	cycles, err := matching.FindCycles(context.Background(), 42)
	if err != nil {
		t.Fatalf("FindCycles() error = %v", err)
	}
	if len(cycles) != 0 {
		t.Fatalf("FindCycles() cycles = %+v, want empty", cycles)
	}
	if len(repo.calls) != 1 || repo.calls[0].limit != 5 {
		t.Fatalf("repository calls = %+v", repo.calls)
	}
}

func TestMatchingFindCyclesWrapsRepositoryErrorWhenGraphIsEmpty(t *testing.T) {
	repoErr := errors.New("database unavailable")
	repo := &repoStub{matchable: true, errByItem: map[int64]error{42: repoErr}}
	matching := newTestMatching(t, repo, 3)

	_, err := matching.FindCycles(context.Background(), 42)
	if !errors.Is(err, repoErr) {
		t.Fatalf("FindCycles() error = %v, want wrapped %v", err, repoErr)
	}
}

func TestMatchingFiltersCandidatesBelowCompatibilityThreshold(t *testing.T) {
	repo := &repoStub{
		matchable: true,
		matchesByItem: map[int64][]model.ItemMatch{
			1: {{SourceID: 1, TargetItem: model.Item{ID: 2}, Similarity: 0.49}},
		},
	}
	matching := newTestMatching(t, repo, 3)
	matching.cfg.CompatibilityThreshold = 0.5

	cycles, err := matching.FindCycles(context.Background(), 1)
	if err != nil {
		t.Fatalf("FindCycles() error = %v", err)
	}
	if len(cycles) != 0 {
		t.Fatalf("FindCycles() cycles = %+v, want empty", cycles)
	}
}

func TestMatchingThreeItemCycle(t *testing.T) {
	matching := newTestMatching(t, cycleRepo(map[int64]int64{1: 2, 2: 3, 3: 1}), 3)

	cycles, err := matching.FindCycles(context.Background(), 1)
	if err != nil {
		t.Fatalf("FindCycles() error = %v", err)
	}
	if len(cycles) != 1 || len(cycles[0]) != 3 {
		t.Fatalf("cycles = %+v, want one three-item cycle", cycles)
	}
}

func TestMatchingAcceptsTwoItemChainLength(t *testing.T) {
	matching := newTestMatching(t, cycleRepo(map[int64]int64{1: 2, 2: 1}), 2)

	cycles, err := matching.FindCycles(context.Background(), 1)
	if err != nil {
		t.Fatalf("FindCycles() error = %v", err)
	}
	if len(cycles) != 1 || len(cycles[0]) != 2 {
		t.Fatalf("cycles = %+v, want one two-item cycle", cycles)
	}
}

func TestMatchingDebugLogsCandidatesAndCycles(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	matching, err := NewMatching(zap.New(core), cycleRepo(map[int64]int64{1: 2, 2: 1}), scorerStub{score: 1}, MatchingConfig{
		SimilarItemsAmount:     5,
		CompatibilityThreshold: 0.5,
		ChainLen:               2,
		Debug:                  true,
	})
	if err != nil {
		t.Fatalf("NewMatching() error = %v", err)
	}

	if _, err := matching.FindCycles(context.Background(), 1); err != nil {
		t.Fatalf("FindCycles() error = %v", err)
	}
	for _, message := range []string{
		"matching debug: candidate evaluated",
		"matching debug: raw cycles found",
		"matching debug: cycle search completed",
	} {
		if logs.FilterMessage(message).Len() == 0 {
			t.Fatalf("debug log %q was not emitted", message)
		}
	}
}

func TestApplyElbowMethodCutsSharpSimilarityDrop(t *testing.T) {
	matches := []model.ItemMatch{
		{Similarity: 0.95},
		{Similarity: 0.93},
		{Similarity: 0.90},
		{Similarity: 0.60},
	}
	got := applyElbowMethod(matches, 0.5)
	if len(got) != 3 {
		t.Fatalf("applyElbowMethod() length = %d, want 3", len(got))
	}
	if got := applyElbowMethod(nil, 0.5); got != nil {
		t.Fatalf("applyElbowMethod(nil) = %#v", got)
	}
}

func newTestMatching(t *testing.T, repo MatchingRepo, chainLen int) *Matching {
	t.Helper()
	m, err := NewMatching(zap.NewNop(), repo, scorerStub{score: 1}, MatchingConfig{
		SimilarItemsAmount:     5,
		CompatibilityThreshold: 0.5,
		ChainLen:               chainLen,
	})
	if err != nil {
		t.Fatalf("NewMatching() error = %v", err)
	}
	return m
}

func cycleRepo(edges map[int64]int64) *repoStub {
	matches := make(map[int64][]model.ItemMatch, len(edges))
	for source, target := range edges {
		matches[source] = []model.ItemMatch{{
			SourceID:   source,
			TargetItem: model.Item{ID: target},
			Similarity: 1,
		}}
	}
	return &repoStub{matchable: true, matchesByItem: matches}
}
