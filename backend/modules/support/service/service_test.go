package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"swap-chain/modules/support/model"
)

type repoStub struct {
	thread   model.Thread
	messages []model.Message
	sendErr  error
	joined   bool
	left     bool
}

func (r *repoStub) UserThread(context.Context, int64) (model.Thread, error) { return r.thread, nil }
func (r *repoStub) AdminThreads(context.Context, int64, int) ([]model.Thread, error) {
	return []model.Thread{r.thread}, nil
}
func (r *repoStub) AdminThread(context.Context, int64, int64) (model.Thread, error) {
	return r.thread, nil
}
func (r *repoStub) ListMessages(_ context.Context, _ int64, _ int64, after int64, _ int, _ bool) ([]model.Message, error) {
	var out []model.Message
	for _, m := range r.messages {
		if m.ID > after {
			out = append(out, m)
		}
	}
	return out, nil
}
func (r *repoStub) Send(_ context.Context, actor, thread int64, client, text string, admin bool) (model.Message, bool, error) {
	if r.sendErr != nil {
		return model.Message{}, false, r.sendErr
	}
	kind := model.SenderUser
	if admin {
		kind = model.SenderModerator
	}
	m := model.Message{ID: int64(len(r.messages) + 1), ThreadID: thread, SenderType: kind, Sender: &model.User{ID: actor}, ClientMessageID: &client, Text: text}
	r.messages = append(r.messages, m)
	return m, true, nil
}
func (r *repoStub) Join(context.Context, int64, int64) (model.Thread, bool, error) {
	r.joined = true
	return r.thread, true, nil
}
func (r *repoStub) Leave(context.Context, int64, int64) (model.Thread, bool, error) {
	r.left = true
	return r.thread, true, nil
}
func (r *repoStub) MarkRead(_ context.Context, _ int64, thread, message int64, _ bool) (model.ReadState, error) {
	return model.ReadState{ThreadID: thread, LastReadMessageID: message}, nil
}

func TestSendValidatesAndNotifiesUser(t *testing.T) {
	repo := &repoStub{thread: model.Thread{ID: 4, User: model.User{ID: 7}}}
	var changedUsers []int64
	var changedThread int64
	svc, _ := New(repo, 100, 25*time.Second, func(users []int64, thread int64) { changedUsers, changedThread = users, thread })
	if _, _, err := svc.Send(context.Background(), 7, 4, "bad id", "hello", false); err == nil {
		t.Fatal("invalid client ID accepted")
	}
	m, created, err := svc.Send(context.Background(), 7, 4, "web-1", "  hello  ", false)
	if err != nil || !created || m.Text != "hello" {
		t.Fatalf("Send()=%+v,%v,%v", m, created, err)
	}
	if len(changedUsers) != 1 || changedUsers[0] != 7 || changedThread != 4 {
		t.Fatalf("changed=%v/%d", changedUsers, changedThread)
	}
}

func TestAdminJoinLeaveAreObservable(t *testing.T) {
	repo := &repoStub{thread: model.Thread{ID: 4, User: model.User{ID: 7}}}
	count := 0
	svc, _ := New(repo, 100, time.Second, func([]int64, int64) { count++ })
	if _, joined, err := svc.Join(context.Background(), 9, 4); err != nil || !joined || !repo.joined {
		t.Fatalf("Join()=%v,%v", joined, err)
	}
	if _, left, err := svc.Leave(context.Background(), 9, 4); err != nil || !left || !repo.left {
		t.Fatalf("Leave()=%v,%v", left, err)
	}
	if count != 2 {
		t.Fatalf("notifications=%d", count)
	}
}

func TestListLongPollWakesOnNewMessage(t *testing.T) {
	repo := &repoStub{thread: model.Thread{ID: 4, User: model.User{ID: 7}}}
	svc, _ := New(repo, 100, time.Second, nil)
	done := make(chan []model.Message, 1)
	go func() {
		messages, _ := svc.List(context.Background(), 7, 4, 0, 50, time.Second, false)
		done <- messages
	}()
	time.Sleep(20 * time.Millisecond)
	_, _, _ = svc.Send(context.Background(), 7, 4, "web-1", "hello", false)
	select {
	case messages := <-done:
		if len(messages) != 1 {
			t.Fatalf("messages=%v", messages)
		}
	case <-time.After(300 * time.Millisecond):
		t.Fatal("long poll did not wake")
	}
}

func TestSendPropagatesIdempotencyConflict(t *testing.T) {
	repo := &repoStub{thread: model.Thread{ID: 4, User: model.User{ID: 7}}, sendErr: model.ErrIdempotencyConflict}
	svc, _ := New(repo, 100, time.Second, nil)
	_, _, err := svc.Send(context.Background(), 7, 4, "web-1", "hello", false)
	if !errors.Is(err, model.ErrIdempotencyConflict) {
		t.Fatalf("error=%v", err)
	}
}
