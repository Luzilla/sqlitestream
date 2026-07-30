package sqlitestream

import "log/slog"

// Option configures Open and Restore.
type Option func(*config)

type config struct {
	logger          *slog.Logger
	replicaURL      string
	accessKeyID     string
	secretAccessKey string
}

func apply(opts []Option) *config {
	c := &config{logger: slog.Default()}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// WithLogger sets the slog.Logger used for replication and restore events.
func WithLogger(logger *slog.Logger) Option {
	return func(c *config) {
		if logger != nil {
			c.logger = logger
		}
	}
}

// WithReplicaURL configures the Litestream replica destination as a URL,
// e.g. "s3://bucket/path?region=us-east-1" or "file:///var/lib/replica".
// Empty disables replication entirely.
func WithReplicaURL(url string) Option {
	return func(c *config) { c.replicaURL = url }
}

// WithCredentials sets credentials separately from the URL. Preferred over
// embedding "key:secret@" in the URL because URLs are easy to log or expose
// via process listings.
func WithAWSCredentials(accessKeyID, secretAccessKey string) Option {
	return func(c *config) {
		c.accessKeyID = accessKeyID
		c.secretAccessKey = secretAccessKey
	}
}
