package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"swap-chain/modules/metrics/model"
)

type PostgreSQL struct {
	database *sql.DB
}

func NewPostgreSQL(database *sql.DB) (*PostgreSQL, error) {
	if database == nil {
		return nil, fmt.Errorf("metrics repository: database is required")
	}
	return &PostgreSQL{database: database}, nil
}

func (r *PostgreSQL) Funnel(ctx context.Context, actorID int64) (model.Funnel, error) {
	tx, err := r.database.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return model.Funnel{}, fmt.Errorf("begin funnel metrics snapshot: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var role string
	if err := tx.QueryRowContext(ctx, `SELECT role::text FROM users WHERE id = $1`, actorID).Scan(&role); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Funnel{}, model.ErrForbidden
		}
		return model.Funnel{}, fmt.Errorf("authorize funnel metrics: %w", err)
	}
	if role != "ADMIN" {
		return model.Funnel{}, model.ErrForbidden
	}

	var result model.Funnel
	var averageTime sql.NullFloat64
	if err := tx.QueryRowContext(ctx, `
		WITH eligible AS (
			SELECT id
			FROM items
			WHERE status IN ('MATCHING', 'LOCKED')
		), first_chains AS (
			SELECT item.id,
			       min(chain.created_at) AS first_chain_at,
			       item.created_at
			FROM items AS item
			JOIN chain_items AS participant ON participant.item_id = item.id
			JOIN chains AS chain ON chain.id = participant.chain_id
			GROUP BY item.id, item.created_at
		)
		SELECT now(),
		       (SELECT count(*) FROM eligible),
		       (SELECT count(*) FROM eligible
		         WHERE EXISTS (SELECT 1 FROM chain_items WHERE chain_items.item_id = eligible.id)),
		       (SELECT avg(EXTRACT(EPOCH FROM (first_chain_at - created_at))) FROM first_chains)`,
	).Scan(&result.GeneratedAt, &result.EligibleItems, &result.ItemsWithChain, &averageTime); err != nil {
		return model.Funnel{}, fmt.Errorf("load item funnel metrics: %w", err)
	}
	if averageTime.Valid {
		result.AverageTimeToFirstChainSeconds = &averageTime.Float64
	}
	result.ItemsWithChainRate = ratio(result.ItemsWithChain, result.EligibleItems)

	if err := tx.QueryRowContext(ctx, `
		SELECT count(*) FILTER (WHERE status IN ('ACCEPTED', 'COMPLETED', 'REJECTED')),
		       count(*) FILTER (WHERE status IN ('ACCEPTED', 'COMPLETED')),
		       count(*) FILTER (WHERE status = 'COMPLETED')
		FROM chains`,
	).Scan(&result.DecidedChains, &result.AcceptedChains, &result.CompletedChains); err != nil {
		return model.Funnel{}, fmt.Errorf("load chain funnel metrics: %w", err)
	}
	result.AcceptanceRate = ratio(result.AcceptedChains, result.DecidedChains)
	result.DeliveryCompletionRate = ratio(result.CompletedChains, result.AcceptedChains)

	rows, err := tx.QueryContext(ctx, `
		SELECT reason, count(*)
		FROM chain_rejections
		GROUP BY reason
		ORDER BY count(*) DESC, reason`)
	if err != nil {
		return model.Funnel{}, fmt.Errorf("load rejection reason metrics: %w", err)
	}
	defer func() { _ = rows.Close() }()
	result.RejectionReasons = make([]model.RejectionReason, 0)
	for rows.Next() {
		var reason model.RejectionReason
		if err := rows.Scan(&reason.Reason, &reason.Count); err != nil {
			return model.Funnel{}, fmt.Errorf("scan rejection reason metrics: %w", err)
		}
		result.RejectionReasons = append(result.RejectionReasons, reason)
	}
	if err := rows.Err(); err != nil {
		return model.Funnel{}, fmt.Errorf("iterate rejection reason metrics: %w", err)
	}
	if err := rows.Close(); err != nil {
		return model.Funnel{}, fmt.Errorf("close rejection reason metrics: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return model.Funnel{}, fmt.Errorf("commit funnel metrics snapshot: %w", err)
	}
	return result, nil
}

func ratio(numerator, denominator int64) *float64 {
	if denominator == 0 {
		return nil
	}
	value := float64(numerator) / float64(denominator)
	return &value
}
