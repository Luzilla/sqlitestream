package sqlitestream

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/url"
	"time"

	"github.com/benbjohnson/litestream"
	"github.com/benbjohnson/litestream/s3" // also registers the s3:// scheme
	_ "modernc.org/sqlite"                 // register the "sqlite" driver
)

// dsn assembles the modernc.org/sqlite connection string. PRAGMAs go in the
// query string so they're applied per-connection (the pool may open many).
// WAL mode is required by Litestream.
func dsn(path string) string {
	params := url.Values{}
	params.Add("_pragma", "journal_mode(WAL)")
	params.Add("_pragma", "synchronous(NORMAL)")
	params.Add("_pragma", "busy_timeout(5000)")
	params.Add("_pragma", "foreign_keys(1)")

	return path + "?" + params.Encode()
}

// Open opens path with PRAGMAs suitable for Litestream. If WithReplicaURL is
// supplied, the database is restored from the replica when missing locally
// and background replication is started before the SQL handle is opened.
func Open(ctx context.Context, path string, opts ...Option) (*DB, error) {
	cfg := apply(opts)
	out := &DB{logger: cfg.logger}

	if cfg.replicaURL != "" {
		ldb, err := newLitestreamDB(path, cfg)
		if err != nil {
			return nil, err
		}
		if err := ldb.EnsureExists(ctx); err != nil {
			return nil, fmt.Errorf("restore: %w", err)
		}
		store := litestream.NewStore([]*litestream.DB{ldb}, litestream.DefaultCompactionLevels)
		store.Logger = cfg.logger
		if err := store.Open(ctx); err != nil {
			return nil, fmt.Errorf("open litestream store: %w", err)
		}
		out.ldb = ldb
		out.store = store
		cfg.logger.Info("replicating", "path", path, "replica", cfg.replicaURL)
	}

	db, err := sql.Open("sqlite", dsn(path))
	if err != nil {
		_ = out.closeLitestream(ctx)
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	out.sql = db
	return out, nil
}

// DB returns the underlying *sql.DB for use with Ent or database/sql.
func (d *DB) DB() *sql.DB {
	return d.sql
}

// Replicating reports whether background replication is configured.
func (d *DB) Replicating() bool {
	return d != nil && d.ldb != nil
}

// Retention reports whether litestream is handling retention.
func (d *DB) Retention() bool {
	return d != nil && d.ldb != nil && d.ldb.RetentionEnabled
}

// LastSyncedAt returns the time of the last successful sync to the
// replica. The zero time means replication is off or nothing has been
// synced yet.
func (d *DB) LastSyncedAt() time.Time {
	if d == nil || d.ldb == nil {
		return time.Time{}
	}
	return d.ldb.LastSuccessfulSyncAt()
}

// SyncStatus reports the status between the local database and the
// configured (cloud) storage by comparing transaction records. This
// may entail I/O. Returns ErrNotReplicating when no replica is
// configured.
func (d *DB) SyncStatus(ctx context.Context) (litestream.SyncStatus, error) {
	if d == nil || d.ldb == nil {
		return litestream.SyncStatus{}, ErrNotReplicating
	}
	return d.ldb.SyncStatus(ctx)
}

// Close flushes any pending WAL to S3 (when replication is on), stops the
// replicator, and closes the SQL handle.
//
// Pass a fresh context — the daemon's main context is typically already
// cancelled by shutdown time, and a cancelled context aborts the flush.
func (d *DB) Close(ctx context.Context) error {
	if d == nil {
		return nil
	}
	lerr := d.closeLitestream(ctx)
	var serr error
	if d.sql != nil {
		serr = d.sql.Close()
	}
	if lerr != nil {
		return lerr
	}
	return serr
}

func (d *DB) closeLitestream(ctx context.Context) error {
	if d.ldb != nil {
		if err := d.ldb.SyncAndWait(ctx); err != nil {
			d.logger.Warn("final flush", "err", err)
		}
	}
	if d.store != nil {
		return d.store.Close(ctx)
	}
	return nil
}

// newLitestreamDB constructs a Litestream DB and attaches a replica built
// from cfg's URL.
func newLitestreamDB(path string, cfg *config) (*litestream.DB, error) {
	client, err := newReplicaClient(cfg.replicaURL, cfg.accessKeyID, cfg.secretAccessKey, cfg.logger)
	if err != nil {
		return nil, err
	}
	db := litestream.NewDB(path)
	db.SetLogger(cfg.logger)

	db.Replica = litestream.NewReplicaWithClient(db, client)
	return db, nil
}

// newReplicaClient creates the ReplicaClient.
//
// Credentials, when supplied separately, are set on the s3 client directly.
// Litestream's s3 backend reads them from these fields (or the AWS default
// chain) — it ignores userinfo in the replica URL.
func newReplicaClient(replicaURL, accessKeyID, secretAccessKey string, logger *slog.Logger) (litestream.ReplicaClient, error) {
	client, err := litestream.NewReplicaClientFromURL(replicaURL)
	if err != nil {
		return nil, fmt.Errorf("build replica client: %w", err)
	}

	if accessKeyID != "" || secretAccessKey != "" {
		s3c, ok := client.(*s3.ReplicaClient)
		if !ok {
			return nil, fmt.Errorf("credentials given but replica is %T, not s3", client)
		}
		s3c.AccessKeyID = accessKeyID
		s3c.SecretAccessKey = secretAccessKey
	}

	client.SetLogger(logger)
	return client, nil
}
