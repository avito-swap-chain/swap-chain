package model

type AnalysisItem struct {
	ID               int64
	OfferTitle       string
	OfferDescription string
	WantDescription  string
}

type AnalysisResult struct {
	ItemID           int64
	OfferCategoryID  int32
	WantCategoryID   int32
	ParamRichness    float64
	IsCategoryManual bool
	OfferEmbedding   []float32
	WantEmbedding    []float32
}
