package outputs

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
		expectOutputs int
	}{
		{
			name:          "Single valid output - stdout",
			config:        "stdout",
			expectErr:     false,
			expectOutputs: 1,
		},
		{
			name:          "Unsupported output type",
			config:        "unsupported",
			expectErr:     true,
			expectOutputs: 0,
		},
		{
			name:          "Mixed valid and invalid outputs",
			config:        "stdout,unsupported",
			expectErr:     false,
			expectOutputs: 1,
		},
		{
			name:          "Empty output configuration",
			config:        "",
			expectErr:     true,
			expectOutputs: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockSession := &session.Session{}

			outputs, err := New(tc.config, mockSession)

			if tc.expectErr {
				assert.Error(t, err)
				assert.Nil(t, outputs)
			} else {
				assert.NoError(t, err)
				assert.Len(t, outputs, tc.expectOutputs)
			}
		})
	}
}
