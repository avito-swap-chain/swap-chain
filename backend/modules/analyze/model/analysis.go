package model

type AnalysisItem struct {
	ID                    int64
	UserID                int64
	AnalysisVersion       int64
	OfferTitle            string
	OfferDescription      string
	OfferCategoryID       *int32
	OfferCategoryIsManual bool
}

type AnalysisWish struct {
	ID               int64
	Description      string
	CategoryID       *int32
	CategoryIsManual bool
}

type WishAnalysisResult struct {
	ID                    int64
	CategoryID            *int32
	CategoryIsManual      bool
	WantEmbeddingLocal    []float32
	WantEmbeddingExternal []float32
}

type AnalysisResult struct {
	ItemID                    int64
	AnalysisVersion           int64
	OfferCategoryID           *int32
	OfferCategoryIsManual     bool
	ParamRichness             float64
	OfferEmbeddingLocal       []float32
	OfferEmbeddingExternal    []float32
	Wishes                    []WishAnalysisResult
	RequiresCategoryInput     bool
}
