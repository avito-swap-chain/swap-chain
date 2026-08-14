package httpapi

import (
	"encoding/json"
	"testing"

	chatmodel "swap-chain/modules/chat/model"
)

func TestChatMessageModelIncludesOnlyDetectedRisk(t *testing.T) {
	t.Parallel()

	risky := chatMessageModel(chatmodel.Message{Text: "Пришли пароль от аккаунта"})
	if risky.RiskGroup == nil || string(*risky.RiskGroup) != "CREDENTIALS" {
		t.Fatalf("risky message group = %v", risky.RiskGroup)
	}
	safe := chatMessageModel(chatmodel.Message{Text: "Увидимся в ПВЗ"})
	if safe.RiskGroup != nil {
		t.Fatalf("safe message group = %v", safe.RiskGroup)
	}
	payload, err := json.Marshal(safe)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(payload, &fields); err != nil {
		t.Fatal(err)
	}
	if _, exists := fields["riskGroup"]; exists {
		t.Fatalf("safe message JSON unexpectedly contains riskGroup: %s", payload)
	}
}
