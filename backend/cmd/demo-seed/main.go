package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"

	_ "github.com/lib/pq"
	"github.com/pgvector/pgvector-go"

	"swap-chain/internal/config"
	"swap-chain/internal/infrastructure/postgres"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "seed failed: %v\n", err)
		os.Exit(1)
	}
}

func run(arguments []string) (returnErr error) {
	if len(arguments) > 0 && arguments[0] == "--help" {
		fmt.Println("usage: demo-seed")
		fmt.Println("  Inserts demo users and items with guaranteed 3-person exchange cycle.")
		fmt.Println("  Requires DATABASE_URL env var pointing to a migrated PostgreSQL.")
		fmt.Println("  NOT part of production migrations — run manually for demos only.")
		return nil
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	ctx := context.Background()
	db, err := postgres.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() {
		returnErr = errors.Join(returnErr, db.Close())
	}()

	users := []userSeed{
		{username: "Алиса", phone: "+79001000001", role: "USER"},
		{username: "Борис", phone: "+79001000002", role: "USER"},
		{username: "Вера", phone: "+79001000003", role: "USER"},
	}

	var userIDs []int64
	for _, u := range users {
		var id int64
		err := db.QueryRowContext(ctx,
			`INSERT INTO users (username, phone, role) VALUES ($1, $2, $3::user_role)
			 ON CONFLICT (phone) DO UPDATE SET username = EXCLUDED.username, role = EXCLUDED.role
			 RETURNING id`, u.username, u.phone, u.role).Scan(&id)
		if err != nil {
			return fmt.Errorf("insert user %s: %w", u.username, err)
		}
		userIDs = append(userIDs, id)
		fmt.Printf("user %d: %s (%s)\n", id, u.username, u.phone)
	}

	v1, v2, v3 := embedding(0), embedding(1), embedding(2)
	categoryIDs, err := loadCategoryIDs(ctx, db, 3)
	if err != nil {
		return err
	}

	items := []itemSeed{
		{ // Алиса предлагает v1, хочет v2 → матчится с Борисом (offer=v2)
			userID: userIDs[0], offerTitle: "Книга 'Архитектура ПО'",
			offerDesc:       "Классическая книга по архитектуре, состояние отличное",
			wantDesc:        "Хочу настольную лампу для рабочего стола",
			offerCategoryID: categoryIDs[0], wantCategoryID: categoryIDs[1],
			offerEmb: v1, wantEmb: v2,
		},
		{ // Борис предлагает v2, хочет v3 → матчится с Верой (offer=v3)
			userID: userIDs[1], offerTitle: "Настольная лампа Xiaomi",
			offerDesc:       "LED лампа с регулировкой яркости, почти новая",
			wantDesc:        "Хочу клавиатуру, желательно механическую",
			offerCategoryID: categoryIDs[1], wantCategoryID: categoryIDs[2],
			offerEmb: v2, wantEmb: v3,
		},
		{ // Вера предлагает v3, хочет v1 → матчится с Алисой (offer=v1)
			userID: userIDs[2], offerTitle: "Механическая клавиатура Keychron",
			offerDesc:       "Keychron K2, красные свитчи, в идеальном состоянии",
			wantDesc:        "Хочу книгу по архитектуре ПО",
			offerCategoryID: categoryIDs[2], wantCategoryID: categoryIDs[0],
			offerEmb: v3, wantEmb: v1,
		},
	}

	var itemIDs []int64
	for _, it := range items {
		var id int64
		err := db.QueryRowContext(ctx,
			`INSERT INTO items (user_id, offer_title, offer_description, want_description,
			 offer_category_id, want_category_id, offer_embedding_local, want_embedding_local, status)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'MATCHING')
			 RETURNING id`,
			it.userID, it.offerTitle, it.offerDesc, it.wantDesc,
			it.offerCategoryID, it.wantCategoryID,
			pgvector.NewVector(it.offerEmb), pgvector.NewVector(it.wantEmb),
		).Scan(&id)
		if err != nil {
			return fmt.Errorf("insert item %q: %w", it.offerTitle, err)
		}
		itemIDs = append(itemIDs, id)
		fmt.Printf("item %d: %q (user %d)\n", id, it.offerTitle, it.userID)
	}

	fmt.Printf("\ndemo seed complete: %d users, %d items\n", len(userIDs), len(itemIDs))
	fmt.Println("pickup-point admin from migration: ПВЗ Администратор (demo), +79009999999")
	fmt.Printf("cycle: item %d → item %d → item %d → back to %d\n",
		itemIDs[0], itemIDs[1], itemIDs[2], itemIDs[0])
	fmt.Println("\nrun matching:")
	for _, id := range itemIDs {
		fmt.Printf("  curl -b cookies.txt http://localhost:8080/api/v1/items/%d/matching\n", id)
	}
	fmt.Println("\nitems:")
	fmt.Printf("  %s\n", strings.Join(formatItemIDs(itemIDs), ", "))

	return nil
}

type userSeed struct {
	username, phone, role string
}

type itemSeed struct {
	userID              int64
	offerTitle          string
	offerDesc, wantDesc string
	offerCategoryID     int32
	wantCategoryID      int32
	offerEmb, wantEmb   []float32
}

func loadCategoryIDs(ctx context.Context, database *sql.DB, limit int) ([]int32, error) {
	rows, err := database.QueryContext(ctx, `SELECT id FROM categories WHERE name <> 'Не определено' ORDER BY id LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("load demo categories: %w", err)
	}
	defer func() { _ = rows.Close() }()

	ids := make([]int32, 0, limit)
	for rows.Next() {
		var id int32
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan demo category: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate demo categories: %w", err)
	}
	if len(ids) != limit {
		return nil, fmt.Errorf("load demo categories: got %d, want %d; apply migrations first", len(ids), limit)
	}

	return ids, nil
}

func embedding(pos int) []float32 {
	emb := make([]float32, 1024)
	emb[pos] = 1.0
	return emb
}

func formatItemIDs(ids []int64) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = fmt.Sprintf("%d", id)
	}
	return out
}
