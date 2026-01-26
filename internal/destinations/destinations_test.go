package destinations

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name               string
		config             string
		expectErr          bool
		expectDestinations int
	}{
		{
			name:               "Single valid destination - stdout",
			config:             "stdout",
			expectErr:          false,
			expectDestinations: 1,
		},
		{
			name:               "Unsupported destination type",
			config:             "unsupported",
			expectErr:          true,
			expectDestinations: 0,
		},
		{
			name:               "Mixed valid and invalid destinations",
			config:             "stdout,unsupported",
			expectErr:          false,
			expectDestinations: 1,
		},
		{
			name:               "Empty destination configuration",
			config:             "",
			expectErr:          true,
			expectDestinations: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockSession := &session.Session{}

			destinations, err := New(tc.config, mockSession)

			if tc.expectErr {
				assert.Error(t, err)
				assert.Nil(t, destinations)
			} else {
				assert.NoError(t, err)
				assert.Len(t, destinations, tc.expectDestinations)
			}
		})
	}
}
