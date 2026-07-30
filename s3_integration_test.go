package sqlitestream_test

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/luzilla/sqlitestream"
	"github.com/moby/moby/api/types/container"
	"github.com/ory/dockertest/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	s3Bucket   = "litestream"
	s3Region   = "eu-central-1"
	s3KeyID    = "test"
	s3Secret   = "testtest"
	s3MockPort = "9090/tcp"

	s3MockVersion = "5.1"
)

// TestS3RoundTrip replicates a database to an adobe/s3mock container, then
// restores it into a fresh path and checks the data survived the round-trip.
func TestS3RoundTrip(t *testing.T) {
	endpoint := startS3Mock(t)

	replicaURL := fmt.Sprintf("s3://%s/db?endpoint=%s&region=%s&forcePathStyle=true",
		s3Bucket, endpoint, s3Region)

	opts := []sqlitestream.Option{
		sqlitestream.WithReplicaURL(replicaURL),
		sqlitestream.WithAWSCredentials(s3KeyID, s3Secret),
	}

	t.Run("Write a row, then flush to S3 by closing", func(t *testing.T) {
		dbPath := filepath.Join(t.TempDir(), "app.db")
		db, err := sqlitestream.Open(t.Context(), dbPath, opts...)
		require.NoError(t, err)

		_, err = db.DB().Exec(`CREATE TABLE t (id INTEGER PRIMARY KEY, v TEXT)`)
		require.NoError(t, err)
		_, err = db.DB().Exec(`INSERT INTO t (v) VALUES (?)`, "survives")
		require.NoError(t, err)

		// Close flushes the final WAL segment to the replica.
		require.NoError(t, db.Close(t.Context()))
	})

	t.Run("Restore into a brand-new path from the replica alone", func(t *testing.T) {
		restorePath := filepath.Join(t.TempDir(), "restored.db")
		require.NoError(t, sqlitestream.Restore(t.Context(), restorePath, false, opts...))

		restored, err := sqlitestream.Open(t.Context(), restorePath, opts...)
		require.NoError(t, err)
		t.Cleanup(func() { _ = restored.Close(t.Context()) })

		var v string
		require.NoError(t, restored.DB().QueryRow(`SELECT v FROM t WHERE id = 1`).Scan(&v))
		assert.Equal(t, "survives", v)
	})
}

// startS3Mock runs adobe/s3mock, creates the bucket, and returns the
// http://host:port endpoint. The container is torn down when the test ends.
func startS3Mock(t *testing.T) string {
	t.Helper()

	pool := dockertest.NewPoolT(t, "", dockertest.WithMaxWait(60*time.Second))

	// No explicit port binding: dockertest always sets PublishAllPorts, so
	// s3mock's exposed ports get random free host ports. Pinning a host port
	// (even "" for random) made Docker try to grab 9090 and clash with
	// whatever already holds it on the CI runner.
	resource := pool.RunT(t, "adobe/s3mock",
		dockertest.WithTag(s3MockVersion),
		dockertest.WithEnv([]string{
			"COM_ADOBE_TESTING_S3MOCK_STORE_INITIAL_BUCKETS=" + s3Bucket,
		}),
		dockertest.WithHostConfig(func(hc *container.HostConfig) {
			hc.RestartPolicy = container.RestartPolicy{
				Name: container.RestartPolicyDisabled,
			}
		}),
	)

	// Read back the random host port Docker assigned.
	endpoint := "http://" + resource.GetHostPort(s3MockPort)

	// Wait until s3mock answers and the initial bucket is there.
	require.NoError(t, pool.Retry(t.Context(), (3*time.Minute), func() error {
		ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"/"+s3Bucket, nil)
		if err != nil {
			return err
		}

		// Connection refused is expected while s3mock is still booting —
		// return the error so Retry loops instead of failing the test.
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer res.Body.Close()

		if res.StatusCode != http.StatusOK {
			return fmt.Errorf("not ready: %d", res.StatusCode)
		}
		return nil
	}), "wait for s3mock bucket")

	return endpoint
}
