package processor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFields(t *testing.T) {
	t.Run("No fields provided includes all", func(t *testing.T) {
		fields, err := NewFields("")
		require.NoError(t, err)

		for i := range fieldNames {
			assert.True(t, fields.Include(i))
		}
	})

	t.Run("Valid fields provided", func(t *testing.T) {
		fields, err := NewFields("type,time,elb")
		require.NoError(t, err)

		assert.True(t, fields.Include(fieldIndex("type")))
		assert.True(t, fields.Include(fieldIndex("time")))
		assert.True(t, fields.Include(fieldIndex("elb")))
		assert.False(t, fields.Include(fieldIndex("client:port")))
	})

	t.Run("Invalid field provided", func(t *testing.T) {
		_, err := NewFields("invalid_field")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid field name")
	})
}

func TestFieldName(t *testing.T) {
	fields, err := NewFields("")
	require.NoError(t, err)

	t.Run("Valid index", func(t *testing.T) {
		name, ok := fields.FieldName(0)
		assert.True(t, ok)
		assert.Equal(t, "type", name)

		name, ok = fields.FieldName(1)
		assert.True(t, ok)
		assert.Equal(t, "time", name)
	})

	t.Run("Invalid index", func(t *testing.T) {
		_, ok := fields.FieldName(-1)
		assert.False(t, ok)

		_, ok = fields.FieldName(len(fieldNames))
		assert.False(t, ok)
	})
}

func TestInclude(t *testing.T) {
	t.Run("Include all fields", func(t *testing.T) {
		fields, err := NewFields("")
		require.NoError(t, err)

		for i := range fieldNames {
			assert.True(t, fields.Include(i))
		}
	})

	t.Run("Include specific fields", func(t *testing.T) {
		fields, err := NewFields("type,time")
		require.NoError(t, err)

		assert.True(t, fields.Include(fieldIndex("type")))
		assert.True(t, fields.Include(fieldIndex("time")))
		assert.False(t, fields.Include(fieldIndex("elb")))
	})

	t.Run("Invalid index", func(t *testing.T) {
		fields, err := NewFields("")
		require.NoError(t, err)

		assert.False(t, fields.Include(-1))
		assert.False(t, fields.Include(len(fieldNames)))
	})
}

func fieldIndex(name string) int {
	for i, n := range fieldNames {
		if n == name {
			return i
		}
	}
	return -1
}
