package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"swap-chain/matching/model"

	"go.uber.org/zap"
)

type repoCall struct {
	itemID int
	limit  int
}

type repoStub struct {
	matchesByItem map[int][]model.ItemMatch
	errByItem     map[int]error
	matchable     bool
	matchableErr  error
	calls         []repoCall
}

func (r *repoStub) FindSimilarItems(_ context.Context, itemID, limit int) ([]model.ItemMatch, error) {
	r.calls = append(r.calls, repoCall{itemID: itemID, limit: limit})
	if err := r.errByItem[itemID]; err != nil {
		return nil, err
	}
	return r.matchesByItem[itemID], nil
}

func (r *repoStub) IsSourceMatchable(context.Context, int) (bool, error) {
	return r.matchable, r.matchableErr
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

func TestMatchingFindCyclesReturnsErrorWhenNoCandidatesExist(t *testing.T) {
	repo := &repoStub{matchable: true, matchesByItem: make(map[int][]model.ItemMatch)}
	matching := newTestMatching(t, repo, 3)

	cycles, err := matching.FindCycles(context.Background(), 42)
	if err == nil || !strings.Contains(err.Error(), "no edges found for item 42") {
		t.Fatalf("FindCycles() error = %v", err)
	}
	if cycles != nil {
		t.Fatalf("FindCycles() cycles = %+v, want nil", cycles)
	}
	if len(repo.calls) != 1 || repo.calls[0].limit != 5 {
		t.Fatalf("repository calls = %+v", repo.calls)
	}
}

func TestMatchingFindCyclesWrapsRepositoryErrorWhenGraphIsEmpty(t *testing.T) {
	repoErr := errors.New("database unavailable")
	repo := &repoStub{matchable: true, errByItem: map[int]error{42: repoErr}}
	matching := newTestMatching(t, repo, 3)

	_, err := matching.FindCycles(context.Background(), 42)
	if !errors.Is(err, repoErr) {
		t.Fatalf("FindCycles() error = %v, want wrapped %v", err, repoErr)
	}
}

func TestMatchingFiltersCandidatesBelowCompatibilityThreshold(t *testing.T) {
	repo := &repoStub{
		matchable: true,
		matchesByItem: map[int][]model.ItemMatch{
			1: {{SourceID: 1, TargetItem: model.Item{ID: 2}, Similarity: 0.49}},
		},
	}
	matching := newTestMatching(t, repo, 3)
	matching.cfg.CompatibilityThreshold = 0.5

	_, err := matching.FindCycles(context.Background(), 1)
	if err == nil || !strings.Contains(err.Error(), "no edges found") {
		t.Fatalf("FindCycles() error = %v, want no edges", err)
	}
}

func TestMatchingThreeItemCycle(t *testing.T) {
	matching := newTestMatching(t, cycleRepo(map[int]int{1: 2, 2: 3, 3: 1}), 3)

	cycles, err := matching.FindCycles(context.Background(), 1)
	if err != nil {
		t.Fatalf("FindCycles() error = %v", err)
	}
	if len(cycles) != 1 || len(cycles[0]) != 3 {
		t.Fatalf("cycles = %+v, want one three-item cycle", cycles)
	}
}

func TestMatchingRejectsTwoItemChainLength(t *testing.T) {
	_, err := NewMatching(zap.NewNop(), cycleRepo(map[int]int{1: 2, 2: 1}), scorerStub{score: 1}, MatchingConfig{
		SimilarItemsAmount: 5,
		ChainLen:           2,
	})
	if err == nil {
		t.Fatal("NewMatching() error = nil, want error for chain length 2")
	}
}

func newTestMatching(t *testing.T, repo MatchingRepo, chainLen int) *Matching {
	t.Helper()
	m, err := NewMatching(zap.NewNop(), repo, scorerStub{score: 1}, MatchingConfig{
		SimilarItemsAmount: 5,
		ChainLen:           chainLen,
	})
	if err != nil {
		t.Fatalf("NewMatching() error = %v", err)
	}
	return m
}

func cycleRepo(edges map[int]int) *repoStub {
	matches := make(map[int][]model.ItemMatch, len(edges))
	for source, target := range edges {
		matches[source] = []model.ItemMatch{{
			SourceID:   source,
			TargetItem: model.Item{ID: target},
			Similarity: 1,
		}}
	}
	return &repoStub{matchable: true, matchesByItem: matches}
}
