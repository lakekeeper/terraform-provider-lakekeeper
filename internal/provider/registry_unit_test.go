package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnitSplitInternalID(t *testing.T) {
	testCases := []struct {
		name          string
		input         string
		expectedPart1 string
		expectedPart2 string
		expectErr     bool
	}{
		{
			name:          "valid composite ID",
			input:         "project123/warehouse456",
			expectedPart1: "project123",
			expectedPart2: "warehouse456",
			expectErr:     false,
		},
		{
			name:          "valid UUID composite ID",
			input:         "00000000-0000-0000-0000-000000000000/11111111-1111-1111-1111-111111111111",
			expectedPart1: "00000000-0000-0000-0000-000000000000",
			expectedPart2: "11111111-1111-1111-1111-111111111111",
			expectErr:     false,
		},
		{
			name:      "missing slash separator",
			input:     "warehouse456",
			expectErr: true,
		},
		{
			name:      "empty string",
			input:     "",
			expectErr: true,
		},
		{
			name:      "leading slash only",
			input:     "/warehouse456",
			expectErr: true,
		},
		{
			name:      "trailing slash only",
			input:     "project123/",
			expectErr: true,
		},
		{
			name:      "too many parts",
			input:     "project123/warehouse456/extra",
			expectErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			part1, part2, err := splitInternalID(types.StringValue(tc.input))
			if tc.expectErr {
				require.Error(t, err)
				assert.Empty(t, part1)
				assert.Empty(t, part2)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expectedPart1, part1)
				assert.Equal(t, tc.expectedPart2, part2)
			}
		})
	}
}
