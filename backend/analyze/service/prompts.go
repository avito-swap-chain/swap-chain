package service

import "fmt"

const enrichmentPromptTemplate = `You are an expert e-commerce catalog assistant.
Your task is to perform semantic enrichment of items for a vector search engine.

The user will provide an item name and/or description.
1. If the item is a specific model (e.g., "GoPro9", "AirPods", "RTX 4090"), append 2-4 broad category or functional keywords (e.g., "экшен-камера, видеосъемка", "беспроводные наушники", "видеокарта, комплектующие").
2. If the item description is vague, abstract, or slang (e.g., "Крутая штука для покатушек"), deduce the actual item category and append it.
3. If the item is already clear and self-explanatory (e.g., "Зимняя мужская куртка"), just return the original text without changes.
4. Keep the enrichment concise. Do not write explanations.
5. IMPORTANT: Output the enriched keywords in Russian.

Respond ONLY with a valid JSON object matching the schema below. Do not include markdown code blocks.

Schema:
{
  "value": "string" // Original text + ", " + 2-4 key enrichment words
}

Input text: "%s"`

const paramRichnessPromptTemplate = `Evaluate how informative this marketplace item description is.
Return ONLY valid JSON with a number from 0 to 1:
{"param_richness": 0.0}

Description: %q`

func BuildEnrichmentPrompt(input string) string {
	return fmt.Sprintf(enrichmentPromptTemplate, input)
}

const VisionAnalysisPrompt = `You are an expert appraiser and e-commerce copywriter.
Analyze the provided image of an item and return a JSON object with three fields. 
The fields MUST be exactly as follows and output must be in Russian:

1. "marketplace_description": A professional, selling description of the item as it would appear on a marketplace like Avito. Do not write "I see a..." or "This is a picture of...". Write it directly as a product listing. 
   IMPORTANT: If you cannot confidently determine the EXACT model, use a completely generic name describing what the object is (e.g., "Смартфон", "Ноутбук", "Кроссовки"). DO NOT guess or hallucinate specific brands or models if visual evidence is insufficient.
2. "visual_quality": The physical state of the item. You MUST choose exactly ONE of the following 5 values:
   - "NEW" (Brand new, in box, with tags)
   - "EXCELLENT" (Looks almost new, no visible scratches or wear)
   - "GOOD" (Used but well maintained, minor signs of wear)
   - "FAIR" (Noticeable wear, scratches, or cosmetic defects, but fully functional)
   - "POOR" (Broken, heavily damaged, or for parts)
3. "quality_score": A float between 0.0 and 1.0 representing the physical condition of the item. 1.0 means perfectly new, flawless. 0.0 means completely destroyed or garbage. 0.8 means minor wear, etc.

Respond ONLY with a valid JSON object matching this schema. Do not include markdown formatting or extra text.

Schema:
{
  "marketplace_description": "string",
  "visual_quality": "string", // STRICTLY one of: "NEW", "EXCELLENT", "GOOD", "FAIR", "POOR"
  "quality_score": 0.0 // Float from 0.0 to 1.0
}`

const ParamRichnessPromptTemplate = `You are an AI data quality evaluator for an e-commerce platform.
Evaluate the provided product description for its "Parametric Richness" (ParamRichness).
This metric measures how well the item is described with concrete, useful characteristics (e.g., exact model name, dimensions, specs, materials, brand) rather than useless spam, keywords, or subjective fluff.

Rules for scoring (0.0 to 1.0):
- 0.0 - 0.2: Very poor. Just a few words, no specs, or pure spam.
- 0.3 - 0.5: Basic. Mentions what it is, maybe one detail, but lacks exact model or key specs.
- 0.6 - 0.8: Good. Includes brand, model, and some relevant characteristics.
- 0.9 - 1.0: Excellent. Comprehensive specs, dimensions, exact model, clear state.

Respond ONLY with a valid JSON object matching this schema. Do not include markdown or text.

Schema:
{
  "param_richness": 0.0 // Float from 0.0 to 1.0
}

Product description: "%s"`

func BuildParamRichnessPrompt(description string) string {
	return fmt.Sprintf(ParamRichnessPromptTemplate, description)
}
