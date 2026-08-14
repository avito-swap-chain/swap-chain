package service

import "regexp"

// RiskGroup identifies the warning the client should show for a chat message.
// Empty value means that no deterministic risk marker was found.
type RiskGroup string

const (
	RiskCredentials        RiskGroup = "CREDENTIALS"
	RiskVerificationCode   RiskGroup = "VERIFICATION_CODE"
	RiskPaymentDetails     RiskGroup = "PAYMENT_DETAILS"
	RiskExternalLink       RiskGroup = "EXTERNAL_LINK"
	RiskOffPlatformContact RiskGroup = "OFF_PLATFORM_CONTACT"
)

type riskRule struct {
	group   RiskGroup
	pattern *regexp.Regexp
}

// Rules are ordered from the most dangerous signal to the least dangerous one.
// Keeping the classifier deterministic makes its result stable for stored messages.
var messageRiskRules = []riskRule{
	{RiskCredentials, regexp.MustCompile(`(?i)(парол\w*|логин\w*|данн\w*\s+(?:от|для)\s+(?:аккаунт\w*|вход\w*)|секретн\w*\s+(?:слов\w*|фраз\w*))`)},
	{RiskVerificationCode, regexp.MustCompile(`(?i)(код\w*\s+(?:из|в)\s+(?:смс|sms)|код\w*\s+подтвержден\w*|одноразов\w*\s+код\w*|пришл\w*\s+(?:мне\s+)?код\w*|сообщ\w*\s+(?:мне\s+)?код\w*)`)},
	{RiskPaymentDetails, regexp.MustCompile(`(?i)(номер\w*\s+карт\w*|данн\w*\s+карт\w*|\b(?:cvv|cvc)\b|перевед\w*\s+(?:деньг\w*|рубл\w*)|оплат\w*\s+(?:по|через)\s+ссылк\w*)`)},
	{RiskExternalLink, regexp.MustCompile(`(?i)(https?://|www\.|t\.me/|(?:^|[\s(])(?:[a-zа-яё0-9-]+\.)+(?:ru|com|net|org|рф)(?:[/\s),!?]|$))`)},
	{RiskOffPlatformContact, regexp.MustCompile(`(?i)(телеграм\w*|telegram\w*|ватсап\w*|whats ?app\w*|вайбер\w*|viber\w*|напиш\w*\s+(?:мне\s+)?(?:в|на)\s+(?:тг|лс)|перейд\w*\s+(?:в|на)\s+(?:тг|мессенджер\w*))`)},
}

// ClassifyMessageRisk returns the highest-priority risk group found in text.
func ClassifyMessageRisk(text string) RiskGroup {
	for _, rule := range messageRiskRules {
		if rule.pattern.MatchString(text) {
			return rule.group
		}
	}
	return ""
}
