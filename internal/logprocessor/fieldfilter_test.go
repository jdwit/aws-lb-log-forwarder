package logprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFieldFilter(t *testing.T) {
	t.Run("No fields provided includes all", func(t *testing.T) {
		filter, err := NewFieldFilter("")
		require.NoError(t, err)

		for i := range allFields {
			assert.True(t, filter.Includes(i))
		}
	})

	t.Run("Valid fields provided", func(t *testing.T) {
		filter, err := NewFieldFilter("type,time,elb")
		require.NoError(t, err)

		assert.True(t, filter.Includes(fieldIndex("type")))
		assert.True(t, filter.Includes(fieldIndex("time")))
		assert.True(t, filter.Includes(fieldIndex("elb")))
		assert.False(t, filter.Includes(fieldIndex("client:port")))
	})

	t.Run("Invalid field provided", func(t *testing.T) {
		_, err := NewFieldFilter("invalid_field")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid field name")
	})
}

func TestFieldFilterName(t *testing.T) {
	filter, err := NewFieldFilter("")
	require.NoError(t, err)

	t.Run("Valid index", func(t *testing.T) {
		name, ok := filter.Name(0)
		assert.True(t, ok)
		assert.Equal(t, "type", name)

		name, ok = filter.Name(1)
		assert.True(t, ok)
		assert.Equal(t, "time", name)
	})

	t.Run("Invalid index", func(t *testing.T) {
		_, ok := filter.Name(-1)
		assert.False(t, ok)

		_, ok = filter.Name(len(allFields))
		assert.False(t, ok)
	})
}

func TestFieldFilterIncludes(t *testing.T) {
	t.Run("Include all fields", func(t *testing.T) {
		filter, err := NewFieldFilter("")
		require.NoError(t, err)

		for i := range allFields {
			assert.True(t, filter.Includes(i))
		}
	})

	t.Run("Include specific fields", func(t *testing.T) {
		filter, err := NewFieldFilter("type,time")
		require.NoError(t, err)

		assert.True(t, filter.Includes(fieldIndex("type")))
		assert.True(t, filter.Includes(fieldIndex("time")))
		assert.False(t, filter.Includes(fieldIndex("elb")))
	})

	t.Run("Invalid index", func(t *testing.T) {
		filter, err := NewFieldFilter("")
		require.NoError(t, err)

		assert.False(t, filter.Includes(-1))
		assert.False(t, filter.Includes(len(allFields)))
	})
}

func fieldIndex(name string) int {
	for i, n := range allFields {
		if n == name {
			return i
		}
	}
	return -1
}
