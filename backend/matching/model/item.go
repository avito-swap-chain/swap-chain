package model

type Item struct {
	ID               int
	OfferTitle       string
	OfferDescription string
	WantDescription  string
	OfferVector      []float32
	WantVector       []float32

	Meta ItemMetadata
}

// ItemMetadata хранит информацию, которая используется при подсчете score.
type ItemMetadata struct {
	TitleLen       int
	DescriptionLen int
	ImageAmount    int
	ParamRichness  float64
	QualityScore   float64
	UserRating     float64
	SuccessRate    float64
}

// ItemMatch - представление связи между вещами, которое в отличии от обычного Edge используется в бизнес-правилах.
type ItemMatch struct {
	SourceID   int
	TargetItem Item
	Similarity float64
}
