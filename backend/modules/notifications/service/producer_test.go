package service

import (
	"context"
	"errors"
	"testing"

	"swap-chain/modules/notifications/model"

	"go.uber.org/zap"
)

type producerServiceStub struct {
	created []model.Notification
	err     error
}

func (s *producerServiceStub) Create(_ context.Context, userID int64, kind, title, text string, chainID, itemID *int64) (model.Notification, error) {
	if s.err != nil {
		return model.Notification{}, s.err
	}
	n := model.Notification{ID: int64(len(s.created) + 1), UserID: userID, Kind: kind, Title: title, Text: text, ChainID: chainID, ItemID: itemID}
	s.created = append(s.created, n)
	return n, nil
}

func (*producerServiceStub) List(context.Context, int64, int64, int) (model.ListResult, error) {
	return model.ListResult{}, nil
}

func (*producerServiceStub) MarkRead(context.Context, int64, []int64) error { return nil }

func TestProducerCreatesNotificationForEveryRecipientAndPublishesSSE(t *testing.T) {
	service := &producerServiceStub{}
	type event struct {
		userID    int64
		entityID  string
		eventType string
	}
	var events []event
	producer := NewProducer(service, zap.NewNop(), func(userID int64, eventType, entityID string, _ map[string]any) {
		events = append(events, event{userID: userID, entityID: entityID, eventType: eventType})
	})

	producer.NotifyChainCreated(context.Background(), []int64{7, 8}, 42)
	if len(service.created) != 2 || len(events) != 2 {
		t.Fatalf("notifications/events = %d/%d", len(service.created), len(events))
	}
	for i, notification := range service.created {
		if notification.Kind != model.KindChain || notification.ChainID == nil || *notification.ChainID != 42 {
			t.Fatalf("notification[%d] = %+v", i, notification)
		}
		if events[i].eventType != "notification.created" || events[i].entityID == "" {
			t.Fatalf("event[%d] = %+v", i, events[i])
		}
		if events[i].userID != notification.UserID {
			t.Fatalf("event user = %d, notification user = %d", events[i].userID, notification.UserID)
		}
	}
}

func TestProducerFormatsBusinessSpecificNotifications(t *testing.T) {
	service := &producerServiceStub{}
	producer := NewProducer(service, zap.NewNop(), nil)

	producer.NotifyChainRejected(context.Background(), []int64{1}, 9, "участник отказался")
	producer.NotifyItemUnavailable(context.Background(), []int64{2}, "Велосипед", 15)
	producer.NotifyChatMessage(context.Background(), 3, "Анна", 15)

	if len(service.created) != 3 {
		t.Fatalf("created = %d", len(service.created))
	}
	if service.created[0].Text != "Вариант обмена отменён: участник отказался" {
		t.Fatalf("rejection text = %q", service.created[0].Text)
	}
	if service.created[1].ItemID == nil || *service.created[1].ItemID != 15 || service.created[1].Kind != model.KindOffer {
		t.Fatalf("unavailable notification = %+v", service.created[1])
	}
	if service.created[2].Title != "Новое сообщение от Анна" || service.created[2].Kind != model.KindMessage {
		t.Fatalf("chat notification = %+v", service.created[2])
	}
}

func TestProducerDoesNotPublishWhenPersistenceFails(t *testing.T) {
	service := &producerServiceStub{err: errors.New("db unavailable")}
	published := false
	producer := NewProducer(service, zap.NewNop(), func(int64, string, string, map[string]any) { published = true })
	producer.NotifyDeliveryAtPVZ(context.Background(), 1, "Самокат", 4)
	if published {
		t.Fatal("SSE was published after failed persistence")
	}
}
