package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	contextmocks "github.com/aws/amazon-ssm-agent/agent/mocks/context"
	logmocks "github.com/aws/amazon-ssm-agent/agent/mocks/log"
	"github.com/stretchr/testify/assert"
)

func TestRegisteredPluginsNoConfig(t *testing.T) {
	mockContext := contextmocks.NewMockDefault()
	plugins := RegisteredPlugins(mockContext)
	assert.NotNil(t, plugins)
	if runtime.GOOS == "windows" {
		assert.Equal(t, 1, len(plugins))
	} else {
		assert.Equal(t, 0, len(plugins))
	}

}

func TestDuplicateRegisteredPlugins(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	tempDirLoc, err := os.MkdirTemp("", "test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test configuration file
	testfile, err := os.CreateTemp(tempDir, "test.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	defer testfile.Close()

	testfile1, err := os.CreateTemp(tempDir, "test1.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	defer testfile1.Close()

	packageFile, err := os.CreateTemp(tempDirLoc, "testPackage.txt")
	if err != nil {
		t.Fatalf("Failed to create temp package file: %v", err)
	}
	defer packageFile.Close()

	packagePath := packageFile.Name()

	packageContent := fmt.Sprintf(`{
        "name": "testDaemon",
        "action": "start",
        "packagelocation":%q,
        "command": "test command"
    }`, packagePath)

	if _, err = packageFile.WriteString(packageContent); err != nil {
		t.Fatalf("Failed to write to package file: %v", err)
	}
	if err = packageFile.Sync(); err != nil {
		t.Fatalf("Failed to sync package file: %v", err)
	}

	if runtime.GOOS == "windows" {
		packagePath = filepath.ToSlash(packagePath)
	}

	if _, err = testfile.WriteString(packageContent); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	if err = testfile.Sync(); err != nil {
		t.Fatalf("Failed to sync temp file: %v", err)
	}

	if _, err = testfile1.WriteString(packageContent); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	if err = testfile1.Sync(); err != nil {
		t.Fatalf("Failed to sync temp file: %v", err)
	}

	originalDaemonRoot := appconfig.DaemonRoot
	defer func() { appconfig.DaemonRoot = originalDaemonRoot }()
	appconfig.DaemonRoot = tempDir

	mockLog := logmocks.NewMockLog()
	mockContext := contextmocks.NewMockDefaultWithOwnLogMock(mockLog)
	plugins := RegisteredPlugins(mockContext)
	assert.NotNil(t, plugins)
	if runtime.GOOS == "windows" {
		assert.Equal(t, 2, len(plugins))
		mockLog.AssertNumberOfCalls(t, "Errorf", 1)
		mockLog.AssertNumberOfCalls(t, "Debugf", 4)
	} else {
		assert.Equal(t, 1, len(plugins))
		mockLog.AssertNumberOfCalls(t, "Errorf", 1)
		mockLog.AssertNumberOfCalls(t, "Debugf", 3)
	}

}

func TestInvalidRegisteredPlugins(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test configuration file
	testfile, err := os.CreateTemp(tempDir, "test.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	testfile1, err := os.CreateTemp(tempDir, "test1.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	jsonContent := (`{
        "name": "testDaemon",
    }`)

	_, err = testfile.WriteString(jsonContent)

	_, err = testfile1.WriteString(jsonContent)

	originalDaemonRoot := appconfig.DaemonRoot
	defer func() { appconfig.DaemonRoot = originalDaemonRoot }()
	appconfig.DaemonRoot = tempDir

	mockLog := logmocks.NewMockLog()
	mockContext := contextmocks.NewMockDefaultWithOwnLogMock(mockLog)
	plugins := RegisteredPlugins(mockContext)
	if runtime.GOOS == "windows" {
		assert.Equal(t, 1, len(plugins))
	} else {
		assert.Equal(t, 0, len(plugins))
	}
}
