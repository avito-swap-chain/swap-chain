package items

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestNormalizeAndValidateCreateInput(t *testing.T) {
	categoryID := int32(3)
	input := normalize(CreateInput{
		OfferTitle:       "  Велосипед  ",
		OfferDescription: "  Почти новый  ",
		Wishes:           []string{"  Телефон ", " ", "Самокат"},
		OfferCategoryID:  &categoryID,
	})

	if input.OfferTitle != "Велосипед" || input.OfferDescription != "Почти новый" {
		t.Fatalf("normalize() text = %q/%q", input.OfferTitle, input.OfferDescription)
	}
	if input.Condition != ConditionGood {
		t.Fatalf("normalize() condition = %q, want %q", input.Condition, ConditionGood)
	}
	if !reflect.DeepEqual(input.Wishes, []string{"Телефон", "Самокат"}) {
		t.Fatalf("normalize() wishes = %v", input.Wishes)
	}
	if input.ImageURLs == nil {
		t.Fatal("normalize() must produce a non-nil image list")
	}
	if err := validate(input); err != nil {
		t.Fatalf("validate() error = %v", err)
	}
}

func TestValidateCreateInputRejectsCoreInvalidFields(t *testing.T) {
	categoryID := int32(1)
	tests := []struct {
		name  string
		input CreateInput
		field string
	}{
		{name: "missing category", input: CreateInput{OfferTitle: "A", OfferDescription: "B", Wishes: []string{"C"}, Condition: ConditionGood}, field: "categoryId"},
		{name: "unknown condition", input: CreateInput{OfferTitle: "A", OfferDescription: "B", Wishes: []string{"C"}, OfferCategoryID: &categoryID, Condition: "BROKEN"}, field: "condition"},
		{name: "no wishes", input: CreateInput{OfferTitle: "A", OfferDescription: "B", OfferCategoryID: &categoryID, Condition: ConditionGood}, field: "wishes"},
		{name: "invalid image", input: CreateInput{OfferTitle: "A", OfferDescription: "B", Wishes: []string{"C"}, ImageURLs: []string{"javascript:alert(1)"}, OfferCategoryID: &categoryID, Condition: ConditionGood}, field: "imageUrls"},
		{name: "long title", input: CreateInput{OfferTitle: strings.Repeat("я", 256), OfferDescription: "B", Wishes: []string{"C"}, OfferCategoryID: &categoryID, Condition: ConditionGood}, field: "offerTitle"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var validationError *ValidationError
			if err := validate(test.input); !errors.As(err, &validationError) || validationError.Fields[test.field] == "" {
				t.Fatalf("validate() error = %v, want %s error", err, test.field)
			}
		})
	}
}

func TestValidateUpdateConditionAndConflictingWithdraw(t *testing.T) {
	invalid := "EXCELLENT"
	var validationError *ValidationError
	if err := validateUpdate(UpdateInput{Condition: &invalid}); !errors.As(err, &validationError) || validationError.Fields["condition"] == "" {
		t.Fatalf("invalid condition error = %v", err)
	}

	validationError = nil
	if err := validateUpdate(UpdateInput{Withdraw: true, Wishes: []string{"Телефон"}}); !errors.As(err, &validationError) || validationError.Fields["wishes"] == "" {
		t.Fatalf("withdraw with wishes error = %v", err)
	}
}

func TestCursorRoundTripAndValidation(t *testing.T) {
	for _, value := range []int64{0, 1, 9223372036854775807} {
		parsed, err := ParseCursor(FormatCursor(value))
		if err != nil || parsed != value {
			t.Fatalf("cursor round trip for %d = %d, %v", value, parsed, err)
		}
	}
	for _, cursor := range []string{"-1", "abc", "1.5"} {
		if _, err := ParseCursor(cursor); err == nil {
			t.Fatalf("ParseCursor(%q) succeeded", cursor)
		}
	}
}

func TestValidateCategoryDecision(t *testing.T) {
	categoryID := int32(4)
	if err := validateCategoryDecision(CategoryDecisionInput{
		OfferCategoryID: &categoryID,
		Wishes:          []WishCategoryDecision{{WishID: 1, CategoryID: 2}},
	}); err != nil {
		t.Fatalf("valid decision error = %v", err)
	}

	var validationError *ValidationError
	err := validateCategoryDecision(CategoryDecisionInput{
		Wishes: []WishCategoryDecision{{WishID: 1, CategoryID: 2}, {WishID: 1, CategoryID: 3}},
	})
	if !errors.As(err, &validationError) || validationError.Fields["wishes"] == "" {
		t.Fatalf("duplicate decision error = %v", err)
	}
}

func TestNormalizeUpdateTrimsAndDropsBlankWishes(t *testing.T) {
	t.Parallel()
	title, description := "  Часы  ", "  Рабочие  "
	got := normalizeUpdate(UpdateInput{
		OfferTitle:       &title,
		OfferDescription: &description,
		Wishes:           []string{"  телефон ", "   ", "наушники"},
	})
	if *got.OfferTitle != "Часы" || *got.OfferDescription != "Рабочие" {
		t.Fatalf("normalized text = %q/%q", *got.OfferTitle, *got.OfferDescription)
	}
	if want := []string{"телефон", "наушники"}; !reflect.DeepEqual(got.Wishes, want) {
		t.Fatalf("normalized wishes = %v, want %v", got.Wishes, want)
	}
}

func TestConditionScoreRoundTrip(t *testing.T) {
	t.Parallel()
	tests := []struct {
		condition string
		score     float64
		stored    string
	}{
		{ConditionNew, 1, "1.000"},
		{ConditionGood, 0.8, "0.800"},
		{ConditionUsed, 0.6, "0.600"},
	}
	for _, tt := range tests {
		if got := conditionScore(tt.condition); got != tt.score {
			t.Fatalf("conditionScore(%q) = %v, want %v", tt.condition, got, tt.score)
		}
		if got := conditionFromScore(tt.stored); got != tt.condition {
			t.Fatalf("conditionFromScore(%q) = %q, want %q", tt.stored, got, tt.condition)
		}
		condition := tt.condition
		if got := nullableConditionScore(&condition); got != tt.score {
			t.Fatalf("nullableConditionScore(%q) = %v, want %v", tt.condition, got, tt.score)
		}
	}
	if nullableConditionScore(nil) != nil {
		t.Fatal("nullableConditionScore(nil) must be nil")
	}
}
