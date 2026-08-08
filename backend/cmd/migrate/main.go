// Package main provides the database migration command.
package main

import (
	"errors"
	"fmt"
	"os"

	"swap-chain/internal/config"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "migration failed: %v\n", err)
		os.Exit(1)
	}
}

func run(arguments []string) error {
	if len(arguments) != 1 {
		return errors.New("usage: migrate <up|down|version>")
	}

	cfg := config.LoadMigration()

	migrator, err := migrate.New(cfg.MigrationsURL, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}
	defer func() {
		_, _ = migrator.Close()
	}()

	switch arguments[0] {
	case "up":
		err = migrator.Up()
	case "down":
		err = migrator.Steps(-1)
	case "version":
		var version uint
		var dirty bool
		version, dirty, err = migrator.Version()
		if err == nil {
			fmt.Printf("version=%d dirty=%t\n", version, dirty)
		}
	default:
		return fmt.Errorf("unknown migration command %q; expected up, down, or version", arguments[0])
	}

	if errors.Is(err, migrate.ErrNoChange) {
		fmt.Println("no migration changes")
		return nil
	}
	if err != nil {
		return fmt.Errorf("%s migrations: %w", arguments[0], err)
	}

	return nil
}
