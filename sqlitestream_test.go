package sqlitestream_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/benbjohnson/litestream/file" // registers the file:// scheme for tests
	"github.com/luzilla/sqlitestream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSuite(t *testing.T) {
	// Open with no replica URL opens a usable WAL-mode database.
	t.Run("OpenNoReplica", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "test.db")

		db, err := sqlitestream.Open(t.Context(), path)
		require.NoError(t, err)
		t.Cleanup(func() { _ = db.Close(t.Context()) })

		require.NotNil(t, db.DB())

		_, err = db.DB().Exec(`CREATE TABLE t (id INTEGER PRIMARY KEY, v TEXT)`)
		require.NoError(t, err)
		_, err = db.DB().Exec(`INSERT INTO t (v) VALUES (?)`, "hello")
		require.NoError(t, err)

		var v string
		require.NoError(t, db.DB().QueryRow(`SELECT v FROM t WHERE id = 1`).Scan(&v))
		assert.Equal(t, "hello", v)

		var mode string
		require.NoError(t, db.DB().QueryRow(`PRAGMA journal_mode`).Scan(&mode))
		assert.Equal(t, "wal", mode)
	})

	// Close is safe on a nil receiver.
	t.Run("CloseNil", func(t *testing.T) {
		var db *sqlitestream.DB
		assert.NoError(t, db.Close(t.Context()))
	})

	// Sync status accessors are safe on a nil receiver and without a replica.
	t.Run("SyncStatusNoReplica", func(t *testing.T) {
		var nilDB *sqlitestream.DB
		assert.False(t, nilDB.Replicating())
		assert.False(t, nilDB.Retention())
		assert.True(t, nilDB.LastSyncedAt().IsZero())
		_, err := nilDB.SyncStatus(t.Context())
		assert.ErrorIs(t, err, sqlitestream.ErrNotReplicating)

		db, err := sqlitestream.Open(t.Context(), filepath.Join(t.TempDir(), "test.db"))
		require.NoError(t, err)
		t.Cleanup(func() { _ = db.Close(t.Context()) })

		assert.False(t, db.Replicating())
		assert.False(t, db.Retention())
		assert.True(t, db.LastSyncedAt().IsZero())
		_, err = db.SyncStatus(t.Context())
		assert.ErrorIs(t, err, sqlitestream.ErrNotReplicating)
	})

	// With a replica configured, LastSyncedAt reflects the background sync.
	t.Run("SyncStatusWithReplica", func(t *testing.T) {
		replicaURL := "file://" + filepath.Join(t.TempDir(), "replica")
		db, err := sqlitestream.Open(t.Context(), filepath.Join(t.TempDir(), "test.db"),
			sqlitestream.WithReplicaURL(replicaURL))
		require.NoError(t, err)
		t.Cleanup(func() { _ = db.Close(t.Context()) })

		assert.True(t, db.Replicating())
		assert.True(t, db.Retention()) // litestream default

		_, err = db.DB().Exec(`CREATE TABLE t (id INTEGER PRIMARY KEY, v TEXT)`)
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			return !db.LastSyncedAt().IsZero()
		}, 10*time.Second, 100*time.Millisecond, "no sync recorded")
		assert.WithinDuration(t, time.Now(), db.LastSyncedAt(), time.Minute)

		_, err = db.SyncStatus(t.Context())
		assert.NoError(t, err)
	})

	t.Run("restore", func(t *testing.T) {
		t.Run("RequiresReplicaURL", func(t *testing.T) {
			err := sqlitestream.Restore(t.Context(), filepath.Join(t.TempDir(), "out.db"), false)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "WithReplicaURL")
		})

		// Restore refuses to overwrite an existing file unless force is set. This is
		// checked before any replica access, so a dummy URL is fine.
		t.Run("RefusesExistingWithoutForce", func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "out.db")
			require.NoError(t, os.WriteFile(path, []byte("existing"), 0o600))

			err := sqlitestream.Restore(t.Context(), path, false,
				sqlitestream.WithReplicaURL("file:///nonexistent"))
			require.Error(t, err)
			assert.Contains(t, err.Error(), "exists")
		})

	})
}
