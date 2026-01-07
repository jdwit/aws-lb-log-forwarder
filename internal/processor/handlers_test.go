package processor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseS3URL(t *testing.T) {
	t.Run("Valid S3 URL", func(t *testing.T) {
		bucket, key, err := parseS3URL("s3://mybucket/mykey")
		require.NoError(t, err)
		assert.Equal(t, "mybucket", bucket)
		assert.Equal(t, "mykey", key)
	})

	t.Run("Missing s3 prefix", func(t *testing.T) {
		_, _, err := parseS3URL("mybucket/mykey")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "s3://")
	})

	t.Run("No slash after bucket", func(t *testing.T) {
		_, _, err := parseS3URL("s3://mybucket")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "separator")
	})
}
