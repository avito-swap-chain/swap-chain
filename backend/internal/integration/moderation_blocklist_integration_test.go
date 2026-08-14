package integration_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/pgvector/pgvector-go"
	"go.uber.org/zap"

	applicationmatching "swap-chain/internal/application/matching"
	"swap-chain/internal/chains"
	"swap-chain/internal/outbox"
	adminmodel "swap-chain/modules/admin/model"
	adminrepository "swap-chain/modules/admin/repository"
	blocklistmodel "swap-chain/modules/blocklist/model"
	blocklistrepository "swap-chain/modules/blocklist/repository"
	matchingrepository "swap-chain/modules/matching/repository"
	matchingservice "swap-chain/modules/matching/service"
	moderationmodel "swap-chain/modules/moderation/model"
	moderationrepository "swap-chain/modules/moderation/repository"
	"swap-chain/shared/db"
)

const migrationBeforeBlocklist = 23

func seedTestUser(t *testing.T, database *sql.DB, suffix int64, role string) int64 {
	t.Helper()
	var userID int64
	query := `INSERT INTO users (username, phone) VALUES ($1, $2) RETURNING id`
	if role == "ADMIN" {
		query = `INSERT INTO users (username, phone, role) VALUES ($1, $2, 'ADMIN') RETURNING id`
	}
	if err := database.QueryRow(query, fmt.Sprintf("mod-user-%d", suffix), fmt.Sprintf("+79%010d", suffix)).Scan(&userID); err != nil {
		t.Fatalf("create user %d: %v", suffix, err)
	}
	return userID
}

func seedMatchingItem(t *testing.T, database *sql.DB, userID, suffix int64, offerCat, wantCat int32, offerVecIndex, wantVecIndex int) int64 {
	t.Helper()
	var itemID int64
	if err := database.QueryRow(`
		INSERT INTO items (
			user_id, offer_title, offer_description,
			offer_category_id, offer_embedding_local, offer_embedding_external,
			status, param_richness, quality_score
		)
		VALUES ($1, $2, 'Описание', $3, $4, $5, 'MATCHING', 1, 1)
		RETURNING id`,
		userID, fmt.Sprintf("mod-item-%d", suffix), offerCat,
		pgvector.NewVector(unitVector(offerVecIndex)), pgvector.NewVector(unitVector(offerVecIndex)),
	).Scan(&itemID); err != nil {
		t.Fatalf("create item %d: %v", suffix, err)
	}
	if _, err := database.Exec(`
		INSERT INTO item_wishes (item_id, want_category_id, want_description, want_embedding_local, want_embedding_external)
		VALUES ($1, $2, 'Пожелание', $3, $4)`,
		itemID, wantCat, pgvector.NewVector(unitVector(wantVecIndex)), pgvector.NewVector(unitVector(wantVecIndex)),
	); err != nil {
		t.Fatalf("create item wish %d: %v", suffix, err)
	}
	return itemID
}

func insertChatMessage(t *testing.T, database *sql.DB, chainID, senderID, recipientID int64, text string) int64 {
	t.Helper()
	var messageID int64
	if err := database.QueryRow(`
		INSERT INTO chat_messages (chain_id, item_id, sender_user_id, recipient_user_id, client_message_id, message_text)
		SELECT $1, owner.item_id, $2, $3, $4, $5
		FROM chain_items AS owner
		JOIN chain_items AS recipient
		  ON recipient.chain_id = owner.chain_id
		 AND recipient.next_item_id = owner.item_id
		WHERE owner.chain_id = $1
		  AND (
		      (owner.user_id = $2 AND recipient.user_id = $3)
		      OR (owner.user_id = $3 AND recipient.user_id = $2)
		  )
		ORDER BY owner.item_id
		LIMIT 1
		RETURNING id`,
		chainID, senderID, recipientID, fmt.Sprintf("client-%d-%d", senderID, time.Now().UnixNano()), text,
	).Scan(&messageID); err != nil {
		t.Fatalf("create chat message: %v", err)
	}
	return messageID
}

func newBlocklistRepository(t *testing.T, database *sql.DB) (*blocklistrepository.PostgreSQL, *chains.PostgresService) {
	t.Helper()
	store, err := outbox.NewStore(database)
	if err != nil {
		t.Fatalf("create outbox store: %v", err)
	}
	chainService := chains.NewPostgresServiceWithOutbox(database, nil, store)
	repository, err := blocklistrepository.NewPostgreSQL(database, chainService)
	if err != nil {
		t.Fatalf("create blocklist repository: %v", err)
	}
	return repository, chainService
}

func TestBlocklistCancelsPendingChainsAndKeepsTerminal(t *testing.T) {
	database := newMigratedDatabase(t, 0)
	ctx := context.Background()
	repository, chainService := newBlocklistRepository(t, database)

	userA := seedTestUser(t, database, 1, "USER")
	userB := seedTestUser(t, database, 2, "USER")
	categoryIDs := loadCategoryIDs(t, database, 2)
	itemA := seedMatchingItem(t, database, userA, 1, categoryIDs[0], categoryIDs[1], 0, 1)
	itemB := seedMatchingItem(t, database, userB, 2, categoryIDs[1], categoryIDs[0], 1, 0)

	pending, err := chainService.Create(ctx, userA, chains.CreateInput{Edges: []chains.Edge{
		{SourceItemID: itemA, TargetItemID: itemB},
		{SourceItemID: itemB, TargetItemID: itemA},
	}})
	if err != nil {
		t.Fatalf("create pending chain: %v", err)
	}

	// Terminal chain with two of its participants blocked must stay untouched.
	terminalChain, terminalUsers, _ := seedDeliveryChain(t, database, adminmodel.ChainAccepted)
	if _, err := repository.Block(ctx, terminalUsers[0], terminalUsers[1]); err != nil {
		t.Fatalf("block terminal users: %v", err)
	}
	var terminalStatus string
	if err := database.QueryRow(`SELECT status::text FROM chains WHERE id = $1`, terminalChain).Scan(&terminalStatus); err != nil {
		t.Fatalf("load terminal chain: %v", err)
	}
	if terminalStatus != adminmodel.ChainAccepted {
		t.Fatalf("terminal chain status = %q, want ACCEPTED", terminalStatus)
	}

	block, err := repository.Block(ctx, userA, userB)
	if err != nil {
		t.Fatalf("block user: %v", err)
	}
	if block.BlockedUser.ID != userB {
		t.Fatalf("blocked user = %d, want %d", block.BlockedUser.ID, userB)
	}

	var status string
	if err := database.QueryRow(`SELECT status::text FROM chains WHERE id = $1`, pending.ID).Scan(&status); err != nil {
		t.Fatalf("load cancelled chain: %v", err)
	}
	if status != chains.StatusRejected {
		t.Fatalf("pending chain status after block = %q, want REJECTED", status)
	}
	var reason string
	if err := database.QueryRow(`SELECT reason FROM chain_rejections WHERE chain_id = $1`, pending.ID).Scan(&reason); err != nil {
		t.Fatalf("load rejection reason: %v", err)
	}
	if reason != "blocked" {
		t.Fatalf("rejection reason = %q, want blocked", reason)
	}

	// Idempotent repeat still succeeds and does not create a second block row.
	if _, err := repository.Block(ctx, userA, userB); err != nil {
		t.Fatalf("repeat block: %v", err)
	}
	var blockCount int
	if err := database.QueryRow(`SELECT count(*) FROM user_blocks WHERE blocker_user_id = $1 AND blocked_user_id = $2`, userA, userB).Scan(&blockCount); err != nil {
		t.Fatalf("count blocks: %v", err)
	}
	if blockCount != 1 {
		t.Fatalf("block rows = %d, want 1", blockCount)
	}

	// Unblock removes the row; unblock of a never-blocked pair is a no-op.
	if err := repository.Unblock(ctx, userA, userB); err != nil {
		t.Fatalf("unblock: %v", err)
	}
	if err := repository.Unblock(ctx, userA, userB); err != nil {
		t.Fatalf("unblock never-blocked: %v", err)
	}
	if err := repository.Unblock(ctx, userA, 9_999_999); !errors.Is(err, blocklistmodel.ErrTargetNotFound) {
		t.Fatalf("unblock missing target error = %v, want ErrTargetNotFound", err)
	}
}

func TestBlocklistExcludesMatchingCandidates(t *testing.T) {
	database := newMigratedDatabase(t, 0)
	ctx := context.Background()
	categoryIDs := loadCategoryIDs(t, database, 3)

	userIDs := make([]int64, 3)
	itemIDs := make([]int64, 3)
	for index := 0; index < 3; index++ {
		userIDs[index] = seedTestUser(t, database, int64(10+index), "USER")
		itemIDs[index] = seedMatchingItem(t, database, userIDs[index], int64(10+index), categoryIDs[index], categoryIDs[(index+1)%3], index, (index+1)%3)
	}

	repository, err := matchingrepository.NewPostgreSQLMatching(db.New(database))
	if err != nil {
		t.Fatalf("create matching repository: %v", err)
	}
	matcher, err := matchingservice.NewMatching(zap.NewNop(), repository, matchingservice.NewScoring(), matchingservice.MatchingConfig{
		SimilarItemsAmount:              20,
		CompatibilityThreshold:          0.5,
		UndefinedCategoryID:             47,
		UndefinedCompatibilityThreshold: 0.6,
		ChainLen:                        3,
		PenaltyFactor:                   0.25,
		ChainRatingThreshold:            0.3,
	})
	if err != nil {
		t.Fatalf("create matcher: %v", err)
	}
	store, err := outbox.NewStore(database)
	if err != nil {
		t.Fatalf("create outbox store: %v", err)
	}
	chainService := chains.NewPostgresServiceWithOutbox(database, nil, store)
	finder := applicationmatching.NewFindCycles(matcher, chainService)

	cycles, err := finder.Execute(ctx, itemIDs[0])
	if err != nil {
		t.Fatalf("find cycles before block: %v", err)
	}
	if len(cycles) != 1 {
		t.Fatalf("cycles before block = %d, want 1", len(cycles))
	}

	blocklistRepo, _ := newBlocklistRepository(t, database)
	if _, err := blocklistRepo.Block(ctx, userIDs[0], userIDs[1]); err != nil {
		t.Fatalf("block candidate: %v", err)
	}

	cycles, err = finder.Execute(ctx, itemIDs[0])
	if err != nil {
		t.Fatalf("find cycles after block: %v", err)
	}
	if len(cycles) != 0 {
		t.Fatalf("cycles after block = %d, want 0", len(cycles))
	}
}

func TestChainCreateRejectsBlockedPair(t *testing.T) {
	database := newMigratedDatabase(t, 0)
	ctx := context.Background()
	categoryIDs := loadCategoryIDs(t, database, 3)
	blocklistRepo, _ := newBlocklistRepository(t, database)
	chainService := chains.NewPostgresService(database, nil)

	// Two-user chain with a block between the participants.
	userA := seedTestUser(t, database, 20, "USER")
	userB := seedTestUser(t, database, 21, "USER")
	itemA := seedMatchingItem(t, database, userA, 20, categoryIDs[0], categoryIDs[1], 0, 1)
	itemB := seedMatchingItem(t, database, userB, 21, categoryIDs[1], categoryIDs[0], 1, 0)
	if _, err := blocklistRepo.Block(ctx, userA, userB); err != nil {
		t.Fatalf("block pair: %v", err)
	}
	if _, err := chainService.Create(ctx, userA, chains.CreateInput{Edges: []chains.Edge{
		{SourceItemID: itemA, TargetItemID: itemB},
		{SourceItemID: itemB, TargetItemID: itemA},
	}}); !errors.Is(err, chains.ErrConflict) {
		t.Fatalf("create 2-user blocked chain error = %v, want ErrConflict", err)
	}

	// Three-user chain with a block between two participants.
	userC := seedTestUser(t, database, 22, "USER")
	itemC := seedMatchingItem(t, database, userC, 22, categoryIDs[2], categoryIDs[0], 2, 0)
	if _, err := chainService.Create(ctx, userB, chains.CreateInput{Edges: []chains.Edge{
		{SourceItemID: itemA, TargetItemID: itemB},
		{SourceItemID: itemB, TargetItemID: itemC},
		{SourceItemID: itemC, TargetItemID: itemA},
	}}); !errors.Is(err, chains.ErrConflict) {
		t.Fatalf("create 3-user blocked chain error = %v, want ErrConflict", err)
	}
}

func TestReportLifecycleAndModerationQueue(t *testing.T) {
	database := newMigratedDatabase(t, 0)
	ctx := context.Background()
	adminID := seedTestUser(t, database, 30, "ADMIN")
	otherAdminID := seedTestUser(t, database, 31, "ADMIN")

	chainID, userIDs, _ := seedDeliveryChain(t, database, adminmodel.ChainAccepted)
	sender := userIDs[0]
	recipient := userIDs[1]
	outlier := userIDs[2]
	outsider := seedTestUser(t, database, 32, "USER")

	messageID := insertChatMessage(t, database, chainID, sender, recipient, "привет, меняемся?")
	moderationRepo, err := moderationrepository.NewPostgreSQL(database)
	if err != nil {
		t.Fatalf("create moderation repository: %v", err)
	}

	// Recipient reports the sender's message.
	report, created, err := moderationRepo.CreateReport(ctx, recipient, messageID, moderationmodel.ReasonSpam, strPtr("спам"))
	if err != nil {
		t.Fatalf("create report: %v", err)
	}
	if !created || report.Status != moderationmodel.ReportOpen || report.Assignee != nil {
		t.Fatalf("created report = %+v", report)
	}

	// Idempotent duplicate returns the same report without mutation.
	duplicate, dupCreated, err := moderationRepo.CreateReport(ctx, recipient, messageID, moderationmodel.ReasonAbuse, strPtr("другой комментарий"))
	if err != nil {
		t.Fatalf("duplicate report: %v", err)
	}
	if dupCreated || duplicate.ID != report.ID || duplicate.Reason != moderationmodel.ReasonSpam {
		t.Fatalf("duplicate report = %+v (created=%t)", duplicate, dupCreated)
	}

	// Self-report is rejected.
	if _, _, err := moderationRepo.CreateReport(ctx, sender, messageID, moderationmodel.ReasonSpam, nil); !errors.Is(err, moderationmodel.ErrSelfReport) {
		t.Fatalf("self-report error = %v, want ErrSelfReport", err)
	}
	// A chain participant outside the thread cannot report (privacy: not found).
	if _, _, err := moderationRepo.CreateReport(ctx, outlier, messageID, moderationmodel.ReasonSpam, nil); !errors.Is(err, moderationmodel.ErrReportUnavailable) {
		t.Fatalf("outlier report error = %v, want ErrReportUnavailable", err)
	}
	// An outsider cannot report.
	if _, _, err := moderationRepo.CreateReport(ctx, outsider, messageID, moderationmodel.ReasonSpam, nil); !errors.Is(err, moderationmodel.ErrReportUnavailable) {
		t.Fatalf("outsider report error = %v, want ErrReportUnavailable", err)
	}
	// A non-existent message is not found.
	if _, _, err := moderationRepo.CreateReport(ctx, recipient, 9_999_999, moderationmodel.ReasonSpam, nil); !errors.Is(err, moderationmodel.ErrNotFound) {
		t.Fatalf("missing message error = %v, want ErrNotFound", err)
	}

	// Admin detail returns report + reported message + full thread context.
	detail, err := moderationRepo.GetReport(ctx, adminID, report.ID)
	if err != nil {
		t.Fatalf("get report: %v", err)
	}
	if detail.ReportedMessage.ID != messageID || len(detail.Context) != 1 {
		t.Fatalf("report detail = %+v", detail)
	}

	// Non-admin cannot read the queue.
	if _, _, err := moderationRepo.ListReports(ctx, recipient, moderationmodel.ReportFilter{}, 0, 10); !errors.Is(err, moderationmodel.ErrForbidden) {
		t.Fatalf("non-admin queue error = %v, want ErrForbidden", err)
	}

	// Unassigned queue filter returns the open report.
	queue, next, err := moderationRepo.ListReports(ctx, adminID, moderationmodel.ReportFilter{Unassigned: true}, 0, 10)
	if err != nil {
		t.Fatalf("list unassigned queue: %v", err)
	}
	if len(queue) != 1 || next != nil {
		t.Fatalf("unassigned queue = %d, next = %v", len(queue), next)
	}

	// Assign to the current admin.
	assigned, err := moderationRepo.Assign(ctx, adminID, report.ID)
	if err != nil {
		t.Fatalf("assign report: %v", err)
	}
	if assigned.Assignee == nil || assigned.Assignee.ID != adminID {
		t.Fatalf("assigned report = %+v", assigned)
	}

	// A second admin cannot steal the assignment.
	if _, err := moderationRepo.Assign(ctx, otherAdminID, report.ID); !errors.Is(err, moderationmodel.ErrAlreadyAssigned) {
		t.Fatalf("second assign error = %v, want ErrAlreadyAssigned", err)
	}
	// The same admin repeats idempotently.
	if _, err := moderationRepo.Assign(ctx, adminID, report.ID); err != nil {
		t.Fatalf("repeat assign: %v", err)
	}
	// A non-assignee cannot decide.
	if _, err := moderationRepo.Decide(ctx, otherAdminID, report.ID, moderationmodel.DecisionResolved, "подтверждено"); !errors.Is(err, moderationmodel.ErrStateConflict) {
		t.Fatalf("non-assignee decide error = %v, want ErrStateConflict", err)
	}

	// Assignee resolves.
	resolved, err := moderationRepo.Decide(ctx, adminID, report.ID, moderationmodel.DecisionResolved, "нарушение подтверждено")
	if err != nil {
		t.Fatalf("resolve report: %v", err)
	}
	if resolved.Status != moderationmodel.ReportResolved || resolved.DecisionComment == nil {
		t.Fatalf("resolved report = %+v", resolved)
	}

	// Terminal report cannot be re-decided or assigned.
	repeat, err := moderationRepo.Decide(ctx, adminID, report.ID, moderationmodel.DecisionRejected, "передумал")
	if err != nil {
		t.Fatalf("repeat decide: %v", err)
	}
	if repeat.Status != moderationmodel.ReportResolved || *repeat.DecisionComment != "нарушение подтверждено" {
		t.Fatalf("repeated decision overwrote state: %+v", repeat)
	}
	if _, err := moderationRepo.Assign(ctx, adminID, report.ID); !errors.Is(err, moderationmodel.ErrStateConflict) {
		t.Fatalf("assign terminal error = %v, want ErrStateConflict", err)
	}

	// Audit log contains exactly the expected entries in order.
	entries, _, err := moderationRepo.ListAudit(ctx, adminID, moderationmodel.AuditFilter{}, nil, 10)
	if err != nil {
		t.Fatalf("list audit: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("audit entries = %d, want 2: %+v", len(entries), entries)
	}
	if entries[0].Action != moderationmodel.ActionReportResolved || entries[1].Action != moderationmodel.ActionReportAssigned {
		t.Fatalf("audit order = %+v", entries)
	}
}

func TestBlockVersusChainCreateRace(t *testing.T) {
	database := newMigratedDatabase(t, 0)
	ctx := context.Background()
	blocklistRepo, chainService := newBlocklistRepository(t, database)
	categoryIDs := loadCategoryIDs(t, database, 2)

	for iteration := 0; iteration < 10; iteration++ {
		userA := seedTestUser(t, database, int64(1000+iteration*2), "USER")
		userB := seedTestUser(t, database, int64(1000+iteration*2+1), "USER")
		itemA := seedMatchingItem(t, database, userA, int64(1000+iteration*2), categoryIDs[0], categoryIDs[1], 0, 1)
		itemB := seedMatchingItem(t, database, userB, int64(1000+iteration*2+1), categoryIDs[1], categoryIDs[0], 1, 0)

		start := make(chan struct{})
		var wg sync.WaitGroup
		var blockErr, createErr error
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			_, blockErr = blocklistRepo.Block(ctx, userA, userB)
		}()
		go func() {
			defer wg.Done()
			<-start
			_, createErr = chainService.Create(ctx, userA, chains.CreateInput{Edges: []chains.Edge{
				{SourceItemID: itemA, TargetItemID: itemB},
				{SourceItemID: itemB, TargetItemID: itemA},
			}})
		}()
		close(start)
		wg.Wait()

		if blockErr != nil {
			t.Fatalf("iteration %d block error: %v", iteration, blockErr)
		}
		if createErr != nil && !errors.Is(createErr, chains.ErrConflict) {
			t.Fatalf("iteration %d create error: %v", iteration, createErr)
		}

		// Invariant: after both operations commit, no PENDING chain contains both users.
		var pending int
		if err := database.QueryRow(`
			SELECT count(*) FROM chains c
			WHERE c.status = 'PENDING'
			  AND EXISTS (SELECT 1 FROM chain_items ci1 WHERE ci1.chain_id = c.id AND ci1.user_id = $1)
			  AND EXISTS (SELECT 1 FROM chain_items ci2 WHERE ci2.chain_id = c.id AND ci2.user_id = $2)`,
			userA, userB,
		).Scan(&pending); err != nil {
			t.Fatalf("iteration %d count pending chains: %v", iteration, err)
		}
		if pending != 0 {
			t.Fatalf("iteration %d: %d PENDING chains survive committed block", iteration, pending)
		}
	}
}

func TestBlockVersusAcceptRace(t *testing.T) {
	database := newMigratedDatabase(t, 0)
	ctx := context.Background()
	blocklistRepo, chainService := newBlocklistRepository(t, database)
	categoryIDs := loadCategoryIDs(t, database, 2)

	for iteration := 0; iteration < 10; iteration++ {
		userA := seedTestUser(t, database, int64(2000+iteration*2), "USER")
		userB := seedTestUser(t, database, int64(2000+iteration*2+1), "USER")
		itemA := seedMatchingItem(t, database, userA, int64(2000+iteration*2), categoryIDs[0], categoryIDs[1], 0, 1)
		itemB := seedMatchingItem(t, database, userB, int64(2000+iteration*2+1), categoryIDs[1], categoryIDs[0], 1, 0)

		pending, err := chainService.Create(ctx, userA, chains.CreateInput{Edges: []chains.Edge{
			{SourceItemID: itemA, TargetItemID: itemB},
			{SourceItemID: itemB, TargetItemID: itemA},
		}})
		if err != nil {
			t.Fatalf("iteration %d create chain: %v", iteration, err)
		}
		if _, err := chainService.Decide(ctx, userA, pending.ID, chains.DecisionApproved); err != nil {
			t.Fatalf("iteration %d first participant approval: %v", iteration, err)
		}

		start := make(chan struct{})
		var wg sync.WaitGroup
		var blockErr, decideErr error
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			_, blockErr = blocklistRepo.Block(ctx, userA, userB)
		}()
		go func() {
			defer wg.Done()
			<-start
			_, decideErr = chainService.Decide(ctx, userB, pending.ID, chains.DecisionApproved)
		}()
		close(start)
		wg.Wait()

		if blockErr != nil {
			t.Fatalf("iteration %d block error: %v", iteration, blockErr)
		}
		if decideErr != nil {
			t.Fatalf("iteration %d decide error: %v", iteration, decideErr)
		}

		var status string
		if err := database.QueryRow(`SELECT status::text FROM chains WHERE id = $1`, pending.ID).Scan(&status); err != nil {
			t.Fatalf("iteration %d load chain: %v", iteration, err)
		}
		if status != chains.StatusAccepted && status != chains.StatusRejected {
			t.Fatalf("iteration %d chain status = %q, want ACCEPTED or REJECTED", iteration, status)
		}
		if status == chains.StatusRejected {
			var reason string
			if err := database.QueryRow(`SELECT reason FROM chain_rejections WHERE chain_id = $1`, pending.ID).Scan(&reason); err != nil {
				t.Fatalf("iteration %d load rejection: %v", iteration, err)
			}
			if reason != "blocked" {
				t.Fatalf("iteration %d rejection reason = %q, want blocked", iteration, reason)
			}
		}
	}
}

func TestTwoAdminsConcurrentAssignYieldsSingleWinner(t *testing.T) {
	database := newMigratedDatabase(t, 0)
	ctx := context.Background()
	adminA := seedTestUser(t, database, 40, "ADMIN")
	adminB := seedTestUser(t, database, 41, "ADMIN")

	chainID, userIDs, _ := seedDeliveryChain(t, database, adminmodel.ChainAccepted)
	messageID := insertChatMessage(t, database, chainID, userIDs[0], userIDs[1], "сообщение для конкурентной жалобы")
	repo, err := moderationrepository.NewPostgreSQL(database)
	if err != nil {
		t.Fatalf("create moderation repository: %v", err)
	}
	report, _, err := repo.CreateReport(ctx, userIDs[1], messageID, moderationmodel.ReasonAbuse, nil)
	if err != nil {
		t.Fatalf("create report: %v", err)
	}

	start := make(chan struct{})
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, adminID := range []int64{adminA, adminB} {
		wg.Add(1)
		go func(id int64) {
			defer wg.Done()
			<-start
			_, err := repo.Assign(ctx, id, report.ID)
			results <- err
		}(adminID)
	}
	close(start)
	wg.Wait()
	close(results)

	successes := 0
	for err := range results {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, moderationmodel.ErrAlreadyAssigned):
		default:
			t.Fatalf("unexpected assign error: %v", err)
		}
	}
	if successes != 1 {
		t.Fatalf("concurrent assign winners = %d, want 1", successes)
	}
	var auditRows int
	if err := database.QueryRow(`SELECT count(*) FROM admin_audit_log WHERE target_id = $1 AND action = 'REPORT_ASSIGNED'`, report.ID).Scan(&auditRows); err != nil {
		t.Fatalf("count assign audit: %v", err)
	}
	if auditRows != 1 {
		t.Fatalf("assign audit rows = %d, want 1", auditRows)
	}
}

func TestAdminDeliveryTransitionWritesAuditAndReceiptDoesNot(t *testing.T) {
	database := newMigratedDatabase(t, 0)
	ctx := context.Background()
	chainID, userIDs, deliveryIDs := seedDeliveryChain(t, database, adminmodel.ChainAccepted)
	var adminID int64
	if err := database.QueryRow(`SELECT id FROM users WHERE role = 'ADMIN' ORDER BY id LIMIT 1`).Scan(&adminID); err != nil {
		t.Fatalf("load admin: %v", err)
	}
	store, err := outbox.NewStore(database)
	if err != nil {
		t.Fatalf("create outbox store: %v", err)
	}
	repository, err := adminrepository.NewPostgreSQLWithOutbox(database, store)
	if err != nil {
		t.Fatalf("create admin repository: %v", err)
	}

	// Admin transition writes exactly one audit row; idempotent repeat does not duplicate.
	if _, err := repository.TransitionDelivery(ctx, adminID, deliveryIDs[0], adminmodel.DeliveryAtPVZ); err != nil {
		t.Fatalf("transition delivery: %v", err)
	}
	if _, err := repository.TransitionDelivery(ctx, adminID, deliveryIDs[0], adminmodel.DeliveryAtPVZ); err != nil {
		t.Fatalf("repeat delivery transition: %v", err)
	}
	var auditRows int
	if err := database.QueryRow(`SELECT count(*) FROM admin_audit_log WHERE target_id = $1 AND action = 'DELIVERY_STATUS_CHANGED'`, deliveryIDs[0]).Scan(&auditRows); err != nil {
		t.Fatalf("count delivery audit: %v", err)
	}
	if auditRows != 1 {
		t.Fatalf("delivery audit rows = %d, want 1", auditRows)
	}

	// Move user[0]'s incoming delivery (deliveryIDs[1]) to IN_DELIVERY so the
	// recipient can confirm receipt, then confirm: the user path must not write audit.
	if _, err := repository.TransitionDelivery(ctx, adminID, deliveryIDs[1], adminmodel.DeliveryAtPVZ); err != nil {
		t.Fatalf("transition incoming delivery to PVZ: %v", err)
	}
	if _, err := repository.TransitionDelivery(ctx, adminID, deliveryIDs[1], adminmodel.DeliveryInTransit); err != nil {
		t.Fatalf("transition incoming delivery in transit: %v", err)
	}
	var beforeReceipt int
	if err := database.QueryRow(`SELECT count(*) FROM admin_audit_log WHERE action = 'DELIVERY_STATUS_CHANGED'`).Scan(&beforeReceipt); err != nil {
		t.Fatalf("count audit before receipt: %v", err)
	}
	if _, err := repository.ConfirmReceipt(ctx, userIDs[0], chainID); err != nil {
		t.Fatalf("confirm receipt: %v", err)
	}
	var afterReceipt int
	if err := database.QueryRow(`SELECT count(*) FROM admin_audit_log WHERE action = 'DELIVERY_STATUS_CHANGED'`).Scan(&afterReceipt); err != nil {
		t.Fatalf("count audit after receipt: %v", err)
	}
	if afterReceipt != beforeReceipt {
		t.Fatalf("receipt wrote audit rows: before=%d after=%d", beforeReceipt, afterReceipt)
	}
}

func TestBlocklistModerationMigrationUpAndDown(t *testing.T) {
	databaseURL, cleanup := newTestDatabase(t)
	t.Cleanup(cleanup)

	migrator := newMigrator(t, databaseURL)
	if err := migrator.Steps(migrationBeforeBlocklist); err != nil {
		t.Fatalf("apply migrations through 23: %v", err)
	}
	closeMigrator(t, migrator)

	database := openDatabase(t, databaseURL)
	var chainID int64
	if err := database.QueryRow(`INSERT INTO chains (status, cycle_key) VALUES ('REJECTED', 'blocklist-migration') RETURNING id`).Scan(&chainID); err != nil {
		t.Fatalf("create rejected chain: %v", err)
	}
	if err := database.Close(); err != nil {
		t.Fatalf("close version 23 database: %v", err)
	}

	migrator = newMigrator(t, databaseURL)
	if err := migrator.Steps(1); err != nil {
		t.Fatalf("apply blocklist migration: %v", err)
	}
	closeMigrator(t, migrator)

	database = openDatabase(t, databaseURL)
	t.Cleanup(func() { _ = database.Close() })
	if _, err := database.Exec(`INSERT INTO chain_rejections (chain_id, reason) VALUES ($1, 'blocked')`, chainID); err != nil {
		t.Fatalf("insert blocked rejection: %v", err)
	}
	for _, table := range []string{"user_blocks", "message_reports", "admin_audit_log"} {
		var exists bool
		if err := database.QueryRow(`SELECT to_regclass($1) IS NOT NULL`, table).Scan(&exists); err != nil {
			t.Fatalf("check table %s: %v", table, err)
		}
		if !exists {
			t.Fatalf("table %s missing after migration", table)
		}
	}

	// Roll back: blocked rejection is normalized to unknown, new tables dropped.
	if err := database.Close(); err != nil {
		t.Fatalf("close migrated database: %v", err)
	}
	migrator = newMigrator(t, databaseURL)
	if err := migrator.Migrate(migrationBeforeBlocklist); err != nil {
		t.Fatalf("roll back blocklist migration: %v", err)
	}
	closeMigrator(t, migrator)

	database = openDatabase(t, databaseURL)
	t.Cleanup(func() { _ = database.Close() })
	var reason string
	if err := database.QueryRow(`SELECT reason FROM chain_rejections WHERE chain_id = $1`, chainID).Scan(&reason); err != nil {
		t.Fatalf("load normalized rejection: %v", err)
	}
	if reason != "unknown" {
		t.Fatalf("rejection reason after rollback = %q, want unknown", reason)
	}
	for _, table := range []string{"user_blocks", "message_reports", "admin_audit_log"} {
		var exists bool
		if err := database.QueryRow(`SELECT to_regclass($1) IS NOT NULL`, table).Scan(&exists); err != nil {
			t.Fatalf("check table %s: %v", table, err)
		}
		if exists {
			t.Fatalf("table %s still exists after rollback", table)
		}
	}
}

func strPtr(value string) *string {
	return &value
}
