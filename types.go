package sqlitestream

import (
	"database/sql"
	"log/slog"

	"github.com/benbjohnson/litestream"
)

// DB is an open SQLite database, optionally with background Litestream
// replication.
//
// Use SQL to access the underlying *sql.DB; call Close when
// done — Close flushes the final WAL segment to S3 if replication is on.
type DB struct {
	sql    *sql.DB
	ldb    *litestream.DB    // nil when no replica URL
	store  *litestream.Store // nil when no replica URL
	logger *slog.Logger
}
