package targets

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name          string
		config        string
		expectErr     bool
		expectTargets int
	}{
		{
			name:          "Single valid target - stdout",
			config:        "stdout",
			expectErr:     false,
			expectTargets: 1,
		},
		{
			name:          "Unsupported target type",
			config:        "unsupported",
			expectErr:     true,
			expectTargets: 0,
		},
		{
			name:          "Mixed valid and invalid targets",
			config:        "stdout,unsupported",
			expectErr:     false,
			expectTargets: 1,
		},
		{
			name:          "Empty target configuration",
			config:        "",
			expectErr:     true,
			expectTargets: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockSession := &session.Session{}

			targets, err := New(tc.config, mockSession)

			if tc.expectErr {
				assert.Error(t, err)
				assert.Nil(t, targets)
			} else {
				assert.NoError(t, err)
				assert.Len(t, targets, tc.expectTargets)
			}
		})
	}
}
