package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"swap-chain/modules/chat/model"
)

type repositoryStub struct {
	mu            sync.Mutex
	messages      []model.Message
	listHook      func()
	firstList     chan struct{}
	firstListOnce sync.Once
	nextID        int64
	threads       []model.Thread
}

func TestNewRequiresRepository(t *testing.T) {
	if _, err := New(nil); err == nil {
		t.Fatal("New(nil) error = nil")
	}
}

func (r *repositoryStub) CreateMessage(_ context.Context, chainID, actorID, counterpartID int64, clientMessageID, text string) (model.Message, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, message := range r.messages {
		if message.ChainID == chainID && message.Sender.ID == actorID && message.Recipient.ID == counterpartID && message.ClientMessageID == clientMessageID {
			if message.Text != text {
				return model.Message{}, false, model.ErrIdempotencyConflict
			}
			return message, false, nil
		}
	}
	r.nextID++
	message := model.Message{
		ID:              r.nextID,
		ChainID:         chainID,
		Sender:          model.Sender{ID: actorID, Username: "participant"},
		Recipient:       model.Sender{ID: counterpartID, Username: "counterpart"},
		ClientMessageID: clientMessageID,
		Text:            text,
		CreatedAt:       time.Now().UTC(),
	}
	r.messages = append(r.messages, message)
	return message, true, nil
}

func (r *repositoryStub) ListMessages(_ context.Context, chainID, actorID, counterpartID, afterID int64, limit int) ([]model.Message, error) {
	if r.firstList != nil {
		r.firstListOnce.Do(func() { close(r.firstList) })
	}
	r.mu.Lock()
	result := make([]model.Message, 0, limit)
	for _, message := range r.messages {
		isThreadMessage := (message.Sender.ID == actorID && message.Recipient.ID == counterpartID) ||
			(message.Sender.ID == counterpartID && message.Recipient.ID == actorID)
		if message.ChainID == chainID && isThreadMessage && message.ID > afterID && len(result) < limit {
			result = append(result, message)
		}
	}
	hook := r.listHook
	r.listHook = nil
	r.mu.Unlock()
	if hook != nil {
		hook()
	}
	return result, nil
}

func (r *repositoryStub) ListThreads(_ context.Context, _ int64) ([]model.Thread, error) {
	return append([]model.Thread(nil), r.threads...), nil
}

func (r *repositoryStub) MarkRead(_ context.Context, chainID, _ int64, counterpartID, lastReadMessageID int64) (model.ReadState, error) {
	return model.ReadState{ChainID: chainID, CounterpartID: counterpartID, LastReadMessageID: lastReadMessageID}, nil
}

func TestThreadsAndReadState(t *testing.T) {
	repository := &repositoryStub{threads: []model.Thread{{
		ChainID:     8,
		Counterpart: model.Sender{ID: 5, Username: "Вера"},
		GiveItem:    &model.ItemSummary{ID: 31, Title: "Велосипед"},
		ReceiveItem: &model.ItemSummary{ID: 44, Title: "Телефон", ImageURL: "/media/phone.jpg"},
		UnreadCount: 2,
	}}}
	chat, _ := New(repository)

	threads, err := chat.ListThreads(context.Background(), 3)
	if err != nil || len(threads) != 1 || threads[0].Counterpart.ID != 5 || threads[0].UnreadCount != 2 ||
		threads[0].GiveItem == nil || threads[0].GiveItem.ID != 31 ||
		threads[0].ReceiveItem == nil || threads[0].ReceiveItem.ImageURL != "/media/phone.jpg" {
		t.Fatalf("threads = %+v, err=%v", threads, err)
	}
	readState, err := chat.MarkRead(context.Background(), 8, 3, 5, 14)
	if err != nil || readState.LastReadMessageID != 14 || readState.CounterpartID != 5 {
		t.Fatalf("read state = %+v, err=%v", readState, err)
	}
	if _, err := chat.MarkRead(context.Background(), 8, 3, 5, 0); err == nil {
		t.Fatal("MarkRead with zero message ID error = nil")
	}
}

func TestSendValidatesAndNormalizesMessage(t *testing.T) {
	repository := &repositoryStub{}
	chat, err := New(repository)
	if err != nil {
		t.Fatalf("new chat: %v", err)
	}

	message, created, err := chat.Send(context.Background(), 8, 3, 5, " client-1 ", "  Привет!  ")
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if !created || message.ClientMessageID != "client-1" || message.Text != "Привет!" {
		t.Fatalf("message = %+v, created=%v", message, created)
	}

	repeated, created, err := chat.Send(context.Background(), 8, 3, 5, "client-1", "Привет!")
	if err != nil || created || repeated.ID != message.ID {
		t.Fatalf("repeat = %+v, created=%v, err=%v", repeated, created, err)
	}
	if _, _, err := chat.Send(context.Background(), 8, 3, 5, "client-1", "Другой текст"); !errors.Is(err, model.ErrIdempotencyConflict) {
		t.Fatalf("idempotency conflict error = %v", err)
	}
}

func TestListReturnsExistingMessagesImmediately(t *testing.T) {
	repository := &repositoryStub{}
	chat, _ := New(repository)
	first, _, _ := chat.Send(context.Background(), 4, 2, 3, "first", "one")
	second, _, _ := chat.Send(context.Background(), 4, 2, 3, "second", "two")

	started := time.Now()
	messages, err := chat.List(context.Background(), 4, 2, 3, first.ID, 10, time.Second)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(messages) != 1 || messages[0].ID != second.ID {
		t.Fatalf("messages = %+v", messages)
	}
	if time.Since(started) > 100*time.Millisecond {
		t.Fatalf("existing messages did not return immediately")
	}
}

func TestListWakesWhenMessageIsCreated(t *testing.T) {
	repository := &repositoryStub{firstList: make(chan struct{})}
	chat, _ := New(repository)
	result := make(chan []model.Message, 1)
	errorsChannel := make(chan error, 1)

	go func() {
		messages, err := chat.List(context.Background(), 7, 1, 2, 0, 10, time.Second)
		result <- messages
		errorsChannel <- err
	}()
	select {
	case <-repository.firstList:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("long poll did not perform its initial database read")
	}
	created, _, err := chat.Send(context.Background(), 7, 2, 1, "wake-up", "message")
	if err != nil {
		t.Fatalf("send: %v", err)
	}

	select {
	case messages := <-result:
		if err := <-errorsChannel; err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(messages) != 1 || messages[0].ID != created.ID {
			t.Fatalf("messages = %+v", messages)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("long poll was not woken")
	}
}

func TestListDoesNotLoseNotificationBeforeWait(t *testing.T) {
	repository := &repositoryStub{}
	chat, _ := New(repository)
	repository.listHook = func() {
		if _, _, err := chat.Send(context.Background(), 11, 2, 1, "between-read-and-wait", "message"); err != nil {
			t.Errorf("send from list hook: %v", err)
		}
	}

	messages, err := chat.List(context.Background(), 11, 1, 2, 0, 10, time.Second)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(messages) != 1 || messages[0].ClientMessageID != "between-read-and-wait" {
		t.Fatalf("messages = %+v", messages)
	}
}

func TestListReturnsEmptyAfterTimeout(t *testing.T) {
	chat, _ := New(&repositoryStub{})
	started := time.Now()
	messages, err := chat.List(context.Background(), 1, 1, 2, 0, 10, 20*time.Millisecond)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(messages) != 0 {
		t.Fatalf("messages = %+v, want empty", messages)
	}
	if time.Since(started) < 15*time.Millisecond {
		t.Fatalf("long poll returned before timeout")
	}
}

func TestListStopsWhenContextIsCanceled(t *testing.T) {
	chat, _ := New(&repositoryStub{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := chat.List(ctx, 1, 1, 2, 0, 10, time.Second)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestChatValidationRejectsInvalidCommands(t *testing.T) {
	chat, _ := New(&repositoryStub{})
	tests := []struct {
		name string
		call func() error
	}{
		{name: "empty text", call: func() error { _, _, err := chat.Send(context.Background(), 1, 1, 2, "message-1", "  "); return err }},
		{name: "invalid client ID", call: func() error { _, _, err := chat.Send(context.Background(), 1, 1, 2, "bad id", "text"); return err }},
		{name: "same counterpart", call: func() error { _, _, err := chat.Send(context.Background(), 1, 1, 1, "message-1", "text"); return err }},
		{name: "negative cursor", call: func() error { _, err := chat.List(context.Background(), 1, 1, 2, -1, 10, 0); return err }},
		{name: "zero limit", call: func() error { _, err := chat.List(context.Background(), 1, 1, 2, 0, 0, 0); return err }},
		{name: "long wait", call: func() error {
			_, err := chat.List(context.Background(), 1, 1, 2, 0, 10, maxWait+time.Second)
			return err
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var validation *model.ValidationError
			if err := test.call(); !errors.As(err, &validation) {
				t.Fatalf("error = %v, want ValidationError", err)
			}
		})
	}
}
