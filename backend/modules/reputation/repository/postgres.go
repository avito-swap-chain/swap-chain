package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"swap-chain/modules/reputation/model"

	"github.com/lib/pq"
)

type PostgreSQL struct {
	database *sql.DB
}

func NewPostgreSQL(database *sql.DB) (*PostgreSQL, error) {
	if database == nil {
		return nil, fmt.Errorf("reputation repository: database is required")
	}
	return &PostgreSQL{database: database}, nil
}

func (r *PostgreSQL) Stats(ctx context.Context, userID int64) (model.Stats, error) {
	row := r.database.QueryRowContext(ctx, `
		SELECT reputation.rating,
		       COALESCE(reputation.reviews_count, 0),
		       (SELECT count(*)
		          FROM chain_items participant
		          JOIN chains chain ON chain.id = participant.chain_id
		         WHERE participant.user_id = users.id
		           AND chain.status = 'COMPLETED')
		FROM users
		LEFT JOIN user_reputation reputation ON reputation.user_id = users.id
		WHERE users.id = $1`, userID)

	var stats model.Stats
	var rating sql.NullFloat64
	if err := row.Scan(&rating, &stats.ReviewsCount, &stats.CompletedExchanges); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Stats{}, model.ErrUserNotFound
		}
		return model.Stats{}, fmt.Errorf("load reputation stats: %w", err)
	}
	if rating.Valid {
		stats.Rating = &rating.Float64
	}
	return stats, nil
}

func (r *PostgreSQL) CreateReview(ctx context.Context, authorID, chainID int64, input model.CreateInput) (model.Review, error) {
	tx, err := r.database.BeginTx(ctx, nil)
	if err != nil {
		return model.Review{}, fmt.Errorf("begin create review: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var chainStatus string
	if err := tx.QueryRowContext(ctx, `SELECT status::text FROM chains WHERE id = $1 FOR SHARE`, chainID).Scan(&chainStatus); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Review{}, model.ErrChainNotFound
		}
		return model.Review{}, fmt.Errorf("lock review chain: %w", err)
	}
	if chainStatus != "COMPLETED" {
		return model.Review{}, model.ErrChainNotCompleted
	}

	var neighbour bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM chain_items author
			JOIN chain_items target ON target.chain_id = author.chain_id
			WHERE author.chain_id = $1
			  AND author.user_id = $2
			  AND target.user_id = $3
			  AND (author.next_item_id = target.item_id OR target.next_item_id = author.item_id)
		)`, chainID, authorID, input.TargetUserID).Scan(&neighbour); err != nil {
		return model.Review{}, fmt.Errorf("authorize review neighbour: %w", err)
	}
	if !neighbour {
		return model.Review{}, model.ErrForbidden
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO user_reputation (user_id)
		VALUES ($1)
		ON CONFLICT (user_id) DO NOTHING`, input.TargetUserID); err != nil {
		return model.Review{}, fmt.Errorf("ensure reputation aggregate: %w", err)
	}
	var lockedUserID int64
	if err := tx.QueryRowContext(ctx, `
		SELECT user_id FROM user_reputation WHERE user_id = $1 FOR UPDATE`, input.TargetUserID).Scan(&lockedUserID); err != nil {
		return model.Review{}, fmt.Errorf("lock reputation aggregate: %w", err)
	}

	var review model.Review
	var text sql.NullString
	err = tx.QueryRowContext(ctx, `
		INSERT INTO user_reviews (chain_id, author_user_id, target_user_id, rating, review_text)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, chain_id, target_user_id, rating, review_text, created_at`,
		chainID, authorID, input.TargetUserID, input.Rating, input.Text,
	).Scan(&review.ID, &review.ChainID, &review.TargetUserID, &review.Rating, &text, &review.CreatedAt)
	if err != nil {
		var postgresError *pq.Error
		if errors.As(err, &postgresError) && postgresError.Code == "23505" {
			return model.Review{}, model.ErrAlreadyExists
		}
		return model.Review{}, fmt.Errorf("insert user review: %w", err)
	}
	if text.Valid {
		review.Text = &text.String
	}
	if err := tx.QueryRowContext(ctx, `SELECT id, username FROM users WHERE id = $1`, authorID).
		Scan(&review.Author.ID, &review.Author.Username); err != nil {
		return model.Review{}, fmt.Errorf("load review author: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE user_reputation
		SET rating = aggregate.rating,
		    reviews_count = aggregate.reviews_count,
		    updated_at = now()
		FROM (
			SELECT round(avg(rating)::numeric, 2) AS rating,
			       count(*)::integer AS reviews_count
			FROM user_reviews
			WHERE target_user_id = $1
		) aggregate
		WHERE user_id = $1`, input.TargetUserID); err != nil {
		return model.Review{}, fmt.Errorf("update reputation aggregate: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return model.Review{}, fmt.Errorf("commit create review: %w", err)
	}
	return review, nil
}

func (r *PostgreSQL) ListReviews(ctx context.Context, targetUserID, cursor int64, limit int) (model.ListResult, error) {
	var exists bool
	if err := r.database.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE id = $1)`, targetUserID).Scan(&exists); err != nil {
		return model.ListResult{}, fmt.Errorf("check review target: %w", err)
	}
	if !exists {
		return model.ListResult{}, model.ErrUserNotFound
	}

	rows, err := r.database.QueryContext(ctx, `
		SELECT review.id, review.chain_id, author.id, author.username,
		       review.target_user_id, review.rating, review.review_text, review.created_at
		FROM user_reviews review
		JOIN users author ON author.id = review.author_user_id
		WHERE review.target_user_id = $1
		  AND ($2::bigint = 0 OR review.id < $2)
		ORDER BY review.id DESC
		LIMIT $3`, targetUserID, cursor, limit+1)
	if err != nil {
		return model.ListResult{}, fmt.Errorf("list user reviews: %w", err)
	}
	defer func() { _ = rows.Close() }()

	reviews := make([]model.Review, 0, limit)
	for rows.Next() {
		var review model.Review
		var text sql.NullString
		if err := rows.Scan(
			&review.ID, &review.ChainID, &review.Author.ID, &review.Author.Username,
			&review.TargetUserID, &review.Rating, &text, &review.CreatedAt,
		); err != nil {
			return model.ListResult{}, fmt.Errorf("scan user review: %w", err)
		}
		if text.Valid {
			review.Text = &text.String
		}
		reviews = append(reviews, review)
	}
	if err := rows.Err(); err != nil {
		return model.ListResult{}, fmt.Errorf("iterate user reviews: %w", err)
	}

	result := model.ListResult{Reviews: reviews}
	if len(result.Reviews) > limit {
		cursor := result.Reviews[limit-1].ID
		result.NextCursor = &cursor
		result.Reviews = result.Reviews[:limit]
	}
	return result, nil
}
