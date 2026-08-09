package chains

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/lib/pq"
)

type recordedEvent struct {
	userIDs   []int64
	eventType string
	entityID  string
	data      map[string]any
}

type eventRecorder struct {
	mu     sync.Mutex
	events []recordedEvent
}

func (r *eventRecorder) publish(userIDs []int64, eventType, entityID string, data map[string]any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, recordedEvent{
		userIDs: append([]int64(nil), userIDs...), eventType: eventType, entityID: entityID, data: data,
	})
}

func TestPostgresServiceConcurrentCompetingChainsIntegration(t *testing.T) {
	database := openIntegrationDatabase(t)
	userIDs, itemIDs := seedMatchingItems(t, database, 3)
	recorder := &eventRecorder{}
	service := NewPostgresService(database, recorder.publish)

	forward := []Edge{
		{SourceItemID: itemIDs[0], TargetItemID: itemIDs[1]},
		{SourceItemID: itemIDs[1], TargetItemID: itemIDs[2]},
		{SourceItemID: itemIDs[2], TargetItemID: itemIDs[0]},
	}
	first, err := service.Create(context.Background(), userIDs[0], CreateInput{Edges: forward})
	if err != nil {
		t.Fatalf("create first chain: %v", err)
	}
	_, forwardKey, err := validateCreate(CreateInput{Edges: forward})
	if err != nil {
		t.Fatalf("canonicalize first chain: %v", err)
	}
	known, err := service.KnownCycleKeys(context.Background(), []string{forwardKey, "missing"})
	if err != nil {
		t.Fatalf("KnownCycleKeys() error = %v", err)
	}
	if len(known) != 1 {
		t.Fatalf("KnownCycleKeys() = %#v, want only %q", known, forwardKey)
	}
	if participantStatus(first, userIDs[0]) != ParticipantApproved || participantStatus(first, userIDs[1]) != ParticipantWaiting {
		t.Fatalf("initial participant statuses = %q/%q", participantStatus(first, userIDs[0]), participantStatus(first, userIDs[1]))
	}
	rotated := []Edge{forward[1], forward[2], forward[0]}
	if _, err := service.Create(context.Background(), userIDs[1], CreateInput{Edges: rotated}); !errors.Is(err, ErrConflict) {
		t.Fatalf("create rotated duplicate error = %v, want ErrConflict", err)
	}

	reverse := []Edge{
		{SourceItemID: itemIDs[0], TargetItemID: itemIDs[2]},
		{SourceItemID: itemIDs[2], TargetItemID: itemIDs[1]},
		{SourceItemID: itemIDs[1], TargetItemID: itemIDs[0]},
	}
	second, err := service.Create(context.Background(), userIDs[1], CreateInput{Edges: reverse})
	if err != nil {
		t.Fatalf("create second chain: %v", err)
	}
	if _, err := service.Decide(context.Background(), userIDs[1], first.ID, DecisionApproved); err != nil {
		t.Fatalf("pre-approve first chain: %v", err)
	}
	if _, err := service.Decide(context.Background(), userIDs[0], second.ID, DecisionApproved); err != nil {
		t.Fatalf("pre-approve second chain: %v", err)
	}

	start := make(chan struct{})
	results := make(chan Chain, 2)
	errors := make(chan error, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		chain, decideErr := service.Decide(context.Background(), userIDs[2], first.ID, DecisionApproved)
		results <- chain
		errors <- decideErr
	}()
	go func() {
		defer wg.Done()
		<-start
		chain, decideErr := service.Decide(context.Background(), userIDs[2], second.ID, DecisionApproved)
		results <- chain
		errors <- decideErr
	}()
	close(start)
	wg.Wait()
	close(results)
	close(errors)

	for err := range errors {
		if err != nil {
			t.Fatalf("concurrent Decide() error = %v", err)
		}
	}
	accepted, rejected := 0, 0
	for chain := range results {
		switch chain.Status {
		case StatusAccepted:
			accepted++
		case StatusRejected:
			rejected++
		default:
			t.Fatalf("concurrent result status = %q", chain.Status)
		}
	}
	if accepted != 1 || rejected != 1 {
		t.Fatalf("accepted/rejected = %d/%d, want 1/1", accepted, rejected)
	}

	var locked int
	if err := database.QueryRow(`SELECT count(*) FROM items WHERE id = ANY($1) AND status = 'LOCKED'`, pq.Array(itemIDs)).Scan(&locked); err != nil {
		t.Fatalf("count locked items: %v", err)
	}
	if locked != len(itemIDs) {
		t.Fatalf("locked item count = %d, want %d", locked, len(itemIDs))
	}

	listed, _, err := service.List(context.Background(), userIDs[0], "", 0, 10)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(listed) != 2 {
		t.Fatalf("listed chains = %d, want 2", len(listed))
	}
	if _, err := service.Get(context.Background(), userIDs[0], first.ID); err != nil {
		t.Fatalf("Get() error = %v", err)
	}
}

func TestPostgresServiceDeclineAndExpiryIntegration(t *testing.T) {
	database := openIntegrationDatabase(t)
	userIDs, itemIDs := seedMatchingItems(t, database, 3)
	recorder := &eventRecorder{}
	service := NewPostgresService(database, recorder.publish)

	edges := []Edge{
		{SourceItemID: itemIDs[0], TargetItemID: itemIDs[1]},
		{SourceItemID: itemIDs[1], TargetItemID: itemIDs[2]},
		{SourceItemID: itemIDs[2], TargetItemID: itemIDs[0]},
	}
	declined, err := service.Create(context.Background(), userIDs[0], CreateInput{Edges: edges})
	if err != nil {
		t.Fatalf("create declined chain: %v", err)
	}
	declined, err = service.Decide(context.Background(), userIDs[1], declined.ID, DecisionDeclined)
	if err != nil {
		t.Fatalf("decline chain: %v", err)
	}
	if declined.Status != StatusRejected || participantStatus(declined, userIDs[1]) != ParticipantDeclined {
		t.Fatalf("declined chain status = %q, participant = %q", declined.Status, participantStatus(declined, userIDs[1]))
	}

	baseTime := time.Now().UTC()
	service.now = func() time.Time { return baseTime }
	reverseEdges := []Edge{
		{SourceItemID: itemIDs[0], TargetItemID: itemIDs[2]},
		{SourceItemID: itemIDs[2], TargetItemID: itemIDs[1]},
		{SourceItemID: itemIDs[1], TargetItemID: itemIDs[0]},
	}
	expiring, err := service.Create(context.Background(), userIDs[0], CreateInput{Edges: reverseEdges})
	if err != nil {
		t.Fatalf("create expiring chain: %v", err)
	}
	service.now = func() time.Time { return baseTime.Add(25 * time.Hour) }
	count, err := service.ExpirePending(context.Background(), 10)
	if err != nil {
		t.Fatalf("ExpirePending() error = %v", err)
	}
	if count != 1 {
		t.Fatalf("expired count = %d, want 1", count)
	}
	expiring, err = service.Get(context.Background(), userIDs[0], expiring.ID)
	if err != nil {
		t.Fatalf("get expired chain: %v", err)
	}
	if expiring.Status != StatusRejected {
		t.Fatalf("expired chain status = %q, want REJECTED", expiring.Status)
	}
}

func openIntegrationDatabase(t *testing.T) *sql.DB {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	database, err := sql.Open("postgres", databaseURL)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
}

func seedMatchingItems(t *testing.T, database *sql.DB, count int) ([]int64, []int64) {
	t.Helper()
	baseID := time.Now().UnixNano()
	userIDs := make([]int64, count)
	itemIDs := make([]int64, count)
	for i := 0; i < count; i++ {
		userIDs[i] = baseID + int64(i)
		phone := fmt.Sprintf("+1%014d", (baseID+int64(i))%100000000000000)
		if _, err := database.Exec(`INSERT INTO users (id, username, phone) VALUES ($1, $2, $3)`, userIDs[i], "chain-test-"+time.Now().Format("150405.000000000"), phone); err != nil {
			t.Fatalf("insert user %d: %v", i, err)
		}
		if err := database.QueryRow(`
			INSERT INTO items (user_id, offer_title, offer_description, want_description, status)
			VALUES ($1, $2, 'description', 'wanted', 'MATCHING') RETURNING id`,
			userIDs[i], "item-"+time.Now().Format("150405.000000000"),
		).Scan(&itemIDs[i]); err != nil {
			t.Fatalf("insert item %d: %v", i, err)
		}
	}
	t.Cleanup(func() {
		_, _ = database.Exec(`DELETE FROM chains WHERE id IN (SELECT chain_id FROM chain_items WHERE user_id = ANY($1))`, pq.Array(userIDs))
		_, _ = database.Exec(`DELETE FROM users WHERE id = ANY($1)`, pq.Array(userIDs))
	})
	return userIDs, itemIDs
}

func participantStatus(chain Chain, userID int64) string {
	for _, participant := range chain.Participants {
		if participant.User.ID == userID {
			return participant.Status
		}
	}
	return ""
}
