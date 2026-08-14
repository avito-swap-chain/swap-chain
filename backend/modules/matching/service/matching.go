package service

import (
	"context"
	"errors"
	"fmt"
	"math"

	"swap-chain/modules/matching/model"

	"go.uber.org/zap"
)

type MatchingRepo interface {
	FindSimilarItems(ctx context.Context, sourceID int64, limit int) ([]model.ItemMatch, error)
	ValidateSourceItem(ctx context.Context, itemID int64) error
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
	CompatibilityThreshold float64 // минимальный порог для создания ребра
	ChainLen               int     // длина цепочки (глубина графа)
	PenaltyFactor          float64 // штраф за несбалансированные цепи, например: edgesScore = [90, 90, 10]
	ChainRatingThreshold   float64 // граница рейтинга для цепи, все что ниже, не попадает в выдачу
	Debug                  bool    // включает подробный диагностический лог matching
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

	if cfg.ChainLen < 2 || cfg.ChainLen > 3 {
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

func (m *Matching) FindCycles(ctx context.Context, itemID int64) ([][]model.Edge, error) {
	m.debug("cycle search started", zap.Int64("root_item_id", itemID))
	if err := m.repo.ValidateSourceItem(ctx, itemID); err != nil {
		return nil, fmt.Errorf("validate source item %d: %w", itemID, err)
	}

	graph := model.NewMemoryStore()

	err := m.assembleGraph(ctx, itemID, graph)
	if err != nil {
		return nil, err
	}

	chains, err := graph.FindCycles(m.cfg.ChainLen)
	if err != nil {
		return nil, err
	}

	m.debug("raw cycles found", zap.Int("count", len(chains)), zap.Any("cycles", chains))
	filtered := m.filterChainsByScoreAndRoot(chains, itemID)
	m.debug("cycle search completed", zap.Int64("root_item_id", itemID), zap.Int("accepted_count", len(filtered)), zap.Any("accepted_cycles", filtered))

	return filtered, nil
}

func (m *Matching) assembleGraph(ctx context.Context, rootID int64, graph model.Graph) error {
	var errs []error
	queue := make([]int64, 0, 1)
	visited := make(map[int64]struct{})
	edgeCounter := 0

	if err := graph.AddVertex(model.Vertex{ItemID: rootID}); err != nil {
		return fmt.Errorf("add root item %d to graph: %w", rootID, err)
	}
	queue = append(queue, rootID)
	visited[rootID] = struct{}{}

	for i := 0; i < m.cfg.ChainLen; i++ {
		nextQueue := make([]int64, 0, len(queue))

		for _, candidateID := range queue {
			matches, err := m.findSimilarItems(ctx, candidateID)
			if err != nil {
				errs = append(errs, err)
			}

			matches = applyElbowMethod(matches, m.cfg.CompatibilityThreshold)

			for _, match := range matches {
				m.debug("candidate evaluated",
					zap.Int64("source_item_id", candidateID),
					zap.Int64("target_item_id", match.TargetItem.ID),
					zap.Float64("similarity", match.Similarity),
				)

				if err := graph.AddVertex(model.Vertex{ItemID: match.TargetItem.ID}); err != nil &&
					!errors.Is(err, model.ErrVertexAlreadyExists) {
					errs = append(errs, fmt.Errorf("add vertex %d: %w", match.TargetItem.ID, err))
					continue
				}
				score := m.scorer.CalculateScore(match)
				m.debug("edge accepted",
					zap.Int64("source_item_id", candidateID),
					zap.Int64("target_item_id", match.TargetItem.ID),
					zap.Float64("edge_score", score),
				)

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

	joinedErrs := errors.Join(errs...)
	switch {
	case graph.EdgesAmount() < 1 && joinedErrs != nil:
		return fmt.Errorf("assemble graph: %w", joinedErrs)
	case graph.EdgesAmount() < 1:
		return nil
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
func (m *Matching) findSimilarItems(ctx context.Context, itemID int64) ([]model.ItemMatch, error) {
	matches, err := m.repo.FindSimilarItems(
		ctx,
		itemID,
		m.cfg.SimilarItemsAmount,
	)
	if err != nil {
		return nil, fmt.Errorf("find similar items for item %d: %w", itemID, err)
	}

	return matches, nil
}

// filterChainsByScoreAndRoot - фильтрует цепочки по корневому узлу и рейтингу.
// Корневой узел - та вещь, для которой мы строим цепочку.
func (m *Matching) filterChainsByScoreAndRoot(chains [][]model.Edge, rootID int64) [][]model.Edge {
	filteredChains := make([][]model.Edge, 0, len(chains))

	for index, chain := range chains {
		hasRootItem := false
		for _, edge := range chain {
			if edge.SourceID == rootID {
				hasRootItem = true
				break
			}
		}

		if !hasRootItem {
			m.debug("cycle rejected", zap.Int("cycle_index", index), zap.String("reason", "root item is absent"), zap.Any("edges", chain))
			continue
		}

		score := m.CalculateChainScore(chain)
		if score >= m.cfg.ChainRatingThreshold {
			m.debug("cycle accepted", zap.Int("cycle_index", index), zap.Float64("score", score), zap.Any("edges", chain))
			filteredChains = append(filteredChains, chain)
			continue
		}
		m.debug("cycle rejected",
			zap.Int("cycle_index", index),
			zap.String("reason", "score is below threshold"),
			zap.Float64("score", score),
			zap.Float64("threshold", m.cfg.ChainRatingThreshold),
			zap.Any("edges", chain),
		)
	}

	return filteredChains
}

// applyElbowMethod динамически отсекает семантический мусор по самому резкому падению скора.
func applyElbowMethod(matches []model.ItemMatch, absoluteMin float64) []model.ItemMatch {
	if len(matches) == 0 {
		return matches
	}

	bestScore := matches[0].Similarity
	minAllowedScore := bestScore * 0.80
	if minAllowedScore < absoluteMin {
		minAllowedScore = absoluteMin
	}

	var totalDrop float64
	for i := 0; i < len(matches)-1; i++ {
		totalDrop += (matches[i].Similarity - matches[i+1].Similarity)
	}
	avgDrop := 0.0
	if len(matches) > 1 {
		avgDrop = totalDrop / float64(len(matches)-1)
	}

	dropIndex := len(matches)
	for i := 0; i < len(matches); i++ {
		if matches[i].Similarity < minAllowedScore {
			dropIndex = i
			break
		}

		if i < len(matches)-1 {
			drop := matches[i].Similarity - matches[i+1].Similarity
			if drop > avgDrop*2.5 && drop > 0.01 {
				dropIndex = i + 1
				break
			}
		}
	}

	return matches[:dropIndex]
}

func (m *Matching) debug(message string, fields ...zap.Field) {
	if m.cfg.Debug {
		m.logger.Info("matching debug: "+message, fields...)
	}
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
