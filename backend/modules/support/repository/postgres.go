package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"swap-chain/modules/support/model"
)

type PostgreSQL struct{ db *sql.DB }

func New(db *sql.DB) (*PostgreSQL, error) {
	if db == nil {
		return nil, fmt.Errorf("support repository: database is required")
	}
	return &PostgreSQL{db: db}, nil
}

func (r *PostgreSQL) UserThread(ctx context.Context, userID int64) (model.Thread, error) {
	return r.loadThread(ctx, userID, 0, false)
}

func (r *PostgreSQL) AdminThreads(ctx context.Context, adminID int64, limit int) ([]model.Thread, error) {
	if err := r.requireAdmin(ctx, adminID); err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT t.id, u.id, u.username, t.created_at, t.updated_at,
		       COALESCE((SELECT count(*) FROM support_messages m
		         WHERE m.thread_id=t.id AND m.sender_type='USER'
		           AND m.id > COALESCE((SELECT last_read_message_id FROM support_read_states r WHERE r.thread_id=t.id AND r.user_id=$1),0)),0)
		FROM support_threads t JOIN users u ON u.id=t.user_id
		ORDER BY t.updated_at DESC, t.id DESC LIMIT $2`, adminID, limit)
	if err != nil {
		return nil, fmt.Errorf("list support threads: %w", err)
	}
	defer rows.Close()
	var result []model.Thread
	for rows.Next() {
		var thread model.Thread
		if err := rows.Scan(&thread.ID, &thread.User.ID, &thread.User.Username, &thread.CreatedAt, &thread.UpdatedAt, &thread.UnreadCount); err != nil {
			return nil, err
		}
		thread.LastMessage, err = r.lastMessage(ctx, thread.ID)
		if err != nil {
			return nil, fmt.Errorf("load last support message: %w", err)
		}
		thread.Moderators, err = r.moderators(ctx, thread.ID)
		if err != nil {
			return nil, fmt.Errorf("load support moderators: %w", err)
		}
		result = append(result, thread)
	}
	return result, rows.Err()
}

func (r *PostgreSQL) AdminThread(ctx context.Context, adminID, threadID int64) (model.Thread, error) {
	return r.loadThread(ctx, adminID, threadID, true)
}

func (r *PostgreSQL) ListMessages(ctx context.Context, actorID, threadID, afterID int64, limit int, admin bool) ([]model.Message, error) {
	if _, err := r.loadThread(ctx, actorID, threadID, admin); err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT m.id,m.thread_id,m.sender_type,m.sender_user_id,u.username,m.client_message_id,m.message_text,m.created_at
		FROM support_messages m LEFT JOIN users u ON u.id=m.sender_user_id
		WHERE m.thread_id=$1 AND m.id>$2 ORDER BY m.id ASC LIMIT $3`, threadID, afterID, limit)
	if err != nil {
		return nil, fmt.Errorf("list support messages: %w", err)
	}
	defer rows.Close()
	var messages []model.Message
	for rows.Next() {
		message, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		messages = append(messages, message)
	}
	return messages, rows.Err()
}

func (r *PostgreSQL) Send(ctx context.Context, actorID, threadID int64, clientID, text string, admin bool) (model.Message, bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return model.Message{}, false, err
	}
	defer tx.Rollback()
	if err := authorizeTx(ctx, tx, actorID, threadID, admin, admin); err != nil {
		return model.Message{}, false, err
	}
	senderType := model.SenderUser
	if admin {
		senderType = model.SenderModerator
	}
	var id int64
	err = tx.QueryRowContext(ctx, `INSERT INTO support_messages(thread_id,sender_type,sender_user_id,client_message_id,message_text)
		VALUES($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING RETURNING id`, threadID, senderType, actorID, clientID, text).Scan(&id)
	created := err == nil
	if errors.Is(err, sql.ErrNoRows) {
		var oldText string
		err = tx.QueryRowContext(ctx, `SELECT id,message_text FROM support_messages WHERE thread_id=$1 AND sender_user_id=$2 AND client_message_id=$3`, threadID, actorID, clientID).Scan(&id, &oldText)
		if err == nil && oldText != text {
			return model.Message{}, false, model.ErrIdempotencyConflict
		}
	}
	if err != nil {
		return model.Message{}, false, fmt.Errorf("store support message: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `UPDATE support_threads SET updated_at=now() WHERE id=$1`, threadID); err != nil {
		return model.Message{}, false, err
	}
	message, err := loadMessageTx(ctx, tx, id)
	if err != nil {
		return model.Message{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return model.Message{}, false, err
	}
	return message, created, nil
}

func (r *PostgreSQL) Join(ctx context.Context, adminID, threadID int64) (model.Thread, bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return model.Thread{}, false, err
	}
	defer tx.Rollback()
	if err := authorizeTx(ctx, tx, adminID, threadID, true, false); err != nil {
		return model.Thread{}, false, err
	}
	res, err := tx.ExecContext(ctx, `INSERT INTO support_thread_moderators(thread_id,moderator_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, threadID, adminID)
	if err != nil {
		return model.Thread{}, false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return model.Thread{}, false, fmt.Errorf("check support join result: %w", err)
	}
	joined := n == 1
	if joined {
		var name string
		if err := tx.QueryRowContext(ctx, `SELECT username FROM users WHERE id=$1`, adminID).Scan(&name); err != nil {
			return model.Thread{}, false, err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO support_messages(thread_id,sender_type,message_text) VALUES($1,'SYSTEM',$2)`, threadID, "Модератор "+name+" подключился к диалогу"); err != nil {
			return model.Thread{}, false, err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE support_threads SET updated_at=now() WHERE id=$1`, threadID); err != nil {
			return model.Thread{}, false, fmt.Errorf("touch support thread: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return model.Thread{}, false, err
	}
	thread, err := r.AdminThread(ctx, adminID, threadID)
	return thread, joined, err
}

func (r *PostgreSQL) Leave(ctx context.Context, adminID, threadID int64) (model.Thread, bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return model.Thread{}, false, err
	}
	defer tx.Rollback()
	if err := authorizeTx(ctx, tx, adminID, threadID, true, false); err != nil {
		return model.Thread{}, false, err
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM support_thread_moderators WHERE thread_id=$1 AND moderator_id=$2`, threadID, adminID)
	if err != nil {
		return model.Thread{}, false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return model.Thread{}, false, fmt.Errorf("check support leave result: %w", err)
	}
	left := n == 1
	if left {
		var name string
		if err := tx.QueryRowContext(ctx, `SELECT username FROM users WHERE id=$1`, adminID).Scan(&name); err != nil {
			return model.Thread{}, false, err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO support_messages(thread_id,sender_type,message_text) VALUES($1,'SYSTEM',$2)`, threadID, "Модератор "+name+" отключился от диалога"); err != nil {
			return model.Thread{}, false, err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE support_threads SET updated_at=now() WHERE id=$1`, threadID); err != nil {
			return model.Thread{}, false, fmt.Errorf("touch support thread: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return model.Thread{}, false, err
	}
	thread, err := r.AdminThread(ctx, adminID, threadID)
	return thread, left, err
}

func (r *PostgreSQL) MarkRead(ctx context.Context, actorID, threadID, messageID int64, admin bool) (model.ReadState, error) {
	if _, err := r.loadThread(ctx, actorID, threadID, admin); err != nil {
		return model.ReadState{}, err
	}
	var belongs bool
	if err := r.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM support_messages WHERE id=$1 AND thread_id=$2)`, messageID, threadID).Scan(&belongs); err != nil {
		return model.ReadState{}, err
	}
	if !belongs {
		return model.ReadState{}, model.ErrNotFound
	}
	var watermark int64
	err := r.db.QueryRowContext(ctx, `INSERT INTO support_read_states(thread_id,user_id,last_read_message_id) VALUES($1,$2,$3) ON CONFLICT(thread_id,user_id) DO UPDATE SET last_read_message_id=GREATEST(support_read_states.last_read_message_id,EXCLUDED.last_read_message_id),updated_at=now() RETURNING last_read_message_id`, threadID, actorID, messageID).Scan(&watermark)
	if err != nil {
		return model.ReadState{}, err
	}
	var unread int64
	if admin {
		err = r.db.QueryRowContext(ctx, `SELECT count(*) FROM support_messages WHERE thread_id=$1 AND id>$2 AND sender_type='USER'`, threadID, watermark).Scan(&unread)
	} else {
		err = r.db.QueryRowContext(ctx, `SELECT count(*) FROM support_messages WHERE thread_id=$1 AND id>$2 AND sender_type<>'USER'`, threadID, watermark).Scan(&unread)
	}
	return model.ReadState{ThreadID: threadID, LastReadMessageID: watermark, UnreadCount: unread}, err
}

func (r *PostgreSQL) loadThread(ctx context.Context, actorID, threadID int64, admin bool) (model.Thread, error) {
	if admin {
		if err := r.requireAdmin(ctx, actorID); err != nil {
			return model.Thread{}, err
		}
	} else {
		threadID = 0
	}
	var t model.Thread
	query := `SELECT t.id,u.id,u.username,t.created_at,t.updated_at FROM support_threads t JOIN users u ON u.id=t.user_id WHERE `
	var err error
	if admin {
		err = r.db.QueryRowContext(ctx, query+`t.id=$1`, threadID).Scan(&t.ID, &t.User.ID, &t.User.Username, &t.CreatedAt, &t.UpdatedAt)
	} else {
		err = r.db.QueryRowContext(ctx, query+`t.user_id=$1`, actorID).Scan(&t.ID, &t.User.ID, &t.User.Username, &t.CreatedAt, &t.UpdatedAt)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return model.Thread{}, model.ErrNotFound
	}
	if err != nil {
		return model.Thread{}, err
	}
	t.LastMessage, err = r.lastMessage(ctx, t.ID)
	if err != nil {
		return model.Thread{}, fmt.Errorf("load last support message: %w", err)
	}
	t.Moderators, err = r.moderators(ctx, t.ID)
	if err != nil {
		return model.Thread{}, fmt.Errorf("load support moderators: %w", err)
	}
	if admin {
		err = r.db.QueryRowContext(ctx, `SELECT count(*) FROM support_messages WHERE thread_id=$1 AND sender_type='USER' AND id>COALESCE((SELECT last_read_message_id FROM support_read_states WHERE thread_id=$1 AND user_id=$2),0)`, t.ID, actorID).Scan(&t.UnreadCount)
	} else {
		err = r.db.QueryRowContext(ctx, `SELECT count(*) FROM support_messages WHERE thread_id=$1 AND sender_type<>'USER' AND id>COALESCE((SELECT last_read_message_id FROM support_read_states WHERE thread_id=$1 AND user_id=$2),0)`, t.ID, actorID).Scan(&t.UnreadCount)
	}
	if err != nil {
		return model.Thread{}, fmt.Errorf("count unread support messages: %w", err)
	}
	return t, nil
}

func (r *PostgreSQL) requireAdmin(ctx context.Context, id int64) error {
	var role string
	err := r.db.QueryRowContext(ctx, `SELECT role FROM users WHERE id=$1`, id).Scan(&role)
	if err != nil || role != "ADMIN" {
		return model.ErrForbidden
	}
	return nil
}
func authorizeTx(ctx context.Context, tx *sql.Tx, actorID, threadID int64, admin, requireJoined bool) error {
	var role string
	var owner int64
	err := tx.QueryRowContext(ctx, `SELECT u.role,t.user_id FROM support_threads t JOIN users u ON u.id=$1 WHERE t.id=$2`, actorID, threadID).Scan(&role, &owner)
	if errors.Is(err, sql.ErrNoRows) {
		return model.ErrNotFound
	}
	if err != nil {
		return err
	}
	if admin {
		if role != "ADMIN" {
			return model.ErrForbidden
		}
		if requireJoined {
			var ok bool
			if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM support_thread_moderators WHERE thread_id=$1 AND moderator_id=$2)`, threadID, actorID).Scan(&ok); err != nil {
				return err
			}
			if !ok {
				return model.ErrNotJoined
			}
		}
	} else if owner != actorID {
		return model.ErrForbidden
	}
	return nil
}
func (r *PostgreSQL) moderators(ctx context.Context, id int64) ([]model.User, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT u.id,u.username FROM support_thread_moderators m JOIN users u ON u.id=m.moderator_id WHERE m.thread_id=$1 ORDER BY m.joined_at,u.id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.User
	for rows.Next() {
		var u model.User
		if err := rows.Scan(&u.ID, &u.Username); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}
func (r *PostgreSQL) lastMessage(ctx context.Context, id int64) (*model.Message, error) {
	row := r.db.QueryRowContext(ctx, `SELECT m.id,m.thread_id,m.sender_type,m.sender_user_id,u.username,m.client_message_id,m.message_text,m.created_at FROM support_messages m LEFT JOIN users u ON u.id=m.sender_user_id WHERE m.thread_id=$1 ORDER BY m.id DESC LIMIT 1`, id)
	m, err := scanMessage(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &m, err
}

type scanner interface{ Scan(...any) error }

func scanMessage(s scanner) (model.Message, error) {
	var m model.Message
	var senderID sql.NullInt64
	var name, client sql.NullString
	err := s.Scan(&m.ID, &m.ThreadID, &m.SenderType, &senderID, &name, &client, &m.Text, &m.CreatedAt)
	if senderID.Valid {
		m.Sender = &model.User{ID: senderID.Int64, Username: name.String}
	}
	if client.Valid {
		m.ClientMessageID = &client.String
	}
	return m, err
}
func loadMessageTx(ctx context.Context, tx *sql.Tx, id int64) (model.Message, error) {
	return scanMessage(tx.QueryRowContext(ctx, `SELECT m.id,m.thread_id,m.sender_type,m.sender_user_id,u.username,m.client_message_id,m.message_text,m.created_at FROM support_messages m LEFT JOIN users u ON u.id=m.sender_user_id WHERE m.id=$1`, id))
}
