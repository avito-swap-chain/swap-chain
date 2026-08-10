package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"swap-chain/modules/chat/model"
	"swap-chain/shared/db"
)

type PostgreSQL struct {
	database *sql.DB
}

func NewPostgreSQL(database *sql.DB) (*PostgreSQL, error) {
	if database == nil {
		return nil, fmt.Errorf("postgres chat repository init: 'database' is required")
	}

	return &PostgreSQL{database: database}, nil
}

func (r *PostgreSQL) CreateMessage(
	ctx context.Context,
	chainID int64,
	actorID int64,
	counterpartID int64,
	clientMessageID string,
	text string,
) (model.Message, bool, error) {
	tx, err := r.database.BeginTx(ctx, nil)
	if err != nil {
		return model.Message{}, false, fmt.Errorf("begin create chat message: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	queries := db.New(tx)
	if err := authorize(ctx, queries, chainID, actorID, counterpartID); err != nil {
		return model.Message{}, false, err
	}

	messageID, err := queries.InsertChatMessage(ctx, db.InsertChatMessageParams{
		ChainID:         chainID,
		SenderUserID:    actorID,
		RecipientUserID: counterpartID,
		ClientMessageID: clientMessageID,
		MessageText:     text,
	})
	created := err == nil
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return model.Message{}, false, fmt.Errorf("insert chat message: %w", err)
	}

	message, err := loadMessage(ctx, queries, messageID, chainID, actorID, counterpartID, clientMessageID, created)
	if err != nil {
		return model.Message{}, false, err
	}
	if !created && message.Text != text {
		return model.Message{}, false, model.ErrIdempotencyConflict
	}

	if err := tx.Commit(); err != nil {
		return model.Message{}, false, fmt.Errorf("commit create chat message: %w", err)
	}

	return message, created, nil
}

func (r *PostgreSQL) ListMessages(
	ctx context.Context,
	chainID int64,
	actorID int64,
	counterpartID int64,
	afterID int64,
	limit int,
) ([]model.Message, error) {
	tx, err := r.database.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin list chat messages: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	queries := db.New(tx)
	if err := authorize(ctx, queries, chainID, actorID, counterpartID); err != nil {
		return nil, err
	}

	rows, err := queries.ListChatMessages(ctx, db.ListChatMessagesParams{
		ChainID:       chainID,
		ActorID:       actorID,
		CounterpartID: counterpartID,
		AfterID:       afterID,
		ResultLimit:   int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("list chat messages: %w", err)
	}

	messages := make([]model.Message, 0, len(rows))
	for _, row := range rows {
		messages = append(messages, mapListedMessage(row))
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit list chat messages: %w", err)
	}

	return messages, nil
}

func (r *PostgreSQL) ListThreads(ctx context.Context, actorID int64) ([]model.Thread, error) {
	rows, err := db.New(r.database).ListChatThreads(ctx, actorID)
	if err != nil {
		return nil, fmt.Errorf("list chat threads: %w", err)
	}

	threads := make([]model.Thread, 0, len(rows))
	for _, row := range rows {
		thread := model.Thread{
			ChainID: row.ChainID,
			Counterpart: model.Sender{
				ID:       row.CounterpartUserID,
				Username: row.CounterpartUsername,
			},
			UnreadCount: row.UnreadCount,
		}
		if row.GiveItemID > 0 {
			thread.GiveItem = &model.ItemSummary{
				ID:       row.GiveItemID,
				Title:    row.GiveItemTitle,
				ImageURL: row.GiveItemImageUrl,
			}
		}
		if row.ReceiveItemID > 0 {
			thread.ReceiveItem = &model.ItemSummary{
				ID:       row.ReceiveItemID,
				Title:    row.ReceiveItemTitle,
				ImageURL: row.ReceiveItemImageUrl,
			}
		}
		if row.LastMessageID > 0 {
			thread.LastMessage = &model.Message{
				ID:      row.LastMessageID,
				ChainID: row.ChainID,
				Sender: model.Sender{
					ID:       row.LastSenderUserID,
					Username: row.LastSenderUsername,
				},
				Recipient: model.Sender{
					ID:       row.LastRecipientUserID,
					Username: row.LastRecipientUsername,
				},
				ClientMessageID: row.LastClientMessageID,
				Text:            row.LastMessageText,
				CreatedAt:       row.LastMessageCreatedAt,
			}
		}
		threads = append(threads, thread)
	}
	return threads, nil
}

func (r *PostgreSQL) MarkRead(
	ctx context.Context,
	chainID int64,
	actorID int64,
	counterpartID int64,
	lastReadMessageID int64,
) (model.ReadState, error) {
	tx, err := r.database.BeginTx(ctx, nil)
	if err != nil {
		return model.ReadState{}, fmt.Errorf("begin mark chat thread read: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	queries := db.New(tx)
	if err := authorize(ctx, queries, chainID, actorID, counterpartID); err != nil {
		return model.ReadState{}, err
	}

	belongs, err := queries.ChatMessageBelongsToThread(ctx, db.ChatMessageBelongsToThreadParams{
		MessageID:     lastReadMessageID,
		ChainID:       chainID,
		ActorID:       actorID,
		CounterpartID: counterpartID,
	})
	if err != nil {
		return model.ReadState{}, fmt.Errorf("check chat read cursor: %w", err)
	}
	if !belongs {
		return model.ReadState{}, model.ErrMessageNotFound
	}

	watermark, err := queries.UpsertChatReadState(ctx, db.UpsertChatReadStateParams{
		ChainID:           chainID,
		ActorID:           actorID,
		CounterpartID:     counterpartID,
		LastReadMessageID: lastReadMessageID,
	})
	if err != nil {
		return model.ReadState{}, fmt.Errorf("mark chat thread read: %w", err)
	}
	unreadCount, err := queries.CountUnreadChatMessages(ctx, db.CountUnreadChatMessagesParams{
		ChainID:           chainID,
		CounterpartID:     counterpartID,
		ActorID:           actorID,
		LastReadMessageID: watermark,
	})
	if err != nil {
		return model.ReadState{}, fmt.Errorf("count unread chat messages: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return model.ReadState{}, fmt.Errorf("commit mark chat thread read: %w", err)
	}

	return model.ReadState{
		ChainID:           chainID,
		CounterpartID:     counterpartID,
		LastReadMessageID: watermark,
		UnreadCount:       unreadCount,
	}, nil
}

func authorize(ctx context.Context, queries *db.Queries, chainID, actorID, counterpartID int64) error {
	access, err := queries.GetChatAccessForShare(ctx, db.GetChatAccessForShareParams{
		ChainID:       chainID,
		ActorID:       actorID,
		CounterpartID: counterpartID,
	})
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return model.ErrChainNotFound
	case err != nil:
		return fmt.Errorf("authorize chat access: %w", err)
	}

	return validateAccess(access.IsActorParticipant, access.IsCounterpartParticipant, access.IsNeighbor)
}

func validateAccess(actorParticipant, counterpartParticipant, neighbor bool) error {
	switch {
	case !actorParticipant:
		return model.ErrForbidden
	case !counterpartParticipant, !neighbor:
		return model.ErrThreadNotFound
	default:
		return nil
	}
}

func loadMessage(
	ctx context.Context,
	queries *db.Queries,
	messageID int64,
	chainID int64,
	actorID int64,
	counterpartID int64,
	clientMessageID string,
	created bool,
) (model.Message, error) {
	if created {
		row, err := queries.GetChatMessage(ctx, messageID)
		if err != nil {
			return model.Message{}, fmt.Errorf("load created chat message: %w", err)
		}

		return mapChatMessage(row), nil
	}

	row, err := queries.GetChatMessageByClientID(ctx, db.GetChatMessageByClientIDParams{
		ChainID:         chainID,
		SenderUserID:    actorID,
		RecipientUserID: counterpartID,
		ClientMessageID: clientMessageID,
	})
	if err != nil {
		return model.Message{}, fmt.Errorf("load idempotent chat message: %w", err)
	}

	return mapIdempotentMessage(row), nil
}

func mapChatMessage(row db.GetChatMessageRow) model.Message {
	return model.Message{
		ID:      row.ID,
		ChainID: row.ChainID,
		Sender: model.Sender{
			ID:       row.SenderUserID,
			Username: row.SenderUsername,
		},
		Recipient: model.Sender{
			ID:       row.RecipientUserID,
			Username: row.RecipientUsername,
		},
		ClientMessageID: row.ClientMessageID,
		Text:            row.MessageText,
		CreatedAt:       row.CreatedAt,
	}
}

func mapIdempotentMessage(row db.GetChatMessageByClientIDRow) model.Message {
	return model.Message{
		ID:      row.ID,
		ChainID: row.ChainID,
		Sender: model.Sender{
			ID:       row.SenderUserID,
			Username: row.SenderUsername,
		},
		Recipient: model.Sender{
			ID:       row.RecipientUserID,
			Username: row.RecipientUsername,
		},
		ClientMessageID: row.ClientMessageID,
		Text:            row.MessageText,
		CreatedAt:       row.CreatedAt,
	}
}

func mapListedMessage(row db.ListChatMessagesRow) model.Message {
	return model.Message{
		ID:      row.ID,
		ChainID: row.ChainID,
		Sender: model.Sender{
			ID:       row.SenderUserID,
			Username: row.SenderUsername,
		},
		Recipient: model.Sender{
			ID:       row.RecipientUserID,
			Username: row.RecipientUsername,
		},
		ClientMessageID: row.ClientMessageID,
		Text:            row.MessageText,
		CreatedAt:       row.CreatedAt,
	}
}
