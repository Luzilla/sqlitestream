package sqlitestream

import "errors"

// ErrNotReplicating is returned when an operation requires a configured
// replica but replication is off.
var ErrNotReplicating = errors.New("sqlitestream: replication not configured")
