// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

package artifact

import (
	"net/url"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/mocks/log"
	"github.com/aws/amazon-ssm-agent/agent/s3util"
	"github.com/stretchr/testify/assert"
)

func TestConvertToS3DualStackURL(t *testing.T) {
	logger := log.NewMockLog()

	testCases := []struct {
		name        string
		originalURL string
		expected    string
	}{
		{
			name:        "Invalid S3 URL - no change",
			originalURL: "https://unit.test.com/file.zip",
			expected:    "https://unit.test.com/file.zip",
		},
		{
			name:        "VPCE path-style URL - no change",
			originalURL: "https://bucket.vpce-12345.s3.us-east-1.vpce.amazonaws.com/mybucket/mykey",
			expected:    "https://bucket.vpce-12345.s3.us-east-1.vpce.amazonaws.com/mybucket/mykey",
		},
		{
			name:        "VPCE virtual-hosted-style URL - no change",
			originalURL: "https://mybucket.vpce-12345.s3.us-east-1.vpce.amazonaws.com/mykey",
			expected:    "https://mybucket.vpce-12345.s3.us-east-1.vpce.amazonaws.com/mykey",
		},
		{
			name:        "Already dual-stack path-style URL - no change",
			originalURL: "https://s3.dualstack.us-east-1.amazonaws.com/mybucket/mykey",
			expected:    "https://s3.dualstack.us-east-1.amazonaws.com/mybucket/mykey",
		},
		{
			name:        "Already dual-stack virtual-hosted-style URL - no change",
			originalURL: "https://mybucket.s3.dualstack.us-west-2.amazonaws.com/mykey",
			expected:    "https://mybucket.s3.dualstack.us-west-2.amazonaws.com/mykey",
		},
		{
			name:        "Global path-style URL - no change",
			originalURL: "https://s3.amazonaws.com/mybucket/mykey",
			expected:    "https://s3.amazonaws.com/mybucket/mykey",
		},
		{
			name:        "Global virtual-hosted-style URL - no change",
			originalURL: "https://mybucket.s3.amazonaws.com/mykey",
			expected:    "https://mybucket.s3.amazonaws.com/mykey",
		},
		{
			name:        "Regional path-style URL conversion",
			originalURL: "https://s3.us-east-1.amazonaws.com/mybucket/mykey",
			expected:    "https://s3.dualstack.us-east-1.amazonaws.com/mybucket/mykey",
		},
		{
			name:        "Regional virtual-hosted-style URL conversion",
			originalURL: "https://mybucket.s3.us-west-2.amazonaws.com/mykey",
			expected:    "https://mybucket.s3.dualstack.us-west-2.amazonaws.com/mykey",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			parsedURL, err := url.Parse(tc.originalURL)
			assert.NoError(t, err)

			s3URL := s3util.ParseAmazonS3URL(logger, parsedURL)
			result := ConvertToS3DualStackURL(tc.originalURL, s3URL)
			assert.Equal(t, tc.expected, result)
		})
	}
}
