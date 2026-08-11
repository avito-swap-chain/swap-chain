package outbox

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
)

type workerStoreStub struct {
	events      []Event
	claimErr    error
	markErr     error
	retryErr    error
	markedIDs   []int64
	retriedIDs  []int64
	retryErrors []string
}

func (s *workerStoreStub) Claim(context.Context, string, int, time.Duration) ([]Event, error) {
	return append([]Event(nil), s.events...), s.claimErr
}

func (s *workerStoreStub) MarkPublished(_ context.Context, eventID int64, _ string) error {
	s.markedIDs = append(s.markedIDs, eventID)
	return s.markErr
}

func (s *workerStoreStub) Retry(_ context.Context, eventID int64, _ string, message string, _ time.Duration) error {
	s.retriedIDs = append(s.retriedIDs, eventID)
	s.retryErrors = append(s.retryErrors, message)
	return s.retryErr
}

type publisherStub struct {
	errors    map[int64]error
	published []int64
}

func (p *publisherStub) Publish(_ context.Context, event Event) error {
	p.published = append(p.published, event.ID)
	return p.errors[event.ID]
}

func TestNewWorkerValidatesDependenciesAndConfig(t *testing.T) {
	config := DefaultWorkerConfig()
	publisher := &publisherStub{}
	store := &Store{}
	logger := zap.NewNop()

	for name, create := range map[string]func() (*Worker, error){
		"store":     func() (*Worker, error) { return NewWorker(nil, publisher, logger, config) },
		"publisher": func() (*Worker, error) { return NewWorker(store, nil, logger, config) },
		"logger":    func() (*Worker, error) { return NewWorker(store, publisher, nil, config) },
		"config": func() (*Worker, error) {
			invalid := config
			invalid.BatchSize = 0
			return NewWorker(store, publisher, logger, invalid)
		},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := create(); err == nil {
				t.Fatal("NewWorker() error = nil")
			}
		})
	}

	first, err := NewWorker(store, publisher, logger, config)
	if err != nil {
		t.Fatalf("NewWorker() error = %v", err)
	}
	second, err := NewWorker(store, publisher, logger, config)
	if err != nil {
		t.Fatalf("second NewWorker() error = %v", err)
	}
	if first.workerID == "" || first.workerID == second.workerID {
		t.Fatalf("worker IDs = %q and %q, want distinct non-empty IDs", first.workerID, second.workerID)
	}
}

func TestWorkerDispatchPublishesAndRetriesIndependently(t *testing.T) {
	publishErr := errors.New("publisher unavailable")
	store := &workerStoreStub{events: []Event{{ID: 1}, {ID: 2}}}
	publisher := &publisherStub{errors: map[int64]error{1: publishErr}}
	worker := testWorker(store, publisher)

	if err := worker.dispatch(context.Background()); err != nil {
		t.Fatalf("dispatch() error = %v", err)
	}
	if len(store.retriedIDs) != 1 || store.retriedIDs[0] != 1 || store.retryErrors[0] != publishErr.Error() {
		t.Fatalf("retried = %v errors = %v", store.retriedIDs, store.retryErrors)
	}
	if len(store.markedIDs) != 1 || store.markedIDs[0] != 2 {
		t.Fatalf("marked IDs = %v, want [2]", store.markedIDs)
	}
	if len(publisher.published) != 2 {
		t.Fatalf("published IDs = %v, want both events", publisher.published)
	}
}

func TestWorkerDispatchReportsClaimPublishAndAckFailures(t *testing.T) {
	claimErr := errors.New("claim failed")
	if err := testWorker(&workerStoreStub{claimErr: claimErr}, &publisherStub{}).dispatch(context.Background()); !errors.Is(err, claimErr) {
		t.Fatalf("claim error = %v, want %v", err, claimErr)
	}

	publishErr := errors.New("publish failed")
	retryErr := errors.New("retry failed")
	store := &workerStoreStub{events: []Event{{ID: 3}}, retryErr: retryErr}
	err := testWorker(store, &publisherStub{errors: map[int64]error{3: publishErr}}).dispatch(context.Background())
	if !errors.Is(err, publishErr) || !errors.Is(err, retryErr) {
		t.Fatalf("publish/retry error = %v", err)
	}

	ackErr := errors.New("ack failed")
	store = &workerStoreStub{events: []Event{{ID: 4}}, markErr: ackErr}
	err = testWorker(store, &publisherStub{}).dispatch(context.Background())
	if !errors.Is(err, ackErr) || len(store.retriedIDs) != 1 || !strings.Contains(store.retryErrors[0], "ack failed") {
		t.Fatalf("ack error = %v, retries = %v/%v", err, store.retriedIDs, store.retryErrors)
	}
}

func TestWorkerRunStopsWhenContextIsCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	worker := testWorker(&workerStoreStub{}, &publisherStub{})
	done := make(chan struct{})
	go func() {
		worker.Run(ctx)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run() did not stop after context cancellation")
	}
}

func testWorker(store workerStore, publisher Publisher) *Worker {
	return &Worker{
		store:     store,
		publisher: publisher,
		logger:    zap.NewNop(),
		config: WorkerConfig{
			PollInterval: time.Millisecond,
			RetryDelay:   time.Millisecond,
			Lease:        time.Second,
			BatchSize:    10,
		},
		workerID: "test-worker",
	}
}
