package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"swap-chain/modules/support/model"
)

type Repository interface {
	UserThread(context.Context, int64) (model.Thread, error)
	AdminThreads(context.Context, int64, int) ([]model.Thread, error)
	AdminThread(context.Context, int64, int64) (model.Thread, error)
	ListMessages(context.Context, int64, int64, int64, int, bool) ([]model.Message, error)
	Send(context.Context, int64, int64, string, string, bool) (model.Message, bool, error)
	Join(context.Context, int64, int64) (model.Thread, bool, error)
	Leave(context.Context, int64, int64) (model.Thread, bool, error)
	MarkRead(context.Context, int64, int64, int64, bool) (model.ReadState, error)
}

type OnChanged func(userIDs []int64, threadID int64)
type Chat struct {
	repo     Repository
	maxLimit int
	maxWait  time.Duration
	notifier *notifier
	changed  OnChanged
}

func New(repo Repository, maxLimit int, maxWait time.Duration, changed OnChanged) (*Chat, error) {
	if repo == nil {
		return nil, fmt.Errorf("support service: repository is required")
	}
	return &Chat{repo: repo, maxLimit: maxLimit, maxWait: maxWait, notifier: newNotifier(), changed: changed}, nil
}
func (s *Chat) UserThread(ctx context.Context, userID int64) (model.Thread, error) {
	if userID <= 0 {
		return model.Thread{}, model.ErrForbidden
	}
	return s.repo.UserThread(ctx, userID)
}
func (s *Chat) AdminThreads(ctx context.Context, adminID int64, limit int) ([]model.Thread, error) {
	if limit < 1 || limit > s.maxLimit {
		return nil, &model.ValidationError{Field: "limit", Message: fmt.Sprintf("must be between 1 and %d", s.maxLimit)}
	}
	return s.repo.AdminThreads(ctx, adminID, limit)
}
func (s *Chat) AdminThread(ctx context.Context, adminID, threadID int64) (model.Thread, error) {
	if threadID <= 0 {
		return model.Thread{}, &model.ValidationError{Field: "threadId", Message: "must be positive"}
	}
	return s.repo.AdminThread(ctx, adminID, threadID)
}
func (s *Chat) Join(ctx context.Context, adminID, threadID int64) (model.Thread, bool, error) {
	t, joined, err := s.repo.Join(ctx, adminID, threadID)
	if err == nil && joined {
		s.notify(t)
	}
	return t, joined, err
}
func (s *Chat) Leave(ctx context.Context, adminID, threadID int64) (model.Thread, bool, error) {
	t, left, err := s.repo.Leave(ctx, adminID, threadID)
	if err == nil && left {
		s.notify(t)
	}
	return t, left, err
}
func (s *Chat) Send(ctx context.Context, actorID, threadID int64, clientID, text string, admin bool) (model.Message, bool, error) {
	clientID, text, err := validateSend(clientID, text)
	if err != nil {
		return model.Message{}, false, err
	}
	m, created, err := s.repo.Send(ctx, actorID, threadID, clientID, text, admin)
	if err == nil && created {
		var t model.Thread
		var e error
		if admin {
			t, e = s.repo.AdminThread(ctx, actorID, threadID)
		} else {
			t, e = s.repo.UserThread(ctx, actorID)
		}
		if e == nil {
			s.notify(t)
		}
	}
	return m, created, err
}
func (s *Chat) List(ctx context.Context, actorID, threadID, afterID int64, limit int, wait time.Duration, admin bool) ([]model.Message, error) {
	if afterID < 0 {
		return nil, &model.ValidationError{Field: "afterId", Message: "must be non-negative"}
	}
	if limit < 1 || limit > s.maxLimit {
		return nil, &model.ValidationError{Field: "limit", Message: fmt.Sprintf("must be between 1 and %d", s.maxLimit)}
	}
	if wait < 0 || wait > s.maxWait {
		return nil, &model.ValidationError{Field: "waitSeconds", Message: "out of range"}
	}
	ch, off := s.notifier.subscribe(threadID)
	defer off()
	messages, err := s.repo.ListMessages(ctx, actorID, threadID, afterID, limit, admin)
	if err != nil || len(messages) > 0 || wait == 0 {
		return messages, err
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timer.C:
		return []model.Message{}, nil
	case <-ch:
		return s.repo.ListMessages(ctx, actorID, threadID, afterID, limit, admin)
	}
}
func (s *Chat) MarkRead(ctx context.Context, actorID, threadID, messageID int64, admin bool) (model.ReadState, error) {
	if messageID <= 0 {
		return model.ReadState{}, &model.ValidationError{Field: "lastReadMessageId", Message: "must be positive"}
	}
	return s.repo.MarkRead(ctx, actorID, threadID, messageID, admin)
}
func (s *Chat) notify(thread model.Thread) {
	s.notifier.notify(thread.ID)
	if s.changed != nil {
		userIDs := make([]int64, 0, len(thread.Moderators)+1)
		userIDs = append(userIDs, thread.User.ID)
		for _, moderator := range thread.Moderators {
			userIDs = append(userIDs, moderator.ID)
		}
		s.changed(userIDs, thread.ID)
	}
}

var idPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,63}$`)

func validateSend(id, text string) (string, string, error) {
	id = strings.TrimSpace(id)
	text = strings.TrimSpace(text)
	if !idPattern.MatchString(id) {
		return "", "", &model.ValidationError{Field: "clientMessageId", Message: "has invalid format"}
	}
	if text == "" {
		return "", "", &model.ValidationError{Field: "text", Message: "must not be empty"}
	}
	if utf8.RuneCountInString(text) > model.MaxMessageLen {
		return "", "", &model.ValidationError{Field: "text", Message: "must contain at most 2000 characters"}
	}
	return id, text, nil
}

type notifier struct {
	mu   sync.Mutex
	subs map[int64]map[chan struct{}]struct{}
}

func newNotifier() *notifier { return &notifier{subs: map[int64]map[chan struct{}]struct{}{}} }
func (n *notifier) subscribe(id int64) (<-chan struct{}, func()) {
	ch := make(chan struct{})
	n.mu.Lock()
	if n.subs[id] == nil {
		n.subs[id] = map[chan struct{}]struct{}{}
	}
	n.subs[id][ch] = struct{}{}
	n.mu.Unlock()
	return ch, func() {
		n.mu.Lock()
		if _, ok := n.subs[id][ch]; ok {
			delete(n.subs[id], ch)
			close(ch)
		}
		n.mu.Unlock()
	}
}
func (n *notifier) notify(id int64) {
	n.mu.Lock()
	defer n.mu.Unlock()
	for ch := range n.subs[id] {
		delete(n.subs[id], ch)
		close(ch)
	}
	delete(n.subs, id)
}
