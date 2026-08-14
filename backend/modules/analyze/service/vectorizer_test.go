package service

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"go.uber.org/zap"
)

type vectorizerEnricherStub struct {
	value string
	err   error
}

func (s vectorizerEnricherStub) GenerateJSON(context.Context, string) (string, error) {
	return s.value, s.err
}

type embedderStub struct {
	texts  []string
	vector []float32
	err    error
}

func (s *embedderStub) Vectorize(_ context.Context, text string) ([]float32, error) {
	s.texts = append(s.texts, text)
	return s.vector, s.err
}

func TestVectorizerUsesEnrichedTextForBothEmbeddings(t *testing.T) {
	t.Parallel()
	local := &embedderStub{vector: []float32{1}}
	external := &embedderStub{vector: []float32{2}}
	svc, err := NewVectorizer(vectorizerEnricherStub{value: `{"value":"смартфон, электроника"}`}, local, external, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	gotLocal, gotExternal, err := svc.EnrichAndVectorize(context.Background(), "телефон")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotLocal, []float32{1}) || !reflect.DeepEqual(gotExternal, []float32{2}) {
		t.Fatalf("vectors = %v/%v", gotLocal, gotExternal)
	}
	if !reflect.DeepEqual(local.texts, []string{"смартфон, электроника"}) || !reflect.DeepEqual(external.texts, local.texts) {
		t.Fatalf("embedded texts = %v/%v", local.texts, external.texts)
	}
}

func TestVectorizerFallsBackToOriginalText(t *testing.T) {
	t.Parallel()
	for _, enricher := range []vectorizerEnricherStub{
		{err: errors.New("unavailable")},
		{value: "not-json"},
		{value: `{"value":""}`},
	} {
		local := &embedderStub{vector: []float32{1}}
		external := &embedderStub{err: errors.New("external unavailable")}
		svc, err := NewVectorizer(enricher, local, external, zap.NewNop())
		if err != nil {
			t.Fatal(err)
		}
		_, gotExternal, err := svc.EnrichAndVectorize(context.Background(), "исходный текст")
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(local.texts, []string{"исходный текст"}) || gotExternal != nil {
			t.Fatalf("fallback text/vector = %v/%v", local.texts, gotExternal)
		}
	}
}

func TestVectorizerValidatesDependenciesAndLocalFailure(t *testing.T) {
	t.Parallel()
	enricher := vectorizerEnricherStub{value: `{"value":"ok"}`}
	embedder := &embedderStub{vector: []float32{1}}
	tests := []struct {
		name string
		make func() error
	}{
		{"enricher", func() error { _, err := NewVectorizer(nil, embedder, embedder, zap.NewNop()); return err }},
		{"local", func() error { _, err := NewVectorizer(enricher, nil, embedder, zap.NewNop()); return err }},
		{"external", func() error { _, err := NewVectorizer(enricher, embedder, nil, zap.NewNop()); return err }},
		{"logger", func() error { _, err := NewVectorizer(enricher, embedder, embedder, nil); return err }},
	}
	for _, tt := range tests {
		if err := tt.make(); err == nil || !strings.Contains(err.Error(), "required") {
			t.Fatalf("%s dependency error = %v", tt.name, err)
		}
	}

	localErr := errors.New("local unavailable")
	svc, _ := NewVectorizer(enricher, &embedderStub{err: localErr}, embedder, zap.NewNop())
	_, _, err := svc.EnrichAndVectorize(context.Background(), "text")
	if !errors.Is(err, localErr) {
		t.Fatalf("local vectorization error = %v", err)
	}
}

func TestPromptsContainUserInputAndExchangeWording(t *testing.T) {
	t.Parallel()
	if prompt := BuildEnrichmentPrompt("GoPro9"); !strings.Contains(prompt, "GoPro9") {
		t.Fatalf("enrichment prompt misses input: %q", prompt)
	}
	if prompt := BuildParamRichnessPrompt("точное описание"); !strings.Contains(prompt, "точное описание") {
		t.Fatalf("richness prompt misses description: %q", prompt)
	}
	if prompt := BuildCategoryDefinitionPrompt("1: Электроника", "Телефон", "Рабочий"); !strings.Contains(prompt, "1: Электроника") || !strings.Contains(prompt, "Телефон") || !strings.Contains(prompt, "Рабочий") {
		t.Fatalf("category prompt misses input: %q", prompt)
	}
	if strings.Contains(strings.ToLower(VisionAnalysisPrompt), "offered for sale") || !strings.Contains(VisionAnalysisPrompt, "offered for exchange") {
		t.Fatal("vision prompt must describe exchange, not sale")
	}
}
