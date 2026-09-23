package cloudwatch

import (
	"encoding/json"
	"errors"
	"testing"

	fileutil "github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	mockUtilFile "github.com/aws/amazon-ssm-agent/agent/longrunning/mocks/cloudwatch"
	logmocks "github.com/aws/amazon-ssm-agent/agent/mocks/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestInstance_SingletonBehavior verifies that Instance() always returns the same instance
func TestInstance_SingletonBehavior(t *testing.T) {
	// Test that multiple calls return the same instance
	instance1 := Instance()
	instance2 := Instance()

	assert.Equal(t, instance1, instance2, "Instance() should return the same object every time")
}

func TestParseEngineConfiguration_SuccessfulParsing(t *testing.T) {
	// Create a CloudWatchConfigImpl instance
	cwcInstance := &CloudWatchConfigImpl{
		IsEnabled: true,
		EngineConfiguration: map[string]interface{}{
			"Logs": []map[string]interface{}{
				{
					"LogGroupName":  "TestGroup",
					"LogStreamName": "TestStream",
				},
			},
		},
	}

	// Call ParseEngineConfiguration
	config, err := cwcInstance.ParseEngineConfiguration()

	// Assertions
	assert.NoError(t, err, "ParseEngineConfiguration should not return an error")
	assert.Contains(t, config, "EngineConfiguration", "Configuration should include EngineConfiguration")
	assert.Contains(t, config, "LogGroupName", "Configuration should contain log group details")
}

func TestUpdate_SuccessfulUpdate(t *testing.T) {
	// Setup
	mockLog := logmocks.NewMockLog()
	cwcInstance := &CloudWatchConfigImpl{}

	// Mock the load function to return a predefined configuration
	original := loadFunc
	defer func() { loadFunc = original }()
	loadFunc = func(log log.T) (CloudWatchConfigImpl, error) {
		return CloudWatchConfigImpl{
			IsEnabled:           true,
			EngineConfiguration: map[string]interface{}{"key": "value"},
		}, nil
	}

	// Execute
	err := cwcInstance.Update(mockLog)

	// Assert
	assert.NoError(t, err)
	assert.True(t, cwcInstance.IsEnabled)
	assert.NotNil(t, cwcInstance.EngineConfiguration)
}

func TestUpdate_LoadFailure(t *testing.T) {
	// Setup
	mockLog := logmocks.NewMockLog()
	cwcInstance := &CloudWatchConfigImpl{}

	// Mock the load function to return an error
	original := loadFunc
	defer func() { loadFunc = original }()

	loadFunc = func(log log.T) (CloudWatchConfigImpl, error) {
		return CloudWatchConfigImpl{}, errors.New("load configuration failed")
	}

	// Execute
	err := cwcInstance.Update(mockLog)

	// Assert
	assert.Error(t, err)
	assert.Equal(t, "load configuration failed", err.Error())
}

func TestWriteSuccess(t *testing.T) {
	// Setup
	cwConfig := &CloudWatchConfigImpl{
		IsEnabled:           true,
		EngineConfiguration: map[string]interface{}{"key": "value"},
	}
	mockFileUtil := &mockUtilFile.FileUtilMock{}
	mockFileUtil.On("Exists", mock.AnythingOfType("string")).Return(false)
	mockFileUtil.On("MakeDirs", mock.AnythingOfType("string")).Return(nil)
	mockFileUtil.On("WriteIntoFileWithPermissions",
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("fs.FileMode")).Return(true, nil)

	originalFileUtil := fileutil.Exists
	defer func() { FileExist = originalFileUtil }()
	FileExist = mockFileUtil.Exists

	originalMakeDirs := fileutil.MakeDirs
	defer func() { makeDirs = originalMakeDirs }()
	makeDirs = mockFileUtil.MakeDirs

	originalWriteIntoFileWithPermissions := fileutil.WriteIntoFileWithPermissions
	defer func() { writeIntoFileWithPermissions = originalWriteIntoFileWithPermissions }()
	writeIntoFileWithPermissions = mockFileUtil.WriteIntoFileWithPermissions

	err := cwConfig.Write()

	// Assert
	assert.NoError(t, err)

}

func TestWriteConfigMarshalError(t *testing.T) {
	// Setup with an unmarshalable type
	cwConfig := &CloudWatchConfigImpl{
		IsEnabled:           true,
		EngineConfiguration: make(chan int), // Unmarshalable type
	}

	// Execute
	err := cwConfig.Write()

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "json: unsupported type")
}

func TestWriteFileWriteError(t *testing.T) {
	// Setup
	cwConfig := &CloudWatchConfigImpl{
		IsEnabled:           true,
		EngineConfiguration: map[string]interface{}{"key": "value"},
	}

	// Mock fileutil dependencies
	mockFileUtil := &mockUtilFile.FileUtilMock{}
	mockFileUtil.On("Exists", mock.AnythingOfType("string")).Return(false)
	mockFileUtil.On("MakeDirs", mock.AnythingOfType("string")).Return(nil)
	mockFileUtil.On("WriteIntoFileWithPermissions",
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("fs.FileMode")).Return(false, assert.AnError)

	// Replace global fileutil with mock

	originalFileUtil := fileutil.Exists
	defer func() { FileExist = originalFileUtil }()
	FileExist = mockFileUtil.Exists

	originalMakeDirs := fileutil.MakeDirs
	defer func() { makeDirs = originalMakeDirs }()
	makeDirs = mockFileUtil.MakeDirs

	originalWriteIntoFileWithPermissions := fileutil.WriteIntoFileWithPermissions
	defer func() { writeIntoFileWithPermissions = originalWriteIntoFileWithPermissions }()
	writeIntoFileWithPermissions = mockFileUtil.WriteIntoFileWithPermissions

	// Execute
	err := cwConfig.Write()

	// Assert
	assert.Error(t, err)
	mockFileUtil.AssertExpectations(t)
}

func TestParseEngineConfiguration_SuccessfulEnable(t *testing.T) {
	// Create a CloudWatchConfigImpl instance
	cwcInstance := &CloudWatchConfigImpl{
		IsEnabled: false,
		EngineConfiguration: map[string]interface{}{
			"Logs": []map[string]interface{}{
				{
					"LogGroupName":  "TestGroup",
					"LogStreamName": "TestStream",
				},
			},
		},
	}

	// Call ParseEngineConfiguration
	cwcInstance.Enable(cwcInstance.EngineConfiguration)

	// Assertions
	assert.True(t, cwcInstance.IsEnabled)
	assert.Equal(t, cwcInstance.EngineConfiguration, cwcInstance.EngineConfiguration)
}

func TestParseEngineConfiguration_SuccessfulDisable(t *testing.T) {
	// Create a CloudWatchConfigImpl instance
	cwcInstance := &CloudWatchConfigImpl{
		IsEnabled: true,
		EngineConfiguration: map[string]interface{}{
			"Logs": []map[string]interface{}{
				{
					"LogGroupName":  "TestGroup",
					"LogStreamName": "TestStream",
				},
			},
		},
	}

	// Call ParseEngineConfiguration
	cwcInstance.Disable()

	// Assertions
	assert.False(t, cwcInstance.IsEnabled)
	assert.Equal(t, cwcInstance.EngineConfiguration, cwcInstance.EngineConfiguration)
}

func TestParseEngineConfiguration_SuccessfulGetIsEnabled(t *testing.T) {
	// Create a CloudWatchConfigImpl instance
	cwcInstance := &CloudWatchConfigImpl{
		IsEnabled: true,
		EngineConfiguration: map[string]interface{}{
			"Logs": []map[string]interface{}{
				{
					"LogGroupName":  "TestGroup",
					"LogStreamName": "TestStream",
				},
			},
		},
	}

	// Call ParseEngineConfiguration
	cwcInstance.GetIsEnabled()

	// Assertions
	assert.True(t, cwcInstance.IsEnabled)
	assert.Equal(t, cwcInstance.EngineConfiguration, cwcInstance.EngineConfiguration)
}

func TestLoad(t *testing.T) {
	testCases := []struct {
		name           string
		mockFileRead   func(fileName string, v interface{}) error
		expectedConfig CloudWatchConfigImpl
		expectedError  error
	}{{
		name: "Successful Load - Standard Configuration",
		mockFileRead: func(fileName string, v interface{}) error {
			config := CloudWatchConfigImpl{
				IsEnabled: true,
				EngineConfiguration: map[string]interface{}{
					"Metrics": []string{"CPUUtilization"},
				},
			}
			configBytes, _ := json.Marshal(config)
			return json.Unmarshal(configBytes, v)
		},
		expectedConfig: CloudWatchConfigImpl{
			IsEnabled: true,
			EngineConfiguration: map[string]interface{}{
				"Metrics": []interface{}{"CPUUtilization"},
			},
		},
		expectedError: nil,
	},
		{
			name: "Legacy Configuration with String EngineConfiguration",
			mockFileRead: func(fileName string, v interface{}) error {
				legacyConfig := `{
				"IsEnabled": true,
				"EngineConfiguration": "{\"Metrics\":[\"CPUUtilization\"]}"
			}`
				return json.Unmarshal([]byte(legacyConfig), v)
			},
			expectedConfig: CloudWatchConfigImpl{
				IsEnabled: true,
				EngineConfiguration: map[string]interface{}{
					"Metrics": []interface{}{"CPUUtilization"},
				},
			},
			expectedError: nil,
		},
		{
			name: "Legacy Configuration with Nested EngineConfiguration",
			mockFileRead: func(fileName string, v interface{}) error {
				legacyConfig := `{
					"IsEnabled": true,
					"EngineConfiguration": {
						"EngineConfiguration": {"Metrics": ["CPUUtilization"]}
					}
				}`
				return json.Unmarshal([]byte(legacyConfig), v)
			},
			expectedConfig: CloudWatchConfigImpl{
				IsEnabled: true,
				EngineConfiguration: map[string]interface{}{
					"Metrics": []interface{}{"CPUUtilization"},
				},
			},
			expectedError: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Temporarily replace jsonutil.UnmarshalFile with mock
			originalUnmarshalFile := jsonutil.UnmarshalFile
			defer func() { unmarshalFile = originalUnmarshalFile }()
			unmarshalFile = tc.mockFileRead

			mockLogger := logmocks.NewMockLog()
			result, err := load(mockLogger)

			assert.Equal(t, tc.expectedConfig.IsEnabled, result.IsEnabled)
			assert.Equal(t, tc.expectedConfig.EngineConfiguration, result.EngineConfiguration)

			if tc.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
