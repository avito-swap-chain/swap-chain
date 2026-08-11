package service

import (
	"context"
	"fmt"
	"strconv"

	"swap-chain/modules/notifications/model"

	"go.uber.org/zap"
)

type SSEPublisher func(userID int64, eventType, entityID string, data map[string]any)

type Producer struct {
	service Service
	logger  *zap.Logger
	sse     SSEPublisher
}

func NewProducer(service Service, logger *zap.Logger, sse SSEPublisher) *Producer {
	return &Producer{service: service, logger: logger, sse: sse}
}

func (p *Producer) notifyUser(ctx context.Context, userID int64, kind, title, text, targetURL string, entityID *int64) {
	notification, err := p.service.Create(ctx, userID, kind, title, text, targetURL, entityID)
	if err != nil {
		p.logger.Error("create notification",
			zap.Int64("user_id", userID),
			zap.String("kind", kind),
			zap.Error(err),
		)
		return
	}

	if p.sse != nil {
		data := map[string]any{
			"id":       strconv.FormatInt(notification.ID, 10),
			"kind":     notification.Kind,
			"title":    notification.Title,
			"text":     notification.Text,
			"targetUrl": notification.TargetURL,
			"isRead":   notification.IsRead,
		}
		if notification.EntityID != nil {
			data["entityId"] = strconv.FormatInt(*notification.EntityID, 10)
		}
		p.sse(userID, "notification.created", strconv.FormatInt(notification.ID, 10), data)
	}
}

func (p *Producer) NotifyChainCreated(ctx context.Context, userIDs []int64, chainID int64) {
	targetURL := fmt.Sprintf("/chains/%d", chainID)
	eID := chainID
	for _, uid := range userIDs {
		p.notifyUser(ctx, uid, model.KindChain,
			"Новая цепочка обмена",
			"Создано новое предложение обмена с вашим участием",
			targetURL, &eID)
	}
}

func (p *Producer) NotifyChainUpdated(ctx context.Context, userIDs []int64, chainID int64) {
	targetURL := fmt.Sprintf("/chains/%d", chainID)
	eID := chainID
	for _, uid := range userIDs {
		p.notifyUser(ctx, uid, model.KindChain,
			"Обновление цепочки обмена",
			"Один из участников подтвердил участие в цепочке",
			targetURL, &eID)
	}
}

func (p *Producer) NotifyChainAccepted(ctx context.Context, userIDs []int64, chainID int64) {
	targetURL := fmt.Sprintf("/chains/%d", chainID)
	eID := chainID
	for _, uid := range userIDs {
		p.notifyUser(ctx, uid, model.KindChain,
			"Цепочка принята",
			"Все участники подтвердили обмен. Вещи зарезервированы",
			targetURL, &eID)
	}
}

func (p *Producer) NotifyChainRejected(ctx context.Context, userIDs []int64, chainID int64, reason string) {
	targetURL := fmt.Sprintf("/chains/%d", chainID)
	eID := chainID
	var title, text string
	switch reason {
	case "declined":
		title = "Цепочка отклонена"
		text = "Один из участников отклонил предложение обмена"
	case "expired":
		title = "Цепочка истекла"
		text = "Время на принятие решения истекло"
	case "item_unavailable":
		title = "Цепочка недоступна"
		text = "Одна из вещей стала недоступна для обмена"
	default:
		title = "Цепочка отменена"
		text = fmt.Sprintf("Цепочка обмена отменена: %s", reason)
	}
	for _, uid := range userIDs {
		p.notifyUser(ctx, uid, model.KindChain, title, text, targetURL, &eID)
	}
}

func (p *Producer) NotifyChatMessage(ctx context.Context, recipientID int64, senderUsername string, chainID int64, counterpartID int64) {
	targetURL := fmt.Sprintf("/chains/%d/chat/%d", chainID, counterpartID)
	eID := chainID
	p.notifyUser(ctx, recipientID, model.KindMessage,
		fmt.Sprintf("Новое сообщение от %s", senderUsername),
		fmt.Sprintf("У вас новое сообщение в чате от %s", senderUsername),
		targetURL, &eID)
}
