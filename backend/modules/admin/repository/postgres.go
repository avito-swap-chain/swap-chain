package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"swap-chain/internal/outbox"
	"swap-chain/modules/admin/model"
	"swap-chain/shared/db"

	"github.com/lib/pq"
)

type outboxEnqueuer interface {
	EnqueueTx(ctx context.Context, tx *sql.Tx, event outbox.PendingEvent) error
}

type PostgreSQL struct {
	database *sql.DB
	queries  *db.Queries
	outbox   outboxEnqueuer
}

func NewPostgreSQL(database *sql.DB) (*PostgreSQL, error) {
	return NewPostgreSQLWithOutbox(database, nil)
}

func NewPostgreSQLWithOutbox(database *sql.DB, eventOutbox outboxEnqueuer) (*PostgreSQL, error) {
	if database == nil {
		return nil, fmt.Errorf("admin repository init: database is required")
	}

	return &PostgreSQL{
		database: database,
		queries:  db.New(database),
		outbox:   eventOutbox,
	}, nil
}

func (r *PostgreSQL) ListChains(ctx context.Context, actorID int64, status string, afterID int64, limit int) ([]model.Chain, *int64, error) {
	if err := requireAdmin(ctx, r.queries, actorID); err != nil {
		return nil, nil, err
	}
	rows, err := r.database.QueryContext(ctx, `
		SELECT c.id, c.status::text, count(ci.id),
		       count(ci.id) FILTER (WHERE ci.delivery_status='RECEIVED'),
		       c.created_at, c.updated_at
		FROM chains c
		JOIN chain_items ci ON ci.chain_id=c.id
		WHERE c.status IN ('ACCEPTED','COMPLETED')
		  AND ($1='' OR c.status::text=$1) AND c.id>$2
		GROUP BY c.id
		ORDER BY c.id ASC LIMIT $3`, status, afterID, limit+1)
	if err != nil {
		return nil, nil, fmt.Errorf("list admin chains: %w", err)
	}
	defer rows.Close()
	chains := make([]model.Chain, 0, limit+1)
	for rows.Next() {
		var chain model.Chain
		if err := rows.Scan(&chain.ID, &chain.Status, &chain.ParticipantCount, &chain.ReceivedCount, &chain.CreatedAt, &chain.UpdatedAt); err != nil {
			return nil, nil, fmt.Errorf("scan admin chain: %w", err)
		}
		chains = append(chains, chain)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterate admin chains: %w", err)
	}
	var next *int64
	if len(chains) > limit {
		cursor := chains[limit-1].ID
		next = &cursor
		chains = chains[:limit]
	}
	return chains, next, nil
}

func (r *PostgreSQL) GetChain(ctx context.Context, actorID, chainID int64) (model.Chain, error) {
	if err := requireAdmin(ctx, r.queries, actorID); err != nil {
		return model.Chain{}, err
	}
	var chain model.Chain
	err := r.database.QueryRowContext(ctx, `
		SELECT c.id,c.status::text,count(ci.id),
		       count(ci.id) FILTER (WHERE ci.delivery_status='RECEIVED'),c.created_at,c.updated_at
		FROM chains c JOIN chain_items ci ON ci.chain_id=c.id
		WHERE c.id=$1 AND c.status IN ('ACCEPTED','COMPLETED') GROUP BY c.id`, chainID).
		Scan(&chain.ID, &chain.Status, &chain.ParticipantCount, &chain.ReceivedCount, &chain.CreatedAt, &chain.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Chain{}, model.ErrChainNotFound
	}
	if err != nil {
		return model.Chain{}, fmt.Errorf("load admin chain: %w", err)
	}
	rows, err := r.database.QueryContext(ctx, `
		SELECT ci.id,ci.chain_id,ci.item_id,i.offer_title,
		       sender.id,sender.username,recipient.id,recipient.username,
		       ci.delivery_status::text,ci.delivery_updated_at
		FROM chain_items ci
		JOIN items i ON i.id=ci.item_id
		JOIN users sender ON sender.id=ci.user_id
		JOIN chain_items incoming ON incoming.chain_id=ci.chain_id AND incoming.next_item_id=ci.item_id
		JOIN users recipient ON recipient.id=incoming.user_id
		WHERE ci.chain_id=$1 ORDER BY ci.id`, chainID)
	if err != nil {
		return model.Chain{}, fmt.Errorf("load admin chain deliveries: %w", err)
	}
	defer rows.Close()
	chain.Deliveries = make([]model.Delivery, 0, chain.ParticipantCount)
	for rows.Next() {
		var delivery model.Delivery
		if err := rows.Scan(&delivery.ID, &delivery.ChainID, &delivery.ItemID, &delivery.ItemTitle,
			&delivery.SenderID, &delivery.SenderUsername, &delivery.RecipientID, &delivery.RecipientUsername,
			&delivery.Status, &delivery.UpdatedAt); err != nil {
			return model.Chain{}, fmt.Errorf("scan admin chain delivery: %w", err)
		}
		chain.Deliveries = append(chain.Deliveries, delivery)
	}
	if err := rows.Err(); err != nil {
		return model.Chain{}, fmt.Errorf("iterate admin chain deliveries: %w", err)
	}
	return chain, nil
}

func (r *PostgreSQL) ListDeliveries(
	ctx context.Context,
	actorID int64,
	status string,
	afterID int64,
	limit int,
) ([]model.Delivery, *int64, error) {
	if err := requireAdmin(ctx, r.queries, actorID); err != nil {
		return nil, nil, err
	}

	rows, err := r.queries.ListAdminDeliveries(ctx, db.ListAdminDeliveriesParams{
		AfterID:        afterID,
		DeliveryStatus: status,
		PageSize:       int32(limit + 1),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("list admin deliveries: %w", err)
	}

	deliveries := make([]model.Delivery, 0, len(rows))
	for _, row := range rows {
		deliveries = append(deliveries, mapListedDelivery(row))
	}

	var next *int64
	if len(deliveries) > limit {
		cursor := deliveries[limit-1].ID
		next = &cursor
		deliveries = deliveries[:limit]
	}

	return deliveries, next, nil
}

func (r *PostgreSQL) TransitionDelivery(
	ctx context.Context,
	actorID int64,
	deliveryID int64,
	targetStatus string,
) (model.Delivery, error) {
	tx, err := r.database.BeginTx(ctx, nil)
	if err != nil {
		return model.Delivery{}, fmt.Errorf("begin admin delivery transition: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	queries := db.New(tx)
	if err := requireAdmin(ctx, queries, actorID); err != nil {
		return model.Delivery{}, err
	}

	chainID, err := findDeliveryChain(ctx, queries, deliveryID)
	if err != nil {
		return model.Delivery{}, err
	}
	chainStatus, err := lockChain(ctx, queries, chainID)
	if err != nil {
		return model.Delivery{}, err
	}
	currentStatus, itemStatus, err := lockDelivery(ctx, queries, deliveryID)
	if err != nil {
		return model.Delivery{}, err
	}
	if !canApplyTransition(chainStatus, itemStatus, currentStatus, targetStatus) {
		return model.Delivery{}, model.ErrTransitionConflict
	}

	changed := currentStatus != targetStatus
	if changed {
		if err := updateDelivery(ctx, queries, actorID, deliveryID, currentStatus, targetStatus); err != nil {
			return model.Delivery{}, err
		}
		if err := recordDeliveryAudit(ctx, queries, actorID, deliveryID, currentStatus, targetStatus); err != nil {
			return model.Delivery{}, err
		}
	}
	if targetStatus == model.DeliveryReceived {
		chainStatus, err = completeChainIfReceived(ctx, queries, chainID, chainStatus)
		if err != nil {
			return model.Delivery{}, err
		}
	}

	delivery, err := loadDelivery(ctx, queries, deliveryID)
	if err != nil {
		return model.Delivery{}, err
	}
	if changed {
		if err := r.enqueueDeliveryEvent(ctx, tx, chainStatus, delivery); err != nil {
			return model.Delivery{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return model.Delivery{}, fmt.Errorf("commit admin delivery transition: %w", err)
	}

	return delivery, nil
}

// ConfirmReceipt определяет входящую вещь по участнику и атомарно завершает цепочку после последнего получения.
func (r *PostgreSQL) ConfirmReceipt(ctx context.Context, actorID, chainID int64) (model.Receipt, error) {
	return r.confirmReceipt(ctx, actorID, actorID, chainID, false)
}

func (r *PostgreSQL) ConfirmParticipantReceipt(ctx context.Context, actorID, chainID, participantID int64) (model.Receipt, error) {
	return r.confirmReceipt(ctx, actorID, participantID, chainID, true)
}

func (r *PostgreSQL) confirmReceipt(ctx context.Context, actorID, recipientID, chainID int64, admin bool) (model.Receipt, error) {
	tx, err := r.database.BeginTx(ctx, nil)
	if err != nil {
		return model.Receipt{}, fmt.Errorf("begin delivery receipt: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	queries := db.New(tx)
	if admin {
		if err := requireAdmin(ctx, queries, actorID); err != nil {
			return model.Receipt{}, err
		}
	}
	chainStatus, err := lockChain(ctx, queries, chainID)
	if err != nil {
		return model.Receipt{}, err
	}

	recipientDelivery, err := queries.LockRecipientDelivery(ctx, db.LockRecipientDeliveryParams{
		ChainID:     chainID,
		RecipientID: recipientID,
	})
	switch {
	case errors.Is(err, sql.ErrNoRows):
		if admin {
			return model.Receipt{}, model.ErrDeliveryNotFound
		}
		return model.Receipt{}, model.ErrReceiptForbidden
	case err != nil:
		return model.Receipt{}, fmt.Errorf("lock recipient delivery: %w", err)
	}
	if !canApplyTransition(chainStatus, recipientDelivery.ItemStatus, recipientDelivery.DeliveryStatus, model.DeliveryReceived) {
		return model.Receipt{}, model.ErrTransitionConflict
	}

	changed := recipientDelivery.DeliveryStatus != model.DeliveryReceived
	if changed {
		if err := updateDelivery(
			ctx,
			queries,
			actorID,
			recipientDelivery.ID,
			recipientDelivery.DeliveryStatus,
			model.DeliveryReceived,
		); err != nil {
			return model.Receipt{}, err
		}
		if admin {
			if err := recordDeliveryAudit(ctx, queries, actorID, recipientDelivery.ID, recipientDelivery.DeliveryStatus, model.DeliveryReceived); err != nil {
				return model.Receipt{}, err
			}
		}
	}
	chainStatus, err = completeChainIfReceived(ctx, queries, chainID, chainStatus)
	if err != nil {
		return model.Receipt{}, err
	}
	delivery, err := loadDelivery(ctx, queries, recipientDelivery.ID)
	if err != nil {
		return model.Receipt{}, err
	}
	if changed {
		if err := r.enqueueDeliveryEvent(ctx, tx, chainStatus, delivery); err != nil {
			return model.Receipt{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return model.Receipt{}, fmt.Errorf("commit delivery receipt: %w", err)
	}

	return model.Receipt{Delivery: delivery, ChainStatus: chainStatus}, nil
}

func (r *PostgreSQL) enqueueDeliveryEvent(ctx context.Context, tx *sql.Tx, chainStatus string, delivery model.Delivery) error {
	if r.outbox == nil {
		return nil
	}

	var recipients pq.Int64Array
	if err := tx.QueryRowContext(ctx, `
		SELECT array_agg(DISTINCT user_id ORDER BY user_id)
		FROM chain_items
		WHERE chain_id = $1`, delivery.ChainID).Scan(&recipients); err != nil {
		return fmt.Errorf("load delivery event recipients: %w", err)
	}
	if len(recipients) == 0 {
		return fmt.Errorf("load delivery event recipients: chain has no participants")
	}

	if err := r.outbox.EnqueueTx(ctx, tx, outbox.PendingEvent{
		DeduplicationKey: fmt.Sprintf("delivery:%d:%s", delivery.ID, delivery.Status),
		Type:             "chain.updated",
		EntityID:         strconv.FormatInt(delivery.ChainID, 10),
		RecipientIDs:     []int64(recipients),
		Data: map[string]any{
			"deliveryId":     delivery.ID,
			"deliveryStatus": delivery.Status,
			"chainStatus":    chainStatus,
		},
	}); err != nil {
		return fmt.Errorf("enqueue delivery event: %w", err)
	}
	return nil
}

func findDeliveryChain(ctx context.Context, queries *db.Queries, deliveryID int64) (int64, error) {
	chainID, err := queries.FindAdminDeliveryChain(ctx, deliveryID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return 0, model.ErrDeliveryNotFound
	case err != nil:
		return 0, fmt.Errorf("find delivery chain: %w", err)
	default:
		return chainID, nil
	}
}

func lockChain(ctx context.Context, queries *db.Queries, chainID int64) (string, error) {
	status, err := queries.LockAdminChain(ctx, chainID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return "", model.ErrChainNotFound
	case err != nil:
		return "", fmt.Errorf("lock delivery chain: %w", err)
	default:
		return status, nil
	}
}

func lockDelivery(ctx context.Context, queries *db.Queries, deliveryID int64) (string, string, error) {
	delivery, err := queries.LockAdminDelivery(ctx, deliveryID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return "", "", model.ErrDeliveryNotFound
	case err != nil:
		return "", "", fmt.Errorf("lock delivery: %w", err)
	default:
		return delivery.DeliveryStatus, delivery.ItemStatus, nil
	}
}

func canApplyTransition(chainStatus, itemStatus, currentStatus, targetStatus string) bool {
	if chainStatus == model.ChainCompleted {
		return itemStatus == "EXCHANGED" &&
			currentStatus == model.DeliveryReceived &&
			targetStatus == model.DeliveryReceived
	}

	return chainStatus == model.ChainAccepted &&
		itemStatus == "LOCKED" &&
		model.CanTransition(currentStatus, targetStatus)
}

func updateDelivery(
	ctx context.Context,
	queries *db.Queries,
	actorID int64,
	deliveryID int64,
	currentStatus string,
	targetStatus string,
) error {
	err := queries.UpdateAdminDeliveryStatus(ctx, db.UpdateAdminDeliveryStatusParams{
		DeliveryStatus: db.DeliveryStatus(targetStatus),
		DeliveryID:     deliveryID,
	})
	if err != nil {
		return fmt.Errorf("update delivery: %w", err)
	}

	err = queries.CreateAdminDeliveryEvent(ctx, db.CreateAdminDeliveryEventParams{
		DeliveryID: deliveryID,
		ActorID:    actorID,
		FromStatus: db.DeliveryStatus(currentStatus),
		ToStatus:   db.DeliveryStatus(targetStatus),
	})
	if err != nil {
		return fmt.Errorf("record delivery event: %w", err)
	}

	return nil
}

// recordDeliveryAudit writes a DELIVERY_STATUS_CHANGED entry into the
// append-only admin audit log within the current transaction. Only admin
// delivery transitions call it; the recipient receipt path does not.
func recordDeliveryAudit(
	ctx context.Context,
	queries *db.Queries,
	actorID int64,
	deliveryID int64,
	fromStatus string,
	toStatus string,
) error {
	raw, err := json.Marshal(map[string]any{
		"deliveryId": deliveryID,
		"fromStatus": fromStatus,
		"toStatus":   toStatus,
	})
	if err != nil {
		return fmt.Errorf("marshal delivery audit metadata: %w", err)
	}
	if err := queries.InsertAuditLog(ctx, db.InsertAuditLogParams{
		AdminID:    actorID,
		Action:     db.AuditAction(model.AuditDeliveryStatusChanged),
		TargetType: "delivery",
		TargetID:   deliveryID,
		Metadata:   raw,
	}); err != nil {
		return fmt.Errorf("insert delivery audit log: %w", err)
	}
	return nil
}

func completeChainIfReceived(
	ctx context.Context,
	queries *db.Queries,
	chainID int64,
	chainStatus string,
) (string, error) {
	if chainStatus == model.ChainCompleted {
		return chainStatus, nil
	}

	remaining, err := queries.CountUnreceivedAdminDeliveries(ctx, chainID)
	if err != nil {
		return "", fmt.Errorf("count unreceived deliveries: %w", err)
	}
	if remaining > 0 {
		return chainStatus, nil
	}

	updated, err := queries.CompleteAdminChain(ctx, chainID)
	if err != nil {
		return "", fmt.Errorf("complete delivery chain: %w", err)
	}
	if updated != 1 {
		return "", fmt.Errorf("complete delivery chain: unexpected affected rows count %d", updated)
	}
	exchanged, err := queries.ExchangeAdminChainItems(ctx, chainID)
	if err != nil {
		return "", fmt.Errorf("exchange completed chain items: %w", err)
	}
	if exchanged.ItemCount == 0 || exchanged.ExchangedCount != exchanged.ItemCount {
		return "", fmt.Errorf(
			"exchange completed chain items: exchanged %d of %d",
			exchanged.ExchangedCount,
			exchanged.ItemCount,
		)
	}

	return model.ChainCompleted, nil
}

func requireAdmin(ctx context.Context, queries *db.Queries, actorID int64) error {
	isAdmin, err := queries.IsAdminUser(ctx, actorID)
	if errors.Is(err, sql.ErrNoRows) || err == nil && !isAdmin {
		return model.ErrForbidden
	}
	if err != nil {
		return fmt.Errorf("authorize admin: %w", err)
	}

	return nil
}

func loadDelivery(ctx context.Context, queries *db.Queries, deliveryID int64) (model.Delivery, error) {
	row, err := queries.GetAdminDelivery(ctx, deliveryID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return model.Delivery{}, model.ErrDeliveryNotFound
	case err != nil:
		return model.Delivery{}, fmt.Errorf("load admin delivery: %w", err)
	default:
		return mapDelivery(row), nil
	}
}

func mapListedDelivery(row db.ListAdminDeliveriesRow) model.Delivery {
	return model.Delivery{
		ID:                row.ID,
		ChainID:           row.ChainID,
		ItemID:            row.ItemID,
		ItemTitle:         row.ItemTitle,
		SenderID:          row.SenderID,
		SenderUsername:    row.SenderUsername,
		RecipientID:       row.RecipientID,
		RecipientUsername: row.RecipientUsername,
		Status:            row.DeliveryStatus,
		UpdatedAt:         row.DeliveryUpdatedAt,
	}
}

func mapDelivery(row db.GetAdminDeliveryRow) model.Delivery {
	return model.Delivery{
		ID:                row.ID,
		ChainID:           row.ChainID,
		ItemID:            row.ItemID,
		ItemTitle:         row.ItemTitle,
		SenderID:          row.SenderID,
		SenderUsername:    row.SenderUsername,
		RecipientID:       row.RecipientID,
		RecipientUsername: row.RecipientUsername,
		Status:            row.DeliveryStatus,
		UpdatedAt:         row.DeliveryUpdatedAt,
	}
}
