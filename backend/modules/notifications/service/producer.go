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

func (p *Producer) notifyUser(ctx context.Context, userID int64, kind, title, text string, chainID, itemID *int64) {
	notification, err := p.service.Create(ctx, userID, kind, title, text, chainID, itemID)
	if err != nil {
		p.logger.Error("create notification",
			zap.Int64("user_id", userID),
			zap.String("kind", kind),
			zap.Error(err),
		)
		return
	}

	if p.sse != nil {
		p.sse(userID, "notification.created", strconv.FormatInt(notification.ID, 10), nil)
	}
}

func (p *Producer) NotifyChainCreated(ctx context.Context, userIDs []int64, chainID int64) {
	cID := chainID
	for _, uid := range userIDs {
		p.notifyUser(ctx, uid, model.KindChain,
			"Новая цепочка обмена",
			"Создано новое предложение обмена с вашим участием",
			&cID, nil)
	}
}

func (p *Producer) NotifyChainUpdated(ctx context.Context, userIDs []int64, chainID int64) {
	cID := chainID
	for _, uid := range userIDs {
		p.notifyUser(ctx, uid, model.KindChain,
			"Обновление цепочки обмена",
			"Один из участников подтвердил участие в цепочке",
			&cID, nil)
	}
}

func (p *Producer) NotifyChainAccepted(ctx context.Context, userIDs []int64, chainID int64) {
	cID := chainID
	for _, uid := range userIDs {
		p.notifyUser(ctx, uid, model.KindChain,
			"Цепочка собралась",
			"Цепочка собралась. Сдайте вещь в пункт выдачи",
			&cID, nil)
	}
}

func (p *Producer) NotifyChainRejected(ctx context.Context, userIDs []int64, chainID int64, reason string) {
	cID := chainID
	var text string
	if reason != "" {
		text = fmt.Sprintf("Вариант обмена отменён: %s", reason)
	} else {
		text = "Вариант обмена отменён"
	}
	for _, uid := range userIDs {
		p.notifyUser(ctx, uid, model.KindChain,
			"Вариант обмена отменён",
			text,
			&cID, nil)
	}
}

func (p *Producer) NotifyItemUnavailable(ctx context.Context, userIDs []int64, itemTitle string, itemID int64) {
	iID := itemID
	for _, uid := range userIDs {
		p.notifyUser(ctx, uid, model.KindOffer,
			"Вещь недоступна",
			fmt.Sprintf("«%s» ушла в другую цепочку, остальные варианты в силе", itemTitle),
			nil, &iID)
	}
}

func (p *Producer) NotifyCategoryActionRequired(ctx context.Context, userID int64, itemTitle string, itemID int64) {
	iID := itemID
	p.notifyUser(ctx, userID, model.KindOffer,
		"Уточните категорию",
		fmt.Sprintf("Для карточки «%s» не удалось уверенно определить категорию. Выберите один из предложенных вариантов", itemTitle),
		nil, &iID)
}

func (p *Producer) NotifyChatMessage(ctx context.Context, recipientID int64, senderUsername string, chainID int64) {
	cID := chainID
	p.notifyUser(ctx, recipientID, model.KindMessage,
		fmt.Sprintf("Новое сообщение от %s", senderUsername),
		fmt.Sprintf("Новое сообщение от %s", senderUsername),
		&cID, nil)
}

func (p *Producer) NotifyDeliveryAtPVZ(ctx context.Context, userID int64, itemTitle string, itemID int64) {
	iID := itemID
	p.notifyUser(ctx, userID, model.KindDelivery,
		"Вещь в пункте выдачи",
		fmt.Sprintf("Ваша вещь «%s» в пункте выдачи", itemTitle),
		nil, &iID)
}

func (p *Producer) NotifyDeliveryInTransit(ctx context.Context, userID int64, itemTitle string, itemID int64) {
	iID := itemID
	p.notifyUser(ctx, userID, model.KindDelivery,
		"Вещь едет получателю",
		fmt.Sprintf("Вещь «%s» едет получателю", itemTitle),
		nil, &iID)
}
