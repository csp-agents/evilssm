// Copyright 2025 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// either express or implied. See the License for the specific language governing
// permissions and limitations under the License.
package metric

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnit_String(t *testing.T) {
	tests := []struct {
		name     string
		unit     Unit
		expected string
	}{
		{
			name:     "Empty unit",
			unit:     Unit(""),
			expected: "",
		},
		{
			name:     "Count unit",
			unit:     Unit("count"),
			expected: "count",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.unit.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}
