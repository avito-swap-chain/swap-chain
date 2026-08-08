package service

import "swap-chain/internal/modules/matching/model"

type Scoring struct {
}

func NewScoring() *Scoring {
	return &Scoring{}
}

// CalculateScore считает score ребра, алгоритм подсчета основан на методе анализа иерархий Саати.
func (s *Scoring) CalculateScore(match model.ItemMatch) float64 {
	meta := match.TargetItem.Meta

	baseScore := 0.83*match.Similarity + 0.17*meta.ParamRichness
	additionalScore := s.calculateAdditionalScore(meta)
	reliabilityScore := s.calculateReliabilityScore(meta)

	finalScore := 0.65*baseScore + 0.12*additionalScore + 0.23*reliabilityScore

	if finalScore > 1.0 {
		return 1.0
	}
	return finalScore
}

// calculateReliabilityScore подсчитывает score надежности (неразрывности) обмена.
func (s *Scoring) calculateReliabilityScore(meta model.ItemMetadata) float64 {
	normalizedRating := 0.0
	if meta.UserRating > 0 {
		normalizedRating = meta.UserRating / 5.0
	}

	score := 0.75*meta.SuccessRate + 0.25*normalizedRating
	if score > 1.0 {
		return 1.0
	}
	return score
}

// calculateAdditionalScore подсчитывает бонусный score.
func (s *Scoring) calculateAdditionalScore(meta model.ItemMetadata) float64 {
	score := 0.0
	score += 0.39 * meta.QualityScore

	if meta.ImageAmount >= 3 {
		score += 0.39
	} else if meta.ImageAmount > 0 {
		score += 0.19
	}

	if meta.DescriptionLen >= 100 {
		score += 0.15
	} else if meta.DescriptionLen >= 30 {
		score += 0.07
	}

	if meta.TitleLen >= 15 {
		score += 0.07
	}

	if score > 1.0 {
		return 1.0
	}
	return score
}
