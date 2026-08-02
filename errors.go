package sqlitestream

import "errors"

// ErrNotReplicating is returned when an operation requires a configured
// replica but replication is off.
var ErrNotReplicating = errors.New("sqlitestream: replication not configured")

// ErrExists is returned by Restore when the output path already exists
// and force is false.
var ErrExists = errors.New("sqlitestream: file exists")

// ErrReplicaEmpty is returned by Restore when the replica holds no
// snapshots. Callers can treat this as a fresh install.
var ErrReplicaEmpty = errors.New("sqlitestream: replica is empty — nothing to restore")
