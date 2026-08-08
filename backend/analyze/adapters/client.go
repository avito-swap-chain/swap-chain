package adapters

import "context"

type LLMClient interface {
	GenerateJSON(ctx context.Context, prompt string) (string, error)
	Vectorize(ctx context.Context, text string) ([]float32, error)
}

type FallbackClient struct {
	Primary LLMClient
	Fallback LLMClient
}

func (f *FallbackClient) GenerateJSON(ctx context.Context, prompt string) (string, error) {
	res, err := f.Primary.GenerateJSON(ctx, prompt)
	if err != nil {
		return f.Fallback.GenerateJSON(ctx, prompt)
	}

	return res, nil
}

func (f *FallbackClient) Vectorize(ctx context.Context, text string) ([]float32, error) {
	res, err := f.Primary.Vectorize(ctx, text)
	if err != nil {
		return f.Fallback.Vectorize(ctx, text)
	}

	return res, nil
}