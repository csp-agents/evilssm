// Copyright 2024 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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
package docmanager

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	contextmocks "github.com/aws/amazon-ssm-agent/agent/mocks/context"
	"github.com/aws/amazon-ssm-agent/agent/mocks/log"
	logmocks "github.com/aws/amazon-ssm-agent/agent/mocks/log"
	identityMocks "github.com/aws/amazon-ssm-agent/common/identity/mocks"
	"github.com/stretchr/testify/assert"
)

const (
	TEST_ORC_DIR = "orchestration"
	TEST_SEC_DIR = "session"
	TEST_DOC_DIR = "document"
	TIMING_TEST  = "timing_test"
)

func TestDocManagerLock(t *testing.T) {
	acquired := getLock(TEST_ORC_DIR)
	assert.True(t, acquired)

	acquired = getLock(TEST_ORC_DIR)
	assert.False(t, acquired)
	releaseLock(TEST_ORC_DIR)

	acquired = getLock(TEST_ORC_DIR)
	assert.True(t, acquired)
	releaseLock(TEST_ORC_DIR)
}

func TestDocManagerLockTiming(t *testing.T) {
	acquired := getLock(TIMING_TEST)
	assert.True(t, acquired)
	updateTime(TIMING_TEST)
	releaseLock(TIMING_TEST)

	acquired = getLock(TIMING_TEST)
	assert.False(t, acquired)
}

func getAndLock(name string, dc chan int, t *testing.T) {
	acquired := getLock(name)
	assert.True(t, acquired)
	updateTime(name)
	releaseLock(name)
	dc <- 0
}

func TestDocManagerLockMultipleFolders(t *testing.T) {
	dc := make(chan int)
	go getAndLock(TEST_DOC_DIR, dc, t)
	go getAndLock(TEST_SEC_DIR, dc, t)
	<-dc
	<-dc
	assert.False(t, getLock(TEST_DOC_DIR))
	assert.False(t, getLock(TEST_SEC_DIR))
}

func TestMoveDocumentState(t *testing.T) {
	tests := []struct {
		name               string
		instanceIDReturn   string
		instanceIDError    error
		createSourceFile   bool
		expectedFileInDest bool
		src                string
		dst                string
		expectedLogCalls   map[string]int
	}{
		{
			name: "Successful move",

			instanceIDReturn:   "test-instance-id",
			instanceIDError:    nil,
			createSourceFile:   true,
			expectedFileInDest: true,
			src:                "source",
			dst:                "destination",
			expectedLogCalls: map[string]int{
				"Debugf": 1,
				"Errorf": 0,
			},
		},
		{
			name:               "Move failure Source and Destination does not exsist",
			instanceIDReturn:   "test-instance-id",
			instanceIDError:    nil,
			createSourceFile:   false,
			expectedFileInDest: false,
			src:                "source",
			dst:                "destination",
			expectedLogCalls: map[string]int{
				"Debugf": 1,
				"Errorf": 0,
			},
		},
		{
			name: "Instance ID failure",

			instanceIDReturn:   "",
			instanceIDError:    fmt.Errorf("error"),
			createSourceFile:   false,
			expectedFileInDest: false,
			src:                "source",
			dst:                "destination",
			expectedLogCalls: map[string]int{
				"Debugf": 1,
				"Errorf": 1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockContext := contextmocks.NewMockDefaultWithIdentityAndLog(
				&identityMocks.IAgentIdentity{},
				logmocks.NewMockLog(),
			)
			mockIdentity := mockContext.Identity().(*identityMocks.IAgentIdentity)
			mockLog := mockContext.Log().(*logmocks.Mock)

			tempDir, err := os.MkdirTemp("", "temp")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tempDir)

			srcDir := filepath.Join(tempDir, "source")
			dstDir := filepath.Join(tempDir, "destination")
			err = os.MkdirAll(srcDir, os.ModePerm)
			assert.NoError(t, err)
			err = os.MkdirAll(dstDir, os.ModePerm)
			assert.NoError(t, err)

			testFileName := "test-document.json"

			if tt.createSourceFile {
				tt.src = srcDir
				tt.dst = dstDir

				// create source file

				testFilePath := filepath.Join(tt.src, testFileName)
				err = os.WriteFile(testFilePath, []byte("test content"), 0644)
				assert.NoError(t, err)
			}

			// Setup expectations
			mockIdentity.On("ShortInstanceID").Return(tt.instanceIDReturn, tt.instanceIDError)

			// Initialize DocumentFileMgr
			docMgr := NewDocumentFileMgr(
				mockContext,
				tempDir,
				"root",
				"state",
			)

			originalMoveFileFunc := moveFileFunc
			defer func() { moveFileFunc = originalMoveFileFunc }()

			moveFileFunc = func(filename, src, dst string) (bool, error) {
				return fileutil.MoveFile(filename, tt.src, tt.dst)
			}

			// Execute
			docMgr.MoveDocumentState(testFileName, "source", "destination")

			// Verify
			_, err = os.Stat(filepath.Join(tt.dst, testFileName))
			if tt.expectedFileInDest {
				assert.NoError(t, err, "File should exist in destination")
			} else {
				assert.Error(t, err, "File should not exist in destination")
			}

			// Verify log calls
			for method, count := range tt.expectedLogCalls {
				mockLog.AssertNumberOfCalls(t, method, count)
			}
		})
	}
}

func TestPersistDocumentState(t *testing.T) {
	tests := []struct {
		name             string
		instanceIDReturn string
		instanceIDError  error
		overwriteTest    bool
		faliureTest      bool
		expectedLogCalls map[string]int
	}{
		{
			name: "Successful persist",

			instanceIDReturn: "test-instance-id",
			instanceIDError:  nil,
			overwriteTest:    false,
			faliureTest:      false,
			expectedLogCalls: map[string]int{
				"Debugf": 1,
				"Tracef": 1,
				"Errorf": 0,
			},
		},
		{
			name: "Successful persist overwrite",

			instanceIDReturn: "test-instance-id",
			instanceIDError:  nil,
			overwriteTest:    true,
			faliureTest:      false,
			expectedLogCalls: map[string]int{
				"Debugf": 3,
				"Tracef": 2,
				"Errorf": 0,
			},
		},
		{
			name: "Instance ID failure",

			instanceIDReturn: "",
			instanceIDError:  fmt.Errorf("error"),
			overwriteTest:    false,
			faliureTest:      false,
			expectedLogCalls: map[string]int{
				"Debugf": 1,
				"Tracef": 1,
				"Errorf": 1,
			},
		},
		{
			name: "Faliure",

			instanceIDReturn: "test-instance-1",
			instanceIDError:  nil,
			overwriteTest:    false,
			faliureTest:      true,
			expectedLogCalls: map[string]int{
				"Debugf": 2,
				"Tracef": 1,
				"Errorf": 0,
			},
		},
	}
	for _, tt := range tests {
		// SetUp
		mockContext := contextmocks.NewMockDefaultWithIdentityAndLog(
			&identityMocks.IAgentIdentity{},
			logmocks.NewMockLog(),
		)
		mockIdentity := mockContext.Identity().(*identityMocks.IAgentIdentity)
		mockLog := mockContext.Log().(*logmocks.Mock)
		tempDir, err := os.MkdirTemp("", "temp")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)
		testFileName := "test-document.json"
		testFilePath := filepath.Join(tempDir, testFileName)
		if tt.faliureTest {
			err = os.WriteFile(testFilePath, []byte(""), 0444) // 0444 is read-only for all users
			if err != nil {
				t.Fatalf("Failed to create read-only file: %v", err)
			}
		}

		// Setup expectations
		mockIdentity.On("ShortInstanceID").Return(tt.instanceIDReturn, tt.instanceIDError)
		originalJoinPath := joinPathFunc
		defer func() { joinPathFunc = originalJoinPath }()
		joinPathFunc = func(elem ...string) string {
			return testFilePath
		}

		testState := contracts.DocumentState{
			DocumentInformation:        contracts.DocumentInfo{},
			DocumentType:               "Association",
			SchemaVersion:              "schema",
			InstancePluginsInformation: []contracts.PluginState{},
		}

		docMgr := NewDocumentFileMgr(
			mockContext,
			appconfig.DefaultDataStorePath,
			appconfig.DefaultDocumentRootDirName,
			appconfig.DefaultLocationOfState,
		)
		// Execute
		docMgr.PersistDocumentState("test.json", "testfolder", testState)
		if tt.overwriteTest {
			overwriteState := contracts.DocumentState{
				DocumentInformation:        contracts.DocumentInfo{},
				DocumentType:               "Association",
				SchemaVersion:              "1",
				InstancePluginsInformation: []contracts.PluginState{},
			}
			docMgr.PersistDocumentState("test.json", "testfolder", overwriteState)
		}

		// Assert
		for method, count := range tt.expectedLogCalls {
			mockLog.AssertNumberOfCalls(t, method, count)
		}

		if tt.overwriteTest {
			// Assert schema version
			content, err := os.ReadFile(testFilePath)
			if err != nil {
				t.Fatalf("Failed to read test file: %v", err)
			}
			var parsedContent map[string]interface{}
			err = json.Unmarshal([]byte(content), &parsedContent)
			if err != nil {
				t.Fatalf("Failed to parse JSON content: %v", err)
			}

			assert.Equal(t, "1", parsedContent["SchemaVersion"], "Schema version should be 1")

		}
	}
}

func TestGetDocumentStateSuccess(t *testing.T) {
	mockContext := contextmocks.NewMockDefaultWithIdentityAndLog(
		&identityMocks.IAgentIdentity{},
		logmocks.NewMockLog(),
	)
	mockIdentity := mockContext.Identity().(*identityMocks.IAgentIdentity)
	mockLog := mockContext.Log().(*logmocks.Mock)
	tempDir, err := os.MkdirTemp("", "temp")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)
	mockIdentity.On("ShortInstanceID").Return("test-instance-id", nil)

	testFileName := "test-document.json"
	testFile, err := os.CreateTemp(tempDir, testFileName)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer testFile.Close()

	originalJoinPath := joinPathFunc
	originalIsPrivilegeFile := isPrivilegedFile
	defer func() { joinPathFunc = originalJoinPath }()
	defer func() { isPrivilegedFile = originalIsPrivilegeFile }()
	joinPathFunc = func(elem ...string) string {
		return testFile.Name()
	}
	isPrivilegedFile = func(path string) (bool, error) {
		return true, nil
	}

	testState := contracts.DocumentState{
		DocumentInformation:        contracts.DocumentInfo{},
		DocumentType:               "Association",
		SchemaVersion:              "schema",
		InstancePluginsInformation: []contracts.PluginState{},
	}

	jsonContent, err := json.Marshal(testState)
	assert.NoError(t, err)

	err = os.WriteFile(testFile.Name(), jsonContent, 0600)
	assert.NoError(t, err)
	docMgr := NewDocumentFileMgr(
		mockContext,
		appconfig.DefaultDataStorePath,
		appconfig.DefaultDocumentRootDirName,
		appconfig.DefaultLocationOfState,
	)
	// Execute
	commandState := docMgr.GetDocumentState("test.json", "testfolder")
	assert.Equal(t, contracts.DocumentType("Association"), commandState.DocumentType)
	assert.Equal(t, "schema", commandState.SchemaVersion)
	mockLog.AssertNumberOfCalls(t, "Tracef", 1)
}

func TestDocumentStateDirBasicFunctionality(t *testing.T) {
	// Test basic functionality with a typical instance ID and location folder
	instanceID := "i-1234567890abcdef0"
	locationFolder := "active"

	expectedPath := filepath.Join(
		appconfig.DefaultDataStorePath,
		instanceID,
		appconfig.DefaultDocumentRootDirName,
		appconfig.DefaultLocationOfState,
		locationFolder,
	)

	actualPath := DocumentStateDir(instanceID, locationFolder)

	assert.Equal(t, expectedPath, actualPath,
		"DocumentStateDir should return the correct absolute path")
}

func TestOrchestrationDirDefaultDocument(t *testing.T) {
	// Test the default document root directory path generation
	instanceID := "i-1234567890"
	rootDirName := "testOrchestration"
	folderType := "" // Default case

	expectedPath := filepath.Join(
		appconfig.DefaultDataStorePath,
		instanceID,
		appconfig.DefaultDocumentRootDirName,
		rootDirName,
	)

	result := orchestrationDir(instanceID, rootDirName, folderType)

	assert.Equal(t, expectedPath, result,
		"Orchestration directory path for default document root should match expected path")
}

func TestOrchestrationDirSessionRoot(t *testing.T) {
	// Test the session root directory path generation
	instanceID := "i-0987654321"
	rootDirName := "sessionTest"
	folderType := appconfig.DefaultSessionRootDirName

	expectedPath := filepath.Join(
		appconfig.DefaultDataStorePath,
		instanceID,
		appconfig.DefaultSessionRootDirName,
		rootDirName,
	)

	result := orchestrationDir(instanceID, rootDirName, folderType)

	assert.Equal(t, expectedPath, result,
		"Orchestration directory path for session root should match expected path")
}

func TestIsRunCommandDirName_ValidUUID(t *testing.T) {
	// Test valid UUID format directory names
	testCases := []string{
		"a1b2c3d4-e5f6-7890-1234-567890abcdef", // standard lowercase UUID
		"A1B2C3D4-E5F6-7890-1234-567890ABCDEF", // uppercase UUID (should not match)
	}

	for _, dirName := range testCases {
		t.Run(dirName, func(t *testing.T) {
			// Only lowercase UUIDs should match
			if dirName == "a1b2c3d4-e5f6-7890-1234-567890abcdef" {
				assert.True(t, isRunCommandDirName(dirName), "Valid UUID should return true")
			} else {
				assert.False(t, isRunCommandDirName(dirName), "Uppercase UUID should return false")
			}
		})
	}
}

func TestIsRunCommandDirName_InvalidFormats(t *testing.T) {
	// Test various invalid directory name formats
	testCases := []string{
		"",                   // Empty string
		"short",              // Too short
		"a1b2c3d4-e5f6-7890", // Incomplete UUID
		"a1b2c3d4-e5f6-7890-1234-567890abcdef-extra", // Too long
		"g1h2i3j4-k5l6-7890-1234-567890abcdef",       // Invalid hex characters
		"123-456-789-012-345",                        // Numeric but wrong format
	}

	for _, dirName := range testCases {
		t.Run(dirName, func(t *testing.T) {
			assert.False(t, isRunCommandDirName(dirName),
				"Invalid directory name format should return false")
		})
	}
}

func TestIsRunCommandDirName_EdgeCases(t *testing.T) {
	// Test edge cases and boundary conditions
	testCases := []struct {
		name     string
		dirName  string
		expected bool
	}{
		{"MinimumValidUUID", "00000000-0000-0000-0000-000000000000", true},
		{"MaximumValidUUID", "ffffffff-ffff-ffff-ffff-ffffffffffff", true},
		{"MixedCaseUUID", "A1b2C3d4-E5f6-7890-1234-567890AbCdEf", false},
		{"SpacePadded", " a1b2c3d4-e5f6-7890-1234-567890abcdef ", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, isRunCommandDirName(tc.dirName),
				"UUID validation failed for %s", tc.name)
		})
	}
}

func TestIsAssociationRunDirName(t *testing.T) {
	testCases := []struct {
		name     string
		dirName  string
		expected bool
	}{
		{
			name:     "Valid Association Run Directory Name with Date",
			dirName:  "2024-02-15-example-run",
			expected: true,
		},
		{
			name:     "Valid Association Run Directory Name with Only Date",
			dirName:  "2024-02-15",
			expected: true,
		},
		{
			name:     "Invalid Directory Name - Missing Year",
			dirName:  "24-02-15",
			expected: false,
		},
		{
			name:     "Invalid Directory Name - Non-Numeric",
			dirName:  "abcd-ef-gh",
			expected: false,
		},
		{
			name:     "Invalid Directory Name - Empty String",
			dirName:  "",
			expected: false,
		},
		{
			name:     "Valid Directory Name with Additional Characters",
			dirName:  "2024-02-15-some-additional-text",
			expected: true,
		},
		{
			name:     "Invalid Directory Name with Incorrect Format",
			dirName:  "2024/02/15",
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := isAssociationRunDirName(tc.dirName)
			assert.Equal(t, tc.expected, result,
				fmt.Sprintf("Test case %s failed: expected %v for input %s",
					tc.name, tc.expected, tc.dirName))
		})
	}
}

func TestIsOlderThan_NormalCase(t *testing.T) {
	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "test-older-than")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Set modification time to 2 hours ago
	pastTime := time.Now().Add(-2 * time.Hour)
	err = os.Chtimes(tempDir, pastTime, pastTime)
	assert.NoError(t, err)

	// Create a mock logger
	mockLog := log.NewMockLog()

	// Test with 1 hour retention duration
	result := isOlderThan(mockLog, tempDir, 1)
	assert.True(t, result, "Directory should be considered older than retention duration")
}

func TestIsOlderThan_FileModificationError(t *testing.T) {
	// Create a mock logger
	mockLog := log.NewMockLog()

	// Use a non-existent path to trigger an error
	nonExistentPath := filepath.Join(os.TempDir(), "non-existent-directory")

	// Test with error scenario
	result := isOlderThan(mockLog, nonExistentPath, 1)
	assert.False(t, result, "Non-existent path should return false")
}

func TestRemoveDocumentState(t *testing.T) {
	tests := []struct {
		name             string
		instanceIDReturn string
		instanceIDError  error
		faliureTest      bool
		expectedLogCalls map[string]int
	}{
		{
			name: "Successful DocumentState",

			instanceIDReturn: "test-instance-id",
			instanceIDError:  nil,
			faliureTest:      false,
			expectedLogCalls: map[string]int{
				"Debugf": 1,
				"Errorf": 0,
			},
		}, {
			name: "Faliure DocumentState",

			instanceIDReturn: "test-instance-id",
			instanceIDError:  nil,
			faliureTest:      true,
			expectedLogCalls: map[string]int{
				"Debugf": 0,
				"Errorf": 1,
			},
		}, {
			name: "Faliure InstanceId",

			instanceIDReturn: "",
			instanceIDError:  fmt.Errorf("error"),
			faliureTest:      true,
			expectedLogCalls: map[string]int{
				"Debugf": 0,
				"Errorf": 2,
			},
		},
	}
	for _, tt := range tests {
		mockContext := contextmocks.NewMockDefaultWithIdentityAndLog(
			&identityMocks.IAgentIdentity{},
			logmocks.NewMockLog(),
		)
		mockIdentity := mockContext.Identity().(*identityMocks.IAgentIdentity)
		mockLog := mockContext.Log().(*logmocks.Mock)
		tempDir, err := os.MkdirTemp("", "temp")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		testFileName := "test-document.json"
		testFile, err := os.CreateTemp(tempDir, testFileName)
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		testFile.Close()
		if !tt.faliureTest {
			originalRemoveFunc := deleteFileFunc
			defer func() { deleteFileFunc = originalRemoveFunc }()
			deleteFileFunc = func(filepath string) error {
				return fileutil.DeleteFile(testFile.Name())
			}
		}
		// Execute
		mockIdentity.On("ShortInstanceID").Return(tt.instanceIDReturn, tt.instanceIDError)
		docMgr := NewDocumentFileMgr(
			mockContext,
			appconfig.DefaultDataStorePath,
			appconfig.DefaultDocumentRootDirName,
			appconfig.DefaultLocationOfState,
		)
		docMgr.RemoveDocumentState("", "")

		// Verify log calls
		for method, count := range tt.expectedLogCalls {
			mockLog.AssertNumberOfCalls(t, method, count)
		}
		_, err = os.Stat(testFile.Name())
		if tt.faliureTest {
			assert.False(t, os.IsNotExist(err))
		} else {
			assert.True(t, os.IsNotExist(err))
		}
	}
}

func TestGetOrchestrationDirectoryNames_SuccessfulRetrieval(t *testing.T) {
	mockContext := contextmocks.NewMockDefaultWithIdentityAndLog(
		&identityMocks.IAgentIdentity{},
		logmocks.NewMockLog(),
	)
	mockLog := mockContext.Log().(*logmocks.Mock)
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "test-orchestration")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create some test subdirectories
	testDirs := []string{"dir1", "dir2", "dir3"}
	for _, dir := range testDirs {
		err := os.Mkdir(filepath.Join(tempDir, dir), 0755)
		assert.NoError(t, err)
	}

	// Override orchestrationDir function to return our temp directory
	originalFileExsistsFunc := fileExistsFunc
	originalgetDirectoryNameFunc := getDirectoryNamesFunc
	defer func() {
		fileExistsFunc = originalFileExsistsFunc
		getDirectoryNamesFunc = originalgetDirectoryNameFunc
	}()
	fileExistsFunc = func(filepath string) bool {
		return fileutil.Exists(tempDir)
	}
	getDirectoryNamesFunc = func(srcPath string) (directories []string, err error) {
		return fileutil.GetDirectoryNames(tempDir)
	}

	// Call the function
	_, dirNames, err := getOrchestrationDirectoryNames(mockLog, "test-instance", "test-root", appconfig.DefaultDocumentRootDirName)

	// Assertions
	assert.NoError(t, err)
	assert.ElementsMatch(t, testDirs, dirNames)
}

func TestGetOrchestrationDirectoryNames_FailedRetrieval(t *testing.T) {
	mockContext := contextmocks.NewMockDefaultWithIdentityAndLog(
		&identityMocks.IAgentIdentity{},
		logmocks.NewMockLog(),
	)
	mockLog := mockContext.Log().(*logmocks.Mock)
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "test-orchestration")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create some test subdirectories
	testDirs := []string{"dir1", "dir2", "dir3"}
	for _, dir := range testDirs {
		err := os.Mkdir(filepath.Join(tempDir, dir), 0755)
		assert.NoError(t, err)
	}
	// Call the function
	_, dirNames, err := getOrchestrationDirectoryNames(mockLog, "test-instance", "test-root", appconfig.DefaultDocumentRootDirName)

	// Assertions
	assert.NoError(t, err)
	assert.Empty(t, dirNames)
	mockLog.AssertNumberOfCalls(t, "Debugf", 1)
}

func TestCleanAssociationDirectorySucess(t *testing.T) {
	mockContext := contextmocks.NewMockDefaultWithIdentityAndLog(
		&identityMocks.IAgentIdentity{},
		logmocks.NewMockLog(),
	)
	mockLog := mockContext.Log().(*logmocks.Mock)
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "test-orchestration")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	createMockAssociationRunDirectories(tempDir, 3, time.Now().Add(-25*time.Hour))

	// Call the function
	canDelete, deletedCount := cleanupAssociationDirectory(mockLog, 0, tempDir, 24)

	// Assertions
	assert.True(t, canDelete, "Should be able to delete directories")
	assert.Equal(t, 3, deletedCount, "Should have deleted 3 directories")

	remainingDirs, err := fileutil.GetDirectoryNames(tempDir)
	assert.NoError(t, err)
	assert.Len(t, remainingDirs, 0, "All directories should be deleted")
}

func TestCleanupAssociationDirectoryMaxDeletionLimit(t *testing.T) {
	mockContext := contextmocks.NewMockDefaultWithIdentityAndLog(
		&identityMocks.IAgentIdentity{},
		logmocks.NewMockLog(),
	)
	mockLog := mockContext.Log().(*logmocks.Mock)
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "test-orchestration")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)
	// Create more directories than max deletion limit
	createMockAssociationRunDirectories(tempDir, maxOrchestrationDirectoryDeletions+5, time.Now().Add(-25*time.Hour))

	// Call the function
	canDelete, deletedCount := cleanupAssociationDirectory(mockLog, 0, tempDir, 24)

	// Assertions
	assert.False(t, canDelete, "Should not be able to delete all directories")
	assert.Equal(t, maxOrchestrationDirectoryDeletions, deletedCount, "Should have deleted max allowed directories")
}

func TestCleanupAssociationDirectoryWithNonAssociationDirs(t *testing.T) {
	mockContext := contextmocks.NewMockDefaultWithIdentityAndLog(
		&identityMocks.IAgentIdentity{},
		logmocks.NewMockLog(),
	)
	mockLog := mockContext.Log().(*logmocks.Mock)
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "test-orchestration")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)
	// Create mix of association and non-association directories
	createMockAssociationRunDirectories(tempDir, 3, time.Now().Add(-25*time.Hour))
	os.MkdirAll(filepath.Join(tempDir, "non-association-dir"), 0755)

	// Call the function
	canDelete, deletedCount := cleanupAssociationDirectory(mockLog, 0, tempDir, 24)

	// Assertions
	assert.True(t, canDelete, "Should be able to delete directories")
	assert.Equal(t, 3, deletedCount, "Should have deleted only association directories")

	// Verify non-association directory remains
	remainingDirs, err := fileutil.GetDirectoryNames(tempDir)
	assert.NoError(t, err)
	assert.Len(t, remainingDirs, 1, "Non-association directory should remain")
	assert.Equal(t, "non-association-dir", remainingDirs[0], "Unexpected remaining directory")
}

func TestIsLegacyAssociationDirectory_EmptyDirectory(t *testing.T) {
	// Create a temporary empty directory
	tempDir, err := os.MkdirTemp("", "empty-test-dir")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create a mock logger
	mockLog := log.NewMockLog()

	// Test empty directory
	isLegacy, err := isLegacyAssociationDirectory(mockLog, tempDir)
	assert.NoError(t, err)
	assert.False(t, isLegacy, "Empty directory should not be considered a legacy association directory")
}

func TestIsLegacyAssociationDirectory_ValidAssociationRunDir(t *testing.T) {
	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "legacy-assoc-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create a valid association run directory (2024-02-15 format)
	validDirName := "2024-02-15-some-run"
	err = os.Mkdir(filepath.Join(tempDir, validDirName), 0755)
	assert.NoError(t, err)

	// Create a mock logger
	mockLog := log.NewMockLog()

	// Test directory with valid association run directory
	isLegacy, err := isLegacyAssociationDirectory(mockLog, tempDir)
	assert.NoError(t, err)
	assert.True(t, isLegacy, "Directory with valid association run directory should be considered legacy")
}

func TestDeleteOldOrchestrationDirectoriesSuccess(t *testing.T) {
	mockContext := contextmocks.NewMockDefaultWithIdentityAndLog(
		&identityMocks.IAgentIdentity{},
		logmocks.NewMockLog(),
	)
	mockLog := mockContext.Log().(*logmocks.Mock)
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "test-orchestration")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)
	createMockAssociationRunDirectories(tempDir, 3, time.Now().Add(-25*time.Hour))

	originalFileExsistsFunc := fileExistsFunc
	originaljoinPathFunc := joinPathFunc
	originalgetDirectoryNameFunc := getDirectoryNamesFunc
	defer func() {
		fileExistsFunc = originalFileExsistsFunc
		getDirectoryNamesFunc = originalgetDirectoryNameFunc
		joinPathFunc = originaljoinPathFunc
	}()
	fileExistsFunc = func(filepath string) bool {
		return fileutil.Exists(tempDir)
	}
	getDirectoryNamesFunc = func(srcPath string) (directories []string, err error) {
		return fileutil.GetDirectoryNames(tempDir)
	}
	joinPathFunc = func(elem ...string) string {
		return filepath.Join(tempDir)
	}

	// Call the function
	DeleteOldOrchestrationDirectories(mockLog, "", tempDir, 24, 24)

	// Assert
	remainingDirs, err := fileutil.GetDirectoryNames(tempDir)
	assert.NoError(t, err)
	assert.Len(t, remainingDirs, 0, "All directories should be deleted")
}

func TestDeleteSessionOrchestrationDirectoriesSuccess(t *testing.T) {
	mockContext := contextmocks.NewMockDefaultWithIdentityAndLog(
		&identityMocks.IAgentIdentity{},
		logmocks.NewMockLog(),
	)
	mockLog := mockContext.Log().(*logmocks.Mock)
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "test-orchestration")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)
	createMockAssociationRunDirectories(tempDir, 1, time.Now().Add(-25*time.Hour))

	originalFileExsistsFunc := fileExistsFunc
	originaljoinPathFunc := joinPathFunc
	originalgetDirectoryNameFunc := getDirectoryNamesFunc
	defer func() {
		fileExistsFunc = originalFileExsistsFunc
		getDirectoryNamesFunc = originalgetDirectoryNameFunc
		joinPathFunc = originaljoinPathFunc
	}()
	fileExistsFunc = func(filepath string) bool {
		return fileutil.Exists(tempDir)
	}
	getDirectoryNamesFunc = func(srcPath string) (directories []string, err error) {
		return fileutil.GetDirectoryNames(tempDir)
	}
	joinPathFunc = func(elem ...string) string {
		return filepath.Join(tempDir)
	}

	// Call the function
	DeleteSessionOrchestrationDirectories(mockLog, "", tempDir, 24)

	// Assert
	remainingDirs, err := fileutil.GetDirectoryNames(tempDir)
	assert.NoError(t, err)
	assert.Len(t, remainingDirs, 0, "All directories should be deleted")
}

// Helper Functions
func createMockAssociationRunDirectories(baseDir string, count int, modTime time.Time) {
	for i := 0; i < count; i++ {
		dirPath := filepath.Join(baseDir, fmt.Sprintf("2024-02-%02d-test", i+1))
		err := os.MkdirAll(dirPath, 0755)
		if err != nil {
			panic(err)
		}
		_, err1 := os.Stat(dirPath)
		fmt.Printf("checking if for path %v, Error: %v", dirPath, err1)
		// Modify directory modification time
		os.Chtimes(dirPath, modTime, modTime)
	}
}
