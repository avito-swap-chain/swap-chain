package service

import "testing"

func TestClassifyMessageRisk(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		text string
		want RiskGroup
	}{
		{name: "safe", text: "Давайте встретимся завтра в ПВЗ", want: ""},
		{name: "password", text: "Пришли пароль от аккаунта", want: RiskCredentials},
		{name: "sms code", text: "Сообщи мне код из СМС", want: RiskVerificationCode},
		{name: "card", text: "Напиши номер карты и CVV", want: RiskPaymentDetails},
		{name: "url", text: "Посмотри фото на https://example.com/item", want: RiskExternalLink},
		{name: "bare domain", text: "Открой example.ru/deal", want: RiskExternalLink},
		{name: "messenger", text: "Перейдём в телеграм", want: RiskOffPlatformContact},
		{name: "highest priority", text: "Пароль введи по ссылке https://example.com", want: RiskCredentials},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := ClassifyMessageRisk(tt.text); got != tt.want {
				t.Fatalf("ClassifyMessageRisk(%q) = %q, want %q", tt.text, got, tt.want)
			}
		})
	}
}
