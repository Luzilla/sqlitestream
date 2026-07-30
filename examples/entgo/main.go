// Command entgo shows sqlitestream driving an Ent client.
//
// Ent needs the *sql.DB wrapped in its own dialect driver; sqlitestream owns
// the file (PRAGMAs, optional replication) and hands you that *sql.DB via DB().
//
// Generate the Ent code once, then run:
//
//	go generate ./...
//	go run . app.db
//
// With an S3 replica:
//
//	REPLICA_URL=s3://bucket/db?region=eu-central-1 \
//	AWS_ACCESS_KEY_ID=... AWS_SECRET_ACCESS_KEY=... \
//	go run . app.db
package main

import (
	"context"
	"log"
	"os"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/luzilla/sqlitestream"
	"github.com/luzilla/sqlitestream/examples/entgo/ent"
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

	// Wrap sqlitestream's *sql.DB in Ent's sqlite dialect driver.
	drv := entsql.OpenDB(dialect.SQLite, ls.DB())
	client := ent.NewClient(ent.Driver(drv))

	if err := client.Schema.Create(ctx); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	u, err := client.User.Create().SetName("ada").Save(ctx)
	if err != nil {
		log.Fatalf("create user: %v", err)
	}

	count, err := client.User.Query().Count(ctx)
	if err != nil {
		log.Fatalf("count: %v", err)
	}
	log.Printf("created user %d, users total: %d", u.ID, count)
}
