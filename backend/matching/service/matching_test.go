package service

import (
	"context"
	"errors"
	"testing"

	"swap-chain/matching/model"

	"go.uber.org/zap"
)

type repoCall struct {
	itemID            int64
	undefinedCategory int32
	limit             int
}

type repoStub struct {
	matchesByItem map[int64][]model.ItemMatch
	errByItem     map[int64]error
	matchable     bool
	matchableErr  error
	calls         []repoCall
}

func (r *repoStub) FindSimilarItems(_ context.Context, itemID int64, undefinedCategoryID int32, limit int) ([]model.ItemMatch, error) {
	r.calls = append(r.calls, repoCall{itemID: itemID, undefinedCategory: undefinedCategoryID, limit: limit})
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
	if len(repo.calls) != 1 || repo.calls[0].limit != 5 || repo.calls[0].undefinedCategory != 99 {
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

func newTestMatching(t *testing.T, repo MatchingRepo, chainLen int) *Matching {
	t.Helper()
	m, err := NewMatching(zap.NewNop(), repo, scorerStub{score: 1}, MatchingConfig{
		SimilarItemsAmount:              5,
		CompatibilityThreshold:          0.5,
		UndefinedCategoryID:             99,
		UndefinedCompatibilityThreshold: 0.8,
		ChainLen:                        chainLen,
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
