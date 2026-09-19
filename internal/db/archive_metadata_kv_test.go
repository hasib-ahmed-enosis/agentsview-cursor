package db

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArchiveMetadataIntRoundTrip(t *testing.T) {
	d := testDB(t)
	ctx := context.Background()
	const key = "test_metadata_key"
	value, err := d.ArchiveMetadataInt(ctx, key)
	require.NoError(t, err)
	assert.Zero(t, value)

	require.NoError(t, d.SetArchiveMetadata(ctx, key, "42"))
	value, err = d.ArchiveMetadataInt(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, int64(42), value)
}
