package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/lib/pq"

	"swap-chain/modules/notifications/model"
)

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgres(db *sql.DB) (*PostgresRepository, error) {
	if db == nil {
		return nil, fmt.Errorf("notifications repository: db is required")
	}
	return &PostgresRepository{db: db}, nil
}

func (r *PostgresRepository) Create(ctx context.Context, notification model.Notification) (model.Notification, error) {
	row := r.db.QueryRowContext(ctx,
		`INSERT INTO notifications (user_id, kind, title, text, target_url, entity_id)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, user_id, kind, title, text, target_url, entity_id, is_read, created_at`,
		notification.UserID, notification.Kind, notification.Title, notification.Text,
		notification.TargetURL, notification.EntityID)

	var result model.Notification
	var entityID sql.NullInt64
	err := row.Scan(&result.ID, &result.UserID, &result.Kind, &result.Title,
		&result.Text, &result.TargetURL, &entityID, &result.IsRead, &result.CreatedAt)
	if err != nil {
		return model.Notification{}, fmt.Errorf("create notification: %w", err)
	}
	if entityID.Valid {
		result.EntityID = &entityID.Int64
	}
	return result, nil
}

func (r *PostgresRepository) List(ctx context.Context, userID int64, cursor int64, limit int) (model.ListResult, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND NOT is_read`, userID)
	var unreadCount int64
	if err := row.Scan(&unreadCount); err != nil {
		return model.ListResult{}, fmt.Errorf("count unread notifications: %w", err)
	}

	var rows *sql.Rows
	var err error
	if cursor == 0 {
		rows, err = r.db.QueryContext(ctx,
			`SELECT id, user_id, kind, title, text, target_url, entity_id, is_read, created_at
			 FROM notifications
			 WHERE user_id = $1
			 ORDER BY created_at DESC, id DESC
			 LIMIT $2`, userID, limit+1)
	} else {
		rows, err = r.db.QueryContext(ctx,
			`SELECT id, user_id, kind, title, text, target_url, entity_id, is_read, created_at
			 FROM notifications
			 WHERE user_id = $1 AND id < $2
			 ORDER BY created_at DESC, id DESC
			 LIMIT $3`, userID, cursor, limit+1)
	}
	if err != nil {
		return model.ListResult{}, fmt.Errorf("list notifications: %w", err)
	}
	defer rows.Close()

	items := make([]model.Notification, 0, limit)
	for rows.Next() {
		var n model.Notification
		var entityID sql.NullInt64
		if err := rows.Scan(&n.ID, &n.UserID, &n.Kind, &n.Title,
			&n.Text, &n.TargetURL, &entityID, &n.IsRead, &n.CreatedAt); err != nil {
			return model.ListResult{}, fmt.Errorf("scan notification: %w", err)
		}
		if entityID.Valid {
			n.EntityID = &entityID.Int64
		}
		items = append(items, n)
	}
	if err := rows.Err(); err != nil {
		return model.ListResult{}, fmt.Errorf("iterate notifications: %w", err)
	}

	var nextCursor *int64
	if len(items) > limit {
		last := items[limit-1]
		nextCursor = &last.ID
		items = items[:limit]
	}

	return model.ListResult{
		Items:       items,
		NextCursor:  nextCursor,
		UnreadCount: unreadCount,
	}, nil
}

func (r *PostgresRepository) MarkRead(ctx context.Context, userID int64, ids []int64) error {
	if len(ids) > 0 {
		_, err := r.db.ExecContext(ctx,
			`UPDATE notifications SET is_read = TRUE
			 WHERE user_id = $1 AND id = ANY($2) AND NOT is_read`,
			userID, pq.Array(ids))
		if err != nil {
			return fmt.Errorf("mark read selected: %w", err)
		}
	} else {
		_, err := r.db.ExecContext(ctx,
			`UPDATE notifications SET is_read = TRUE
			 WHERE user_id = $1 AND NOT is_read`,
			userID)
		if err != nil {
			return fmt.Errorf("mark all read: %w", err)
		}
	}
	return nil
}
