// Package sqlitestream opens a SQLite database with PRAGMAs suitable for
// Litestream and, when a replica URL is configured via WithReplicaURL,
// restores the file from the replica if it's missing locally and runs
// continuous replication in the background.
package sqlitestream
