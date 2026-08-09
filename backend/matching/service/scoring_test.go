package service

import (
	"math"
	"testing"

	"swap-chain/matching/model"
)

func TestScoringCalculateScore(t *testing.T) {
	tests := []struct {
		name  string
		match model.ItemMatch
		want  float64
	}{
		{
			name: "zero metadata",
			match: model.ItemMatch{
				Similarity: 0,
			},
			want: 0,
		},
		{
			name: "fully described reliable item",
			match: model.ItemMatch{
				Similarity: 1,
				TargetItem: model.Item{Meta: model.ItemMetadata{
					TitleLen:       20,
					DescriptionLen: 120,
					ImageAmount:    3,
					ParamRichness:  1,
					QualityScore:   1,
					UserRating:     5,
					SuccessRate:    1,
				}},
			},
			want: 1,
		},
		{
			name: "score is capped",
			match: model.ItemMatch{
				Similarity: 2,
				TargetItem: model.Item{Meta: model.ItemMetadata{
					TitleLen:       20,
					DescriptionLen: 120,
					ImageAmount:    3,
					ParamRichness:  2,
					QualityScore:   2,
					UserRating:     10,
					SuccessRate:    2,
				}},
			},
			want: 1,
		},
	}

	scoring := NewScoring()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := scoring.CalculateScore(tt.match)
			if !almostEqual(got, tt.want) {
				t.Fatalf("score: got %.6f, want %.6f", got, tt.want)
			}
		})
	}
}

func TestMatchingCalculateChainScore(t *testing.T) {
	tests := []struct {
		name    string
		chain   []model.Edge
		penalty float64
		want    float64
	}{
		{name: "empty", chain: nil, penalty: 1, want: 0},
		{
			name:    "balanced two-edge chain",
			chain:   []model.Edge{{Score: 0.8}, {Score: 0.8}},
			penalty: 1,
			want:    0.8,
		},
		{
			name:    "length penalty",
			chain:   []model.Edge{{Score: 0.8}, {Score: 0.8}, {Score: 0.8}},
			penalty: 1,
			want:    0.72,
		},
		{
			name:    "variance penalty",
			chain:   []model.Edge{{Score: 1}, {Score: 0}},
			penalty: 1,
			want:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matching := &Matching{cfg: MatchingConfig{PenaltyFactor: tt.penalty}}
			got := matching.CalculateChainScore(tt.chain)
			if !almostEqual(got, tt.want) {
				t.Fatalf("chain score: got %.6f, want %.6f", got, tt.want)
			}
		})
	}
}

func TestMatchingFilterChainsByScoreAndRoot(t *testing.T) {
	matching := &Matching{cfg: MatchingConfig{ChainRatingThreshold: 0.7}}
	accepted := []model.Edge{
		{SourceID: 1, TargetID: 2, Score: 0.8},
		{SourceID: 2, TargetID: 1, Score: 0.8},
	}
	lowScore := []model.Edge{
		{SourceID: 1, TargetID: 3, Score: 0.6},
		{SourceID: 3, TargetID: 1, Score: 0.6},
	}
	withoutRoot := []model.Edge{
		{SourceID: 2, TargetID: 3, Score: 0.9},
		{SourceID: 3, TargetID: 2, Score: 0.9},
	}

	got := matching.filterChainsByScoreAndRoot(
		[][]model.Edge{accepted, lowScore, withoutRoot},
		1,
	)
	if len(got) != 1 {
		t.Fatalf("filtered chains amount: got %d, want 1", len(got))
	}
	if got[0][0] != accepted[0] {
		t.Fatalf("unexpected chain accepted: %+v", got[0])
	}
}

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}
