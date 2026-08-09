package adapters

import "context"

type Enricher interface {
	GenerateJSON(ctx context.Context, prompt string) (string, error)
}

type Embedder interface {
	Vectorize(ctx context.Context, text string) ([]float32, error)
}

type FallbackClient struct {
	Primary  Enricher
	Fallback Enricher
}

func (f *FallbackClient) GenerateJSON(ctx context.Context, prompt string) (string, error) {
	res, err := f.Primary.GenerateJSON(ctx, prompt)
	if err != nil {
		return f.Fallback.GenerateJSON(ctx, prompt)
	}

	return res, nil
}
