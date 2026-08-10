package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"swap-chain/modules/chat/model"
)

const (
	maxListLimit = 100
	maxWait      = 25 * time.Second
)

var clientMessageIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,63}$`)

type Repository interface {
	CreateMessage(ctx context.Context, chainID, actorID, counterpartID int64, clientMessageID, text string) (message model.Message, created bool, err error)
	ListMessages(ctx context.Context, chainID, actorID, counterpartID, afterID int64, limit int) ([]model.Message, error)
	ListThreads(ctx context.Context, actorID int64) ([]model.Thread, error)
	MarkRead(ctx context.Context, chainID, actorID, counterpartID, lastReadMessageID int64) (model.ReadState, error)
}

type Service interface {
	Send(ctx context.Context, chainID, actorID, counterpartID int64, clientMessageID, text string) (message model.Message, created bool, err error)
	List(ctx context.Context, chainID, actorID, counterpartID, afterID int64, limit int, wait time.Duration) ([]model.Message, error)
	ListThreads(ctx context.Context, actorID int64) ([]model.Thread, error)
	MarkRead(ctx context.Context, chainID, actorID, counterpartID, lastReadMessageID int64) (model.ReadState, error)
}

type Chat struct {
	repository Repository
	notifier   *notifier
}

func New(repository Repository) (*Chat, error) {
	if repository == nil {
		return nil, fmt.Errorf("chat init: 'repository' is required")
	}
	return &Chat{repository: repository, notifier: newNotifier()}, nil
}

func (s *Chat) Send(ctx context.Context, chainID, actorID, counterpartID int64, clientMessageID, text string) (model.Message, bool, error) {
	clientMessageID, text, err := validateSend(chainID, actorID, counterpartID, clientMessageID, text)
	if err != nil {
		return model.Message{}, false, err
	}

	message, created, err := s.repository.CreateMessage(ctx, chainID, actorID, counterpartID, clientMessageID, text)
	if err != nil {
		return model.Message{}, false, err
	}
	if created {
		s.notifier.Notify(newThreadKey(chainID, actorID, counterpartID))
	}
	return message, created, nil
}

// List подписывается до первого чтения, чтобы не потерять сообщение между чтением БД и ожиданием.
func (s *Chat) List(ctx context.Context, chainID, actorID, counterpartID, afterID int64, limit int, wait time.Duration) ([]model.Message, error) {
	if err := validateList(chainID, actorID, counterpartID, afterID, limit, wait); err != nil {
		return nil, err
	}

	thread := newThreadKey(chainID, actorID, counterpartID)
	wakeUp, unsubscribe := s.notifier.Subscribe(thread)
	defer unsubscribe()

	messages, err := s.repository.ListMessages(ctx, chainID, actorID, counterpartID, afterID, limit)
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
	case <-wakeUp:
		return s.repository.ListMessages(ctx, chainID, actorID, counterpartID, afterID, limit)
	}
}

func (s *Chat) ListThreads(ctx context.Context, actorID int64) ([]model.Thread, error) {
	if actorID <= 0 {
		return nil, model.ErrForbidden
	}
	return s.repository.ListThreads(ctx, actorID)
}

func (s *Chat) MarkRead(ctx context.Context, chainID, actorID, counterpartID, lastReadMessageID int64) (model.ReadState, error) {
	if err := validateThread(chainID, actorID, counterpartID); err != nil {
		return model.ReadState{}, err
	}
	if lastReadMessageID <= 0 {
		return model.ReadState{}, &model.ValidationError{Field: "lastReadMessageId", Message: "must be positive"}
	}
	return s.repository.MarkRead(ctx, chainID, actorID, counterpartID, lastReadMessageID)
}

func validateSend(chainID, actorID, counterpartID int64, clientMessageID, text string) (string, string, error) {
	if err := validateThread(chainID, actorID, counterpartID); err != nil {
		return "", "", err
	}

	clientMessageID = strings.TrimSpace(clientMessageID)
	if !clientMessageIDPattern.MatchString(clientMessageID) {
		return "", "", &model.ValidationError{Field: "clientMessageId", Message: "must contain 1 to 64 letters, digits, dots, colons, underscores or hyphens"}
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return "", "", &model.ValidationError{Field: "text", Message: "must not be empty"}
	}
	if utf8.RuneCountInString(text) > model.MaxMessageLength {
		return "", "", &model.ValidationError{Field: "text", Message: "must contain at most 2000 characters"}
	}
	return clientMessageID, text, nil
}

func validateThread(chainID, actorID, counterpartID int64) error {
	if chainID <= 0 {
		return &model.ValidationError{Field: "chainId", Message: "must be positive"}
	}
	if actorID <= 0 {
		return model.ErrForbidden
	}
	if counterpartID <= 0 {
		return &model.ValidationError{Field: "counterpartId", Message: "must be positive"}
	}
	if actorID == counterpartID {
		return &model.ValidationError{Field: "counterpartId", Message: "must identify another user"}
	}
	return nil
}

func validateList(chainID, actorID, counterpartID, afterID int64, limit int, wait time.Duration) error {
	if err := validateThread(chainID, actorID, counterpartID); err != nil {
		return err
	}
	switch {
	case afterID < 0:
		return &model.ValidationError{Field: "afterId", Message: "must be non-negative"}
	case limit < 1 || limit > maxListLimit:
		return &model.ValidationError{Field: "limit", Message: "must be between 1 and 100"}
	case wait < 0 || wait > maxWait:
		return &model.ValidationError{Field: "waitSeconds", Message: "must be between 0 and 25 seconds"}
	default:
		return nil
	}
}

type threadKey struct {
	chainID  int64
	firstID  int64
	secondID int64
}

func newThreadKey(chainID, firstID, secondID int64) threadKey {
	if firstID > secondID {
		firstID, secondID = secondID, firstID
	}
	return threadKey{chainID: chainID, firstID: firstID, secondID: secondID}
}

type notifier struct {
	mu          sync.Mutex
	subscribers map[threadKey]map[chan struct{}]struct{}
}

func newNotifier() *notifier {
	return &notifier{subscribers: make(map[threadKey]map[chan struct{}]struct{})}
}

func (n *notifier) Subscribe(thread threadKey) (<-chan struct{}, func()) {
	channel := make(chan struct{})
	n.mu.Lock()
	if n.subscribers[thread] == nil {
		n.subscribers[thread] = make(map[chan struct{}]struct{})
	}
	n.subscribers[thread][channel] = struct{}{}
	n.mu.Unlock()

	return channel, func() {
		n.mu.Lock()
		defer n.mu.Unlock()
		if _, exists := n.subscribers[thread][channel]; !exists {
			return
		}
		delete(n.subscribers[thread], channel)
		close(channel)
		if len(n.subscribers[thread]) == 0 {
			delete(n.subscribers, thread)
		}
	}
}

func (n *notifier) Notify(thread threadKey) {
	n.mu.Lock()
	defer n.mu.Unlock()
	for channel := range n.subscribers[thread] {
		delete(n.subscribers[thread], channel)
		close(channel)
	}
	delete(n.subscribers, thread)
}
