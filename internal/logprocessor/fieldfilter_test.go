package logprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFieldFilter(t *testing.T) {
	t.Run("ALB no fields provided includes all", func(t *testing.T) {
		filter, err := NewFieldFilter(LBTypeALB, "")
		require.NoError(t, err)

		for i := range albFields {
			assert.True(t, filter.Includes(i))
		}
	})

	t.Run("NLB no fields provided includes all", func(t *testing.T) {
		filter, err := NewFieldFilter(LBTypeNLB, "")
		require.NoError(t, err)

		for i := range nlbFields {
			assert.True(t, filter.Includes(i))
		}
	})

	t.Run("ALB valid fields provided", func(t *testing.T) {
		filter, err := NewFieldFilter(LBTypeALB, "type,time,elb")
		require.NoError(t, err)

		assert.True(t, filter.Includes(fieldIndex(albFields, "type")))
		assert.True(t, filter.Includes(fieldIndex(albFields, "time")))
		assert.True(t, filter.Includes(fieldIndex(albFields, "elb")))
		assert.False(t, filter.Includes(fieldIndex(albFields, "client:port")))
	})

	t.Run("NLB valid fields provided", func(t *testing.T) {
		filter, err := NewFieldFilter(LBTypeNLB, "type,version,time")
		require.NoError(t, err)

		assert.True(t, filter.Includes(fieldIndex(nlbFields, "type")))
		assert.True(t, filter.Includes(fieldIndex(nlbFields, "version")))
		assert.True(t, filter.Includes(fieldIndex(nlbFields, "time")))
		assert.False(t, filter.Includes(fieldIndex(nlbFields, "elb")))
	})

	t.Run("Invalid field provided", func(t *testing.T) {
		_, err := NewFieldFilter(LBTypeALB, "invalid_field")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid field name")
	})

	t.Run("Invalid LB type", func(t *testing.T) {
		_, err := NewFieldFilter("invalid", "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid load balancer type")
	})

	t.Run("ALB field on NLB fails", func(t *testing.T) {
		_, err := NewFieldFilter(LBTypeNLB, "user_agent") // ALB-only field
		require.Error(t, err)
	})
}

func TestFieldFilterName(t *testing.T) {
	filter, err := NewFieldFilter(LBTypeALB, "")
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

		_, ok = filter.Name(filter.TotalFields())
		assert.False(t, ok)
	})
}

func TestFieldFilterIncludes(t *testing.T) {
	t.Run("Include all ALB fields", func(t *testing.T) {
		filter, err := NewFieldFilter(LBTypeALB, "")
		require.NoError(t, err)

		for i := range albFields {
			assert.True(t, filter.Includes(i))
		}
	})

	t.Run("Include specific fields", func(t *testing.T) {
		filter, err := NewFieldFilter(LBTypeALB, "type,time")
		require.NoError(t, err)

		assert.True(t, filter.Includes(fieldIndex(albFields, "type")))
		assert.True(t, filter.Includes(fieldIndex(albFields, "time")))
		assert.False(t, filter.Includes(fieldIndex(albFields, "elb")))
	})

	t.Run("Invalid index", func(t *testing.T) {
		filter, err := NewFieldFilter(LBTypeALB, "")
		require.NoError(t, err)

		assert.False(t, filter.Includes(-1))
		assert.False(t, filter.Includes(filter.TotalFields()))
	})
}

func TestTotalFields(t *testing.T) {
	albFilter, _ := NewFieldFilter(LBTypeALB, "")
	nlbFilter, _ := NewFieldFilter(LBTypeNLB, "")

	assert.Equal(t, 30, albFilter.TotalFields()) // ALB has 30 fields
	assert.Equal(t, 24, nlbFilter.TotalFields()) // NLB TLS has 24 fields
}

func fieldIndex(fields []string, name string) int {
	for i, n := range fields {
		if n == name {
			return i
		}
	}
	return -1
}
