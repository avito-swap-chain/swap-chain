package service

import (
	"context"
	"errors"
	"fmt"
	"math"

	"swap-chain/matching/model"

	"go.uber.org/zap"
)

type MatchingRepo interface {
	FindSimilarItems(ctx context.Context, sourceID int, limit int) ([]model.ItemMatch, error)
	IsSourceMatchable(ctx context.Context, itemID int) (bool, error)
}

type Scorer interface {
	CalculateScore(match model.ItemMatch) float64
}

type Matching struct {
	logger *zap.Logger
	repo   MatchingRepo
	scorer Scorer
	cfg    MatchingConfig
}

type MatchingConfig struct {
	SimilarItemsAmount     int     // сколько похожих вещей мы берём из БД для каждого узла графа
	CompatibilityThreshold float64 // минимальная схожесть желания и предложения для создания ребра
	ChainLen               int     // длина цепочки (глубина графа)
	PenaltyFactor          float64 // штраф за несбалансированные цепи, например: edgesScore = [90, 90, 10]
	ChainRatingThreshold   float64 // граница рейтинга для цепи, все что ниже, не попадает в выдачу
}

func NewMatching(
	logger *zap.Logger,
	repo MatchingRepo,
	scorer Scorer,
	cfg MatchingConfig,
) (*Matching, error) {
	switch {
	case logger == nil:
		return nil, fmt.Errorf("matching init: 'logger' is required")
	case repo == nil:
		return nil, fmt.Errorf("matching init: 'matching repo' is required")
	case scorer == nil:
		return nil, fmt.Errorf("matching init: 'scorer implementation' is required")
	}

	if cfg.ChainLen < 3 || cfg.ChainLen > 3 {
		return nil, fmt.Errorf("matching init: invalid 'max chain len' %d", cfg.ChainLen)
	}
	if cfg.SimilarItemsAmount <= 0 || cfg.SimilarItemsAmount > 40 {
		return nil, fmt.Errorf("matching init: invalid 'similar items amount' range %d", cfg.SimilarItemsAmount)
	}
	if math.IsNaN(cfg.ChainRatingThreshold) || math.IsInf(cfg.ChainRatingThreshold, 0) ||
		cfg.ChainRatingThreshold < 0 || cfg.ChainRatingThreshold > 1 {
		return nil, fmt.Errorf("matching init: invalid 'chain rating threshold' range %g", cfg.ChainRatingThreshold)
	}
	if math.IsNaN(cfg.CompatibilityThreshold) || math.IsInf(cfg.CompatibilityThreshold, 0) ||
		cfg.CompatibilityThreshold < -1 || cfg.CompatibilityThreshold > 1 {
		return nil, fmt.Errorf("matching init: invalid 'compatibility threshold' range %g", cfg.CompatibilityThreshold)
	}
	if math.IsNaN(cfg.PenaltyFactor) || math.IsInf(cfg.PenaltyFactor, 0) || cfg.PenaltyFactor < 0 {
		return nil, fmt.Errorf("matching init: invalid 'penalty factor' %g", cfg.PenaltyFactor)
	}

	return &Matching{
		logger: logger,
		repo:   repo,
		scorer: scorer,
		cfg:    cfg,
	}, nil
}

func (m *Matching) FindCycles(ctx context.Context, itemID int) ([][]model.Edge, error) {
	matchable, err := m.repo.IsSourceMatchable(ctx, itemID)
	if err != nil {
		return nil, fmt.Errorf("validate source item %d: %w", itemID, err)
	}
	if !matchable {
		return nil, fmt.Errorf("find cycles for item %d: %w", itemID, model.ErrItemNotMatchable)
	}

	graph := model.NewMemoryStore()

	err = m.assembleGraph(ctx, itemID, graph)
	if err != nil {
		return nil, err
	}

	chains, err := graph.FindCycles(m.cfg.ChainLen)
	if err != nil {
		return nil, err
	}

	return m.filterChainsByScoreAndRoot(chains, itemID), nil
}

func (m *Matching) assembleGraph(ctx context.Context, rootID int, graph model.Graph) error {
	var errs []error
	queue := make([]int, 0, 1)
	visited := make(map[int]struct{})
	edgeCounter := 0

	if err := graph.AddVertex(model.Vertex{ItemID: rootID}); err != nil {
		return fmt.Errorf("add root item %d to graph: %w", rootID, err)
	}
	queue = append(queue, rootID)
	visited[rootID] = struct{}{}

	for i := 0; i < m.cfg.ChainLen; i++ {
		nextQueue := make([]int, 0, len(queue))

		for _, candidateID := range queue {
			matches, err := m.findSimilarItems(ctx, candidateID)
			if err != nil {
				errs = append(errs, err)
			}

			for _, match := range matches {
				if match.Similarity < m.cfg.CompatibilityThreshold {
					continue
				}

				graph.AddVertex(model.Vertex{ItemID: match.TargetItem.ID})
				score := m.scorer.CalculateScore(match)

				if err := graph.AddEdge(model.Edge{
					ID:       edgeCounter,
					SourceID: candidateID,
					TargetID: match.TargetItem.ID,
					Score:    score,
				}); err != nil {
					errs = append(errs, fmt.Errorf("add edge %d->%d: %w", candidateID, match.TargetItem.ID, err))
					continue
				}
				edgeCounter++

				if _, ok := visited[match.TargetItem.ID]; !ok {
					visited[match.TargetItem.ID] = struct{}{}
					nextQueue = append(nextQueue, match.TargetItem.ID)
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
	switch {
	case graph.EdgesAmount() < 1 && joinedErrs != nil:
		return fmt.Errorf("assemble graph: %w", joinedErrs)
	case graph.EdgesAmount() < 1:
		return fmt.Errorf("assemble graph: no edges found for item %d", rootID)
	case joinedErrs != nil:
		m.logger.Warn("assemble graph: failed chains", zap.Error(joinedErrs))
	}

	return nil
}

// findSimilarItems ищет K похожих предметов для itemID вещи.
// Данный метод не устанавливает конечную связь, а только ищет претендентов.
//
// returns:
// - []model.ItemMatch - кандидаты на создание связи с itemID вещью
// - error - ошибка если она была
func (m *Matching) findSimilarItems(ctx context.Context, itemID int) ([]model.ItemMatch, error) {
	matches, err := m.repo.FindSimilarItems(ctx, itemID, m.cfg.SimilarItemsAmount)
	if err != nil {
		return nil, fmt.Errorf("find similar items for item %d: %w", itemID, err)
	}

	return matches, nil
}

// filterChainsByScoreAndRoot - фильтрует цепочки по корневому узлу и рейтингу.
// Корневой узел - та вещь, для которой мы строим цепочку.
func (m *Matching) filterChainsByScoreAndRoot(chains [][]model.Edge, rootID int) [][]model.Edge {
	filteredChains := make([][]model.Edge, 0, len(chains))

	for _, chain := range chains {
		hasRootItem := false
		for _, edge := range chain {
			if edge.SourceID == rootID {
				hasRootItem = true
				break
			}
		}

		if !hasRootItem {
			continue
		}

		if m.CalculateChainScore(chain) >= m.cfg.ChainRatingThreshold {
			filteredChains = append(filteredChains, chain)
		}
	}

	return filteredChains
}

// CalculateChainScore - рассчитывает рейтинг целой цепочки при помощи среднего скоректированного на дисперсию.
func (m *Matching) CalculateChainScore(chain []model.Edge) float64 {
	if len(chain) == 0 {
		return 0
	}

	sum := 0.0
	for _, edge := range chain {
		sum += edge.Score
	}
	mean := sum / float64(len(chain))

	varianceSum := 0.0
	for _, edge := range chain {
		diff := edge.Score - mean
		varianceSum += diff * diff // math.Pow() тяжелый для обычного возведения во 2-ую степень
	}
	variance := varianceSum / float64(len(chain))
	stdDev := math.Sqrt(variance)

	// lenPenalty штрафует цепь за длину
	lenPenalty := math.Pow(0.9, float64(len(chain)-2))

	return (mean - (m.cfg.PenaltyFactor * stdDev)) * lenPenalty
}
