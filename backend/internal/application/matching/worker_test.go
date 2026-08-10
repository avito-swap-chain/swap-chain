package matching

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"
)

type jobRepositoryStub struct {
	jobs          []Job
	claimErr      error
	completed     []int64
	retried       []int64
	retryAt       time.Time
	retryError    string
	completeError error
	retryRepoErr  error
}

func (stub *jobRepositoryStub) Claim(context.Context, time.Time, int) ([]Job, error) {
	return stub.jobs, stub.claimErr
}

func (stub *jobRepositoryStub) Complete(_ context.Context, itemID int64) (bool, error) {
	stub.completed = append(stub.completed, itemID)
	return true, stub.completeError
}

func (stub *jobRepositoryStub) Retry(
	_ context.Context,
	itemID int64,
	availableAt time.Time,
	lastError string,
) (bool, error) {
	stub.retried = append(stub.retried, itemID)
	stub.retryAt = availableAt
	stub.retryError = lastError
	return true, stub.retryRepoErr
}

type jobMaterializerStub struct {
	created int
	err     error
	items   []int64
}

func (stub *jobMaterializerStub) Materialize(_ context.Context, itemID int64) (int, error) {
	stub.items = append(stub.items, itemID)
	return stub.created, stub.err
}

func TestMatchingWorkerCompletesSuccessfulJob(t *testing.T) {
	repo := &jobRepositoryStub{jobs: []Job{{ItemID: 5, Attempts: 1}}}
	materializer := &jobMaterializerStub{created: 1}
	worker := newTestWorker(t, repo, materializer)

	if err := worker.runOnce(context.Background()); err != nil {
		t.Fatalf("runOnce() error = %v", err)
	}
	if len(repo.completed) != 1 || repo.completed[0] != 5 {
		t.Fatalf("completed = %v, want [5]", repo.completed)
	}
	if len(repo.retried) != 0 {
		t.Fatalf("retried = %v, want empty", repo.retried)
	}
}

func TestMatchingWorkerRetriesFailedJobWithBackoff(t *testing.T) {
	materializeErr := errors.New("matching failed")
	repo := &jobRepositoryStub{jobs: []Job{{ItemID: 6, Attempts: 3}}}
	materializer := &jobMaterializerStub{err: materializeErr}
	worker := newTestWorker(t, repo, materializer)
	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	worker.now = func() time.Time { return now }

	if err := worker.runOnce(context.Background()); err != nil {
		t.Fatalf("runOnce() error = %v", err)
	}
	if len(repo.retried) != 1 || repo.retried[0] != 6 {
		t.Fatalf("retried = %v, want [6]", repo.retried)
	}
	if !repo.retryAt.Equal(now.Add(20 * time.Second)) {
		t.Fatalf("retry at = %s, want %s", repo.retryAt, now.Add(20*time.Second))
	}
	if repo.retryError != materializeErr.Error() {
		t.Fatalf("retry error = %q, want %q", repo.retryError, materializeErr.Error())
	}
	if len(repo.completed) != 0 {
		t.Fatalf("completed = %v, want empty", repo.completed)
	}
}

func newTestWorker(t *testing.T, repo JobRepository, materializer JobMaterializer) *Worker {
	t.Helper()
	worker, err := NewWorker(repo, materializer, zap.NewNop(), WorkerConfig{
		PollInterval:   time.Second,
		LeaseDuration:  2 * time.Minute,
		AttemptTimeout: time.Minute,
		RetryBaseDelay: 5 * time.Second,
		RetryMaxDelay:  time.Minute,
		BatchSize:      10,
	})
	if err != nil {
		t.Fatalf("NewWorker() error = %v", err)
	}
	return worker
}
