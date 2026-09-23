package datastore

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/longrunning/plugin"
	"github.com/stretchr/testify/assert"
)

// TestRead_FileNotExists verifies the behavior when the datastore file does not exist
func TestRead_FileIsEmpty(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "datastore-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create a non-existent file path
	nonExistentFile := filepath.Join(tempDir, "emptyfile.json")

	// Initialize FsStore
	store := &FsStore{}

	// Attempt to read from non-existent file
	result, err := store.Read(nonExistentFile)

	// Verify expectations
	assert.NoError(t, err)
	assert.Empty(t, result)
}

// TestRead_ValidDataStore tests reading a valid datastore file
func TestRead_ValidDataStore(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "datastore-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Prepare test data
	testData := map[string]plugin.PluginInfo{
		"TestPlugin": {
			Name: "TestPlugin",
		},
	}

	// Create a temporary file with test data
	testFile := filepath.Join(tempDir, "datastore.json")
	store := &FsStore{}
	err = store.Write(testData, tempDir, testFile)
	assert.NoError(t, err)

	// Read the datastore
	result, err := store.Read(testFile)

	// Verify expectations
	assert.NoError(t, err)
	assert.Equal(t, testData, result)
}

// TestRead_InvalidJSONFile tests reading a file with invalid JSON
func TestRead_InvalidJSONFile(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "datastore-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create an invalid JSON file
	invalidFile := filepath.Join(tempDir, "invalid.json")
	err = os.WriteFile(invalidFile, []byte("{invalid json}"), 0644)
	assert.NoError(t, err)

	// Initialize FsStore
	store := &FsStore{}

	// Attempt to read invalid JSON file
	result, err := store.Read(invalidFile)

	// Verify expectations
	assert.Error(t, err)
	assert.Empty(t, result)
}

func TestFsStore_Write(t *testing.T) {
	// Setup test cases
	testCases := []struct {
		name          string
		data          map[string]plugin.PluginInfo
		location      string
		fileName      string
		expectedError bool
		setupFunc     func() error
		cleanupFunc   func() error
	}{
		{
			name: "Successful Write",
			data: map[string]plugin.PluginInfo{
				"testPlugin": {
					Name: "test-id",
				},
			},
			location:      os.TempDir(),
			fileName:      filepath.Join(os.TempDir(), "test_write.json"),
			expectedError: false,
			cleanupFunc: func() error {
				return os.Remove(filepath.Join(os.TempDir(), "test_write.json"))
			},
		},
		{
			name: "Write to Non-Existent Directory",
			data: map[string]plugin.PluginInfo{
				"testPlugin": {
					Name: "test-id",
				},
			},
			location:      filepath.Join(os.TempDir(), "non_existent_dir"),
			fileName:      filepath.Join(os.TempDir(), "non_existent_dir", "test_write.json"),
			expectedError: false,
			cleanupFunc: func() error {
				return os.RemoveAll(filepath.Join(os.TempDir(), "non_existent_dir"))
			},
		},
		{
			name:          "Write with Empty Data",
			data:          map[string]plugin.PluginInfo{},
			location:      os.TempDir(),
			fileName:      filepath.Join(os.TempDir(), "empty_write.json"),
			expectedError: false,
			cleanupFunc: func() error {
				return os.Remove(filepath.Join(os.TempDir(), "empty_write.json"))
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			store := &FsStore{}

			// If a setup function is provided, run it
			if tc.setupFunc != nil {
				err := tc.setupFunc()
				assert.NoError(t, err, "Setup should not fail")
			}

			// Perform the write operation
			err := store.Write(tc.data, tc.location, tc.fileName)

			// Assertions
			if tc.expectedError {
				assert.Error(t, err, "Write should fail")
			} else {
				assert.NoError(t, err, "Write should succeed")

				// Verify file was created
				_, statErr := os.Stat(tc.fileName)
				assert.NoError(t, statErr, "File should exist after write")
			}

			// Cleanup
			if tc.cleanupFunc != nil {
				cleanupErr := tc.cleanupFunc()
				assert.NoError(t, cleanupErr, "Cleanup should not fail")
			}
		})
	}
}
