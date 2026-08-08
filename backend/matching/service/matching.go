package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"swap-chain/shared/db"
	"swap-chain/matching/model"

	"go.uber.org/zap"
)

type VectorAdapter interface {
	Vectorize(ctx context.Context, text string) ([]float64, error)
}

type MatchingRepo interface {
	FindSimilarItems(ctx context.Context, arg db.FindSimilarItemsParams) ([]db.FindSimilarItemsRow, error)
}

type Scorer interface {
	CalculateScore(match model.ItemMatch) float64
}

type Matching struct {
	logger  *zap.Logger
	adapter VectorAdapter
	repo    MatchingRepo
	scorer  Scorer
	cfg     MatchingConfig
}

type MatchingConfig struct {
	SimilarItemsAmount   int     // сколько похожих вещей мы берём из БД для каждого узла графа
	ChainLen             int     // длина цепочки (глубина графа)
	PenaltyFactor        float64 // штраф за несбалансированные цепи, например: edgesScore = [90, 90, 10]
	ChainRatingThreshold float64 // граница рейтинга для цепи, все что ниже, не попадает в выдачу
}

func NewMatching(
	logger *zap.Logger,
	adapter VectorAdapter,
	repo MatchingRepo,
	scorer Scorer,
	cfg MatchingConfig,
) *Matching {
	return &Matching{
		logger:  logger,
		adapter: adapter,
		repo:    repo,
		scorer:  scorer,
		cfg:     cfg,
	}
}

func (m *Matching) Vectorize(ctx context.Context, text string) ([]float64, error) {
	return m.adapter.Vectorize(ctx, text)
}

func (m *Matching) FindCycles(ctx context.Context, itemID int) ([][]model.Edge, error) {
	graph := model.NewMemoryStore()

	err := m.assembleGraph(ctx, itemID, graph)
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

	graph.AddVertex(model.Vertex{ItemID: rootID})
	queue = append(queue, rootID)
	visited[rootID] = struct{}{}

	for i := 0; i < m.cfg.ChainLen-1; i++ {
		nextQueue := make([]int, 0, len(queue))

		for _, candidateID := range queue {
			matches, err := m.findSimilarItems(ctx, candidateID)
			if err != nil {
				errs = append(errs, err)
			}

			for _, match := range matches {
				graph.AddVertex(model.Vertex{ItemID: match.TargetItem.ID})
				score := m.scorer.CalculateScore(match)

				graph.AddEdge(model.Edge{
					ID:       edgeCounter,
					SourceID: candidateID,
					TargetID: match.TargetItem.ID,
					Score:    score,
				})
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
	if graph.EdgesAmount() < 1 && joinedErrs != nil {
		return fmt.Errorf("assemble graph: %w", joinedErrs)
	} else if graph.EdgesAmount() < 1 {
		return fmt.Errorf("assemble graph: no edges found for item %d", rootID)
	} else if graph.EdgesAmount() > 0 && joinedErrs != nil {
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
	params := db.FindSimilarItemsParams{
		ID:    int64(itemID),
		Limit: int32(m.cfg.SimilarItemsAmount),
	}

	res, err := m.repo.FindSimilarItems(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("find similar items: %w", err)
	}

	candidateMatches := make([]model.ItemMatch, 0, len(res))
	for _, dbCandidate := range res {
		candidateMatches = append(candidateMatches, mapDBRowToItemMatch(itemID, dbCandidate))
	}

	return candidateMatches, nil
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

		if m.calculateChainScore(chain) >= m.cfg.ChainRatingThreshold {
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

// mapDBRowToItemMatch конвертирует плоскую строку из БД в доменную структуру связи.
func mapDBRowToItemMatch(sourceID int, row db.FindSimilarItemsRow) model.ItemMatch {
	return model.ItemMatch{
		SourceID: sourceID,
		TargetItem: model.Item{
			ID:               int(row.ID),
			OfferTitle:       row.OfferTitle,
			OfferDescription: row.OfferDescription.String,
			WantDescription:  row.WantDescription.String,
			OfferVector:      row.OfferEmbedding.Slice(),
			WantVector:       row.WantEmbedding.Slice(),
			Meta: model.ItemMetadata{
				TitleLen:       len([]rune(row.OfferTitle)),
				DescriptionLen: len([]rune(row.OfferDescription.String)),
				// НЕ ЗАБЫТЬ ПРОКИНУТЬ ПОЛЯ, КОГДА ОНИ ПОЯВЯТСЯ
			},
		},
		Similarity: row.Similarity,
	}
}
