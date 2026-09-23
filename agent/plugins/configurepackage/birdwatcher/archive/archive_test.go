package archive

import (
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/birdwatcher"
	"github.com/stretchr/testify/assert"
)

func TestParseManifest(t *testing.T) {
	testCases := []struct {
		name           string
		input          []byte
		expectedOutput *birdwatcher.Manifest
		expectError    bool
		errorMessage   string
	}{
		{
			name: "Valid Manifest",
			input: []byte(`{
				"schemaVersion": "1.0",
				"packageArn": "arn:aws:ssm:us-east-1:123456789012:package/test-package",
				"version": "1.0.0",
				"packages": {
					"linux": {
						"1.0.0": {
							"amd64": {}
						}
					}
				},
				"files": {
					"example.txt": {
						"checksums": {
							"sha256": "abc123"
						},
						"downloadLocation": "s3://my-bucket/example.txt",
						"size": 1024
					}
				}
			}`),
			expectedOutput: &birdwatcher.Manifest{
				SchemaVersion: "1.0",
				PackageArn:    "arn:aws:ssm:us-east-1:123456789012:package/test-package",
				Version:       "1.0.0",
				Packages: map[string]map[string]map[string]*birdwatcher.PackageInfo{
					"linux": {
						"1.0.0": {
							"amd64": &birdwatcher.PackageInfo{},
						},
					},
				},
				Files: map[string]*birdwatcher.FileInfo{
					"example.txt": &birdwatcher.FileInfo{
						Checksums: map[string]string{
							"sha256": "abc123",
						},
						DownloadLocation: "s3://my-bucket/example.txt",
						Size:             1024,
					},
				},
			},
			expectError: false,
		},
		{
			name:           "Empty JSON",
			input:          []byte(`{}`),
			expectedOutput: &birdwatcher.Manifest{},
			expectError:    false,
		},
		{
			name:           "Invalid JSON",
			input:          []byte(`{invalid json`),
			expectedOutput: nil,
			expectError:    true,
			errorMessage:   "failed to decode manifest",
		},
		{
			name:           "Nil Input",
			input:          nil,
			expectedOutput: nil,
			expectError:    true,
			errorMessage:   "failed to decode manifest",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			manifest, err := ParseManifest(&tc.input)

			if tc.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.errorMessage)
				assert.Nil(t, manifest)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, manifest)

				// Deep comparison of manifest contents
				assert.Equal(t, tc.expectedOutput, manifest)
			}
		})
	}
}

func TestFormKey(t *testing.T) {
	testCases := []struct {
		name           string
		packageName    string
		packageVersion string
		expectedKey    string
	}{
		{
			name:           "Normal Case",
			packageName:    "test-package",
			packageVersion: "1.0.0",
			expectedKey:    "test-package_1.0.0",
		},
		{
			name:           "Empty Package Name",
			packageName:    "",
			packageVersion: "1.0.0",
			expectedKey:    "_1.0.0",
		},
		{
			name:           "Empty Version",
			packageName:    "test-package",
			packageVersion: "",
			expectedKey:    "test-package_",
		},
		{
			name:           "Both Empty",
			packageName:    "",
			packageVersion: "",
			expectedKey:    "_",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			key := FormKey(tc.packageName, tc.packageVersion)
			assert.Equal(t, tc.expectedKey, key)
		})
	}
}
