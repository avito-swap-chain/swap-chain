package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"swap-chain/modules/admin/model"
	"swap-chain/shared/db"
)

type PostgreSQL struct {
	database *sql.DB
	queries  *db.Queries
}

func NewPostgreSQL(database *sql.DB) (*PostgreSQL, error) {
	if database == nil {
		return nil, fmt.Errorf("admin repository init: database is required")
	}

	return &PostgreSQL{
		database: database,
		queries:  db.New(database),
	}, nil
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

	if currentStatus != targetStatus {
		if err := updateDelivery(ctx, queries, actorID, deliveryID, currentStatus, targetStatus); err != nil {
			return model.Delivery{}, err
		}
	}
	if targetStatus == model.DeliveryReceived {
		if _, err := completeChainIfReceived(ctx, queries, chainID, chainStatus); err != nil {
			return model.Delivery{}, err
		}
	}

	delivery, err := loadDelivery(ctx, queries, deliveryID)
	if err != nil {
		return model.Delivery{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.Delivery{}, fmt.Errorf("commit admin delivery transition: %w", err)
	}

	return delivery, nil
}

// ConfirmReceipt определяет входящую вещь по участнику и атомарно завершает цепочку после последнего получения.
func (r *PostgreSQL) ConfirmReceipt(ctx context.Context, actorID, chainID int64) (model.Receipt, error) {
	tx, err := r.database.BeginTx(ctx, nil)
	if err != nil {
		return model.Receipt{}, fmt.Errorf("begin delivery receipt: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	queries := db.New(tx)
	chainStatus, err := lockChain(ctx, queries, chainID)
	if err != nil {
		return model.Receipt{}, err
	}

	recipientDelivery, err := queries.LockRecipientDelivery(ctx, db.LockRecipientDeliveryParams{
		ChainID:     chainID,
		RecipientID: actorID,
	})
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return model.Receipt{}, model.ErrReceiptForbidden
	case err != nil:
		return model.Receipt{}, fmt.Errorf("lock recipient delivery: %w", err)
	}
	if !canApplyTransition(chainStatus, recipientDelivery.ItemStatus, recipientDelivery.DeliveryStatus, model.DeliveryReceived) {
		return model.Receipt{}, model.ErrTransitionConflict
	}

	if recipientDelivery.DeliveryStatus != model.DeliveryReceived {
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
	}
	chainStatus, err = completeChainIfReceived(ctx, queries, chainID, chainStatus)
	if err != nil {
		return model.Receipt{}, err
	}

	delivery, err := loadDelivery(ctx, queries, recipientDelivery.ID)
	if err != nil {
		return model.Receipt{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.Receipt{}, fmt.Errorf("commit delivery receipt: %w", err)
	}

	return model.Receipt{Delivery: delivery, ChainStatus: chainStatus}, nil
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
	if itemStatus != "LOCKED" || !model.CanTransition(currentStatus, targetStatus) {
		return false
	}
	if chainStatus == model.ChainAccepted {
		return true
	}

	return chainStatus == model.ChainCompleted &&
		currentStatus == model.DeliveryReceived &&
		targetStatus == model.DeliveryReceived
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
