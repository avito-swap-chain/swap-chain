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

func BuildEnrichmentPrompt(input string) string {
	return fmt.Sprintf(enrichmentPromptTemplate, input)
}
