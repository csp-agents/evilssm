package manager

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/longrunning/datastore"
	"github.com/aws/amazon-ssm-agent/agent/longrunning/plugin"
	contextmocks "github.com/aws/amazon-ssm-agent/agent/mocks/context"
	logmocks "github.com/aws/amazon-ssm-agent/agent/mocks/log"
	identityMocks "github.com/aws/amazon-ssm-agent/common/identity/mocks"
	"github.com/stretchr/testify/assert"
)

func Test_DataStoreLocation(t *testing.T) {
	// Create mocks
	mockContext := contextmocks.NewMockDefaultWithIdentityAndLog(
		identityMocks.NewDefaultMockAgentIdentity(),
		logmocks.NewMockLog(),
	)
	mockIdentity := mockContext.Identity().(*identityMocks.IAgentIdentity)

	// Setup expectations
	mockIdentity.On("ShortInstanceID").Return("test-instance-id", nil)

	// Perform write
	location, fileName, err := getDataStoreLocation(mockContext)

	// Assertions
	assert.NoError(t, err)
	assert.NotEmpty(t, location)
	assert.NotEmpty(t, fileName)

}

func TestWrite_SuccessfulWrite(t *testing.T) {

	tempDir, err := os.MkdirTemp("", "test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	mockContext := contextmocks.NewMockDefaultWithIdentityAndLog(
		identityMocks.NewDefaultMockAgentIdentity(),
		logmocks.NewMockLog(),
	)
	mockIdentity := mockContext.Identity().(*identityMocks.IAgentIdentity)

	originalGetDataStoreLocation := getDataStoreLocationFunc
	defer func() {
		getDataStoreLocationFunc = originalGetDataStoreLocation
	}()

	getDataStoreLocationFunc = func(context context.T) (location string, filename string, err error) {
		return tempDir, filepath.Join(tempDir, "test.json"), nil
	}

	testData := map[string]plugin.PluginInfo{
		"testPlugin": {
			Name: "testPlugin",
		},
	}

	dataStore := ds{
		context: mockContext,
		dsImpl:  datastore.FsStore{},
	}

	// Setup expectations
	mockIdentity.On("ShortInstanceID").Return("test-instance-id", nil)

	err = dataStore.Write(testData)
	assert.NoError(t, err)
}

func TestWrite_FailedRead(t *testing.T) {

	tempDir, err := os.MkdirTemp("", "test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	mockContext := contextmocks.NewMockDefaultWithIdentityAndLog(
		identityMocks.NewDefaultMockAgentIdentity(),
		logmocks.NewMockLog(),
	)
	mockIdentity := mockContext.Identity().(*identityMocks.IAgentIdentity)

	originalGetDataStoreLocation := getDataStoreLocationFunc
	defer func() {
		getDataStoreLocationFunc = originalGetDataStoreLocation
	}()

	getDataStoreLocationFunc = func(context context.T) (location string, filename string, err error) {
		return tempDir, "test.json", fmt.Errorf("error")
	}

	testData := map[string]plugin.PluginInfo{
		"testPlugin": {
			Name: "testPlugin",
		},
	}

	dataStore := ds{
		context: mockContext,
		dsImpl:  datastore.FsStore{},
	}

	// Setup expectations
	mockIdentity.On("ShortInstanceID").Return("test-instance-id", nil)

	err = dataStore.Write(testData)
	assert.Error(t, err)
}

func TestWrite_FailedWrite(t *testing.T) {

	tempDir, err := os.MkdirTemp("", "test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	mockContext := contextmocks.NewMockDefaultWithIdentityAndLog(
		identityMocks.NewDefaultMockAgentIdentity(),
		logmocks.NewMockLog(),
	)
	mockIdentity := mockContext.Identity().(*identityMocks.IAgentIdentity)

	originalGetDataStoreLocation := getDataStoreLocationFunc
	defer func() {
		getDataStoreLocationFunc = originalGetDataStoreLocation
	}()

	getDataStoreLocationFunc = func(context context.T) (location string, filename string, err error) {
		return tempDir, "test.json", fmt.Errorf("error")
	}

	testData := map[string]plugin.PluginInfo{
		"testPlugin": {
			Name: "testPlugin",
		},
	}

	dataStore := ds{
		context: mockContext,
		dsImpl:  datastore.FsStore{},
	}

	// Setup expectations
	mockIdentity.On("ShortInstanceID").Return("test-instance-id", nil)

	err = dataStore.Write(testData)
	assert.Error(t, err)

}

func TestRead_SuccessfulRead(t *testing.T) {

	tempDir, err := os.MkdirTemp("", "test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	defer os.RemoveAll(tempDir)

	testfile, err := os.CreateTemp(tempDir, "test.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	_, err = testfile.WriteString(`{"testPlugin":{"Name":"testPlugin"}}`)

	defer testfile.Close()

	mockContext := contextmocks.NewMockDefaultWithIdentityAndLog(
		identityMocks.NewDefaultMockAgentIdentity(),
		logmocks.NewMockLog(),
	)
	mockIdentity := mockContext.Identity().(*identityMocks.IAgentIdentity)

	originalGetDataStoreLocation := getDataStoreLocationFunc
	defer func() {
		getDataStoreLocationFunc = originalGetDataStoreLocation
	}()

	getDataStoreLocationFunc = func(context context.T) (location string, filename string, err error) {
		return tempDir, testfile.Name(), nil
	}

	testData := map[string]plugin.PluginInfo{
		"testPlugin": {
			Name: "testPlugin",
		},
	}

	dataStore := ds{
		context: mockContext,
		dsImpl:  datastore.FsStore{},
	}

	// Setup expectations
	mockIdentity.On("ShortInstanceID").Return("test-instance-id", nil)

	readData, err := dataStore.Read()
	assert.NoError(t, err)
	assert.Equal(t, testData, readData)
}

func TestRead_FailedRead(t *testing.T) {

	tempDir, err := os.MkdirTemp("", "test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	defer os.RemoveAll(tempDir)

	testfile, err := os.CreateTemp(tempDir, "test.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	_, err = testfile.WriteString(`{"testPlugin":{"Name":"testPlugin"}}`)

	defer testfile.Close()

	mockContext := contextmocks.NewMockDefaultWithIdentityAndLog(
		identityMocks.NewDefaultMockAgentIdentity(),
		logmocks.NewMockLog(),
	)
	mockIdentity := mockContext.Identity().(*identityMocks.IAgentIdentity)

	originalGetDataStoreLocation := getDataStoreLocationFunc
	defer func() {
		getDataStoreLocationFunc = originalGetDataStoreLocation
	}()

	getDataStoreLocationFunc = func(context context.T) (location string, filename string, err error) {
		return tempDir, testfile.Name(), fmt.Errorf("error")
	}

	dataStore := ds{
		context: mockContext,
		dsImpl:  datastore.FsStore{},
	}

	// Setup expectations
	mockIdentity.On("ShortInstanceID").Return("test-instance-id", nil)

	_, err = dataStore.Read()
	assert.Error(t, err)
}
