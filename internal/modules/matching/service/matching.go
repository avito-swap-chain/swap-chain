package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"swap-chain/internal/db"
	"swap-chain/internal/modules/matching/model"

	"go.uber.org/zap"
)

type VectorAdapter interface {
	Vectorize(ctx context.Context, text string) ([]float64, error)
}

type MathcingRepo interface {
	FindSimilarItems(ctx context.Context, arg db.FindSimilarItemsParams) ([]db.FindSimilarItemsRow, error)
}

type Matching struct {
	logger               *zap.Logger
	adapter              VectorAdapter
	repo                 MathcingRepo
	graph                *model.InMemoryStore
	similarItemsAmount   int     // сколько похожих вещей мы берём из БД для каждого узла графа
	chainLen             int     // длина цепочки (глубина графа)
	penaltyFactor        float64 // штраф за несбалансированные цепи, например: edgesScore = [90, 90, 10]
	chainRatingThreshold float64 // граница рейтинга для цепи, все что ниже, не попадает в выдачу
}

func NewMatching(
	logger *zap.Logger,
	adapter VectorAdapter,
	repo MathcingRepo,
	graph *model.InMemoryStore,
	similarItemsAmount int,
	chainLen int,
	penaltyFactor float64,
	chainRatingThreshold float64,
) *Matching {
	return &Matching{
		logger:               logger,
		adapter:              adapter,
		repo:                 repo,
		graph:                graph,
		similarItemsAmount:   similarItemsAmount,
		chainLen:             chainLen,
		penaltyFactor:        penaltyFactor,
		chainRatingThreshold: chainRatingThreshold,
	}
}

func (m *Matching) Vectorize(ctx context.Context, text string) ([]float64, error) {
	return m.adapter.Vectorize(ctx, text)
}

type Candidate struct {
	ID               int
	OfferTitle       string
	OfferDescription string
	WantDescription  string
	Similarity       float64
	OfferVector      []float32
	WantVector       []float32
}

func (m *Matching) FindCycles(ctx context.Context, itemID int) ([][]model.Edge, error) {
	err := m.assembleGraph(ctx, itemID)
	if err != nil {
		return nil, err
	}

	chains, err := m.graph.FindCycles(m.chainLen)
	if err != nil {
		return nil, err
	}

	return m.filterChainsByScore(chains), nil
}

// findSimilarItems ищет K похожих предметов для itemID вещи.
// returns:
// - []Candidate - кандидаты на создание связи с itemID вещью
// - []int - идентификаторы кандидатов, используются для нахождения кандидатов на следующем уровне графа
// - error
func (m *Matching) findSimilarItems(ctx context.Context, itemID int) ([]Candidate, []int, error) {
	params := db.FindSimilarItemsParams{
		ID:    int64(itemID),
		Limit: int32(m.similarItemsAmount),
	}

	res, err := m.repo.FindSimilarItems(ctx, params)
	if err != nil {
		return nil, nil, fmt.Errorf("find similar items: %w", err)
	}

	candidates := make([]Candidate, 0, len(res))
	candidateIDs := make([]int, 0, len(res))
	for _, dbCandidate := range res {
		candidates = append(candidates, Candidate{
			ID:               int(dbCandidate.ID),
			OfferTitle:       dbCandidate.OfferTitle,
			OfferDescription: dbCandidate.OfferDescription.String,
			WantDescription:  dbCandidate.WantDescription.String,
			Similarity:       dbCandidate.Similarity,
			OfferVector:      dbCandidate.OfferEmbedding.Slice(),
			WantVector:       dbCandidate.WantEmbedding.Slice(),
		})

		candidateIDs = append(candidateIDs, int(dbCandidate.ID))
	}

	return candidates, candidateIDs, nil
}

func (m *Matching) calculateScore() {

}

func (m *Matching) assembleGraph(ctx context.Context, rootID int) error {
	var errs []error
	queue := make([]int, 0, 1)
	visited := make(map[int]struct{})
	edgeCounter := 0

	m.graph.AddVertex(model.Vertex{ItemID: rootID})
	queue = append(queue, rootID)
	visited[rootID] = struct{}{}

	for i := 0; i < m.chainLen-1; i++ {
		nextQueue := make([]int, 0, len(queue))

		for _, candidateID := range queue {
			candidates, _, err := m.findSimilarItems(ctx, candidateID)
			if err != nil {
				errs = append(errs, err)
			}

			for _, candidate := range candidates {
				m.graph.AddVertex(model.Vertex{ItemID: candidate.ID})

				m.graph.AddEdge(model.Edge{
					ID:       edgeCounter,
					SourceID: candidateID,
					TargetID: candidate.ID,
					Score:    candidate.Similarity,
				})
				edgeCounter++

				if _, ok := visited[candidate.ID]; !ok {
					visited[candidate.ID] = struct{}{}
					nextQueue = append(nextQueue, candidate.ID)
				}
			}
		}

		queue = nextQueue

		if len(queue) == 0 {
			break
		}
	}

	// нужно хотя бы одно ребро для возможного обмена
	joinedErrs := errors.Join(errs...)
	if m.graph.EdgesAmount() < 1 {
		return fmt.Errorf("assemble graph: %w", joinedErrs)
	} else if m.graph.EdgesAmount() > 0 && joinedErrs != nil {
		m.logger.Warn("failed chains", zap.Error(joinedErrs))
	}

	return nil
}

// filtersChainsByScore - отсекает цепочки у которых score ниже порога.
func (m *Matching) filterChainsByScore(chains [][]model.Edge) [][]model.Edge {
	filteredChains := make([][]model.Edge, 0, len(chains))

	for _, chain := range chains {
		score := m.calculateChainScore(chain)

		if score >= m.chainRatingThreshold {
			filteredChains = append(filteredChains, chain)
		}
	}

	return filteredChains
}

// calculateChainScore - рассчитывает рейтинг целой цепочки при помощи среднего скоректированного на дисперсию.
func (m *Matching) calculateChainScore(chain []model.Edge) float64 {
	if len(chain) == 0 {
		return 0
	}

	sum := 0.0
	for _, edge := range chain {
		sum += edge.Score
	}
	mean := sum / float64(len(chain))

	varianceSum := 0.0
	for _, s := range chain {
		edgeScore := s.Score

		diff := edgeScore - mean
		varianceSum += diff * diff // math.Pow() тяжелый для обычного возведения во 2-ую степень
	}
	variance := varianceSum / float64(len(chain))
	stdDev := math.Sqrt(variance)

	return mean - (m.penaltyFactor * stdDev)
}
