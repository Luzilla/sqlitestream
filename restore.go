package sqlitestream

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/benbjohnson/litestream"
)

// Restore is a one-shot restore from the replica (configured via
// WithReplicaURL) into outputPath. It refuses to overwrite an existing file
// unless force is true; with force the .db, .db-wal and .db-shm files are
// removed first to give SQLite a clean slate. Used by ops scripts and the
// `firehose restore` subcommand.
func Restore(ctx context.Context, outputPath string, force bool, opts ...Option) error {
	cfg := apply(opts)
	if cfg.replicaURL == "" {
		return fmt.Errorf("%w: WithReplicaURL is required for restore", ErrNotReplicating)
	}

	switch _, err := os.Stat(outputPath); {
	case err == nil:
		if !force {
			return fmt.Errorf("%w: %s (pass force=true to overwrite)", ErrExists, outputPath)
		}
		if err := removeDBFiles(outputPath); err != nil {
			return fmt.Errorf("clear destination: %w", err)
		}
	case errors.Is(err, os.ErrNotExist):
		// ok — nothing to clear
	default:
		return fmt.Errorf("unexpected error opening path: %s (%w)", outputPath, err)
	}

	db, err := newLitestreamDB(outputPath, cfg)
	if err != nil {
		return err
	}
	opt := litestream.NewRestoreOptions()
	opt.OutputPath = outputPath
	if err := db.Replica.Restore(ctx, opt); err != nil {
		// litestream reports an empty replica as either error, depending
		// on the client.
		if errors.Is(err, litestream.ErrNoSnapshots) || errors.Is(err, litestream.ErrTxNotAvailable) {
			return ErrReplicaEmpty
		}
		return fmt.Errorf("restore: %w", err)
	}
	cfg.logger.Info("restored", "path", outputPath, "replica", cfg.replicaURL)
	return nil
}

func removeDBFiles(path string) error {
	for _, suffix := range []string{"", "-wal", "-shm"} {
		if err := os.Remove(path + suffix); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}
