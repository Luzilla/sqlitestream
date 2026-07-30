// Command plain shows sqlitestream with database/sql.
//
// Run without replication:
//
//	go run . app.db
//
// Run with an S3 replica (restores on start if the file is missing, then
// replicates in the background):
//
//	REPLICA_URL=s3://bucket/db?region=eu-central-1 \
//	AWS_ACCESS_KEY_ID=... AWS_SECRET_ACCESS_KEY=... \
//	go run . app.db
package main

import (
	"context"
	"log"
	"os"

	"github.com/luzilla/sqlitestream"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("usage: %s <db-path>", os.Args[0])
	}
	path := os.Args[1]

	ctx := context.Background()

	var opts []sqlitestream.Option
	if url := os.Getenv("REPLICA_URL"); url != "" {
		opts = append(opts,
			sqlitestream.WithReplicaURL(url),
			sqlitestream.WithAWSCredentials(
				os.Getenv("AWS_ACCESS_KEY_ID"),
				os.Getenv("AWS_SECRET_ACCESS_KEY"),
			),
		)
	}

	ls, err := sqlitestream.Open(ctx, path, opts...)
	if err != nil {
		log.Fatalf("open: %v", err)
	}
	// Fresh context so the final WAL flush is not aborted by shutdown.
	defer func() {
		if err := ls.Close(context.Background()); err != nil {
			log.Printf("close: %v", err)
		}
	}()

	db := ls.DB()

	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS visits (
			id  INTEGER PRIMARY KEY AUTOINCREMENT,
			at  TEXT NOT NULL DEFAULT (datetime('now'))
		)`); err != nil {
		log.Fatalf("create: %v", err)
	}

	if _, err := db.ExecContext(ctx, `INSERT INTO visits DEFAULT VALUES`); err != nil {
		log.Fatalf("insert: %v", err)
	}

	var count int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM visits`).Scan(&count); err != nil {
		log.Fatalf("count: %v", err)
	}
	log.Printf("visits: %d", count)
}
