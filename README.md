# sqlitestream

[![Go Reference](https://pkg.go.dev/badge/github.com/luzilla/sqlitestream.svg)](https://pkg.go.dev/github.com/luzilla/sqlitestream)

Open a SQLite database with WAL PRAGMAs suitable for [Litestream](https://litestream.io). When a replica URL is set, the file is restored from the replica if it is missing locally, and continuous replication runs in the background until `Close`.

Pure-Go: uses the `modernc.org/sqlite` driver, no cgo.

## Install

```sh
go get github.com/luzilla/sqlitestream
```

## API

```go
func Open(ctx context.Context, path string, opts ...Option) (*DB, error)
func Restore(ctx context.Context, outputPath string, force bool, opts ...Option) error

func (*DB) DB() *sql.DB                    // underlying handle
func (*DB) Close(ctx context.Context) error // flushes final WAL, stops replication

func WithReplicaURL(url string) Option
func WithAWSCredentials(accessKeyID, secretAccessKey string) Option
func WithLogger(logger *slog.Logger) Option
```

Without `WithReplicaURL`, `Open` just opens the file in WAL mode and does no
replication.

## Replica URL

An S3 replica URL looks like:

```
s3://bucket/path?region=eu-central-1
```

For an S3-compatible endpoint add `endpoint` and `forcePathStyle`:

```
s3://bucket/path?endpoint=http://127.0.0.1:9000&region=eu-central-1&forcePathStyle=true
```

> [!IMPORTANT]
> Pass credentials with `WithAWSCredentials`. Without them, Litestream falls back to the AWS default chain (env vars, shared config, instance role). Credentials in the URL userinfo (`s3://key:secret@…`) are **not** read.

## Examples

- [`examples/plain`](examples/plain) — `database/sql`
- [`examples/entgo`](examples/entgo) — [Ent](https://entgo.io)

## Development

```sh
make test   # go test -race
make lint   # go vet + go fmt
```

The suite includes an S3 round-trip test that starts an S3 mock server (`adobe/s3mock`); it needs a running Docker daemon.
