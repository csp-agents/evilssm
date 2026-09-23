// Copyright 2023 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package common contains common constants and functions needed to be accessed across ssm-setup-cli
package configurationmanager

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/aws/amazon-ssm-agent/common/identity/availableidentities/onprem"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	logmocks "github.com/aws/amazon-ssm-agent/agent/mocks/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// Define ConfigManager TestSuite struct
type ConfigManagerTestSuite struct {
	suite.Suite
	logMock *logmocks.Mock
}

// Initialize the ConfigManagerTestSuite test suite struct
func (suite *ConfigManagerTestSuite) SetupTest() {
	logMock := logmocks.NewMockLog()
	suite.logMock = logMock

}

func (suite *ConfigManagerTestSuite) TestConfigManager_IsConfigAvailable_ValidPaths() {
	// Arrange
	configMgr := New()
	expectedOutput := true
	fileExists = func(filePath string) bool {
		// Configuration already exists on device
		if strings.Contains(filePath, agentConfigFolderPath) {
			return true
		}
		return false
	}

	unmarshalFile = func(filePath string, dest interface{}) (err error) {
		if _, ok := dest.(*appconfig.SsmagentConfig); !ok {
			assert.FailNow(suite.T(), "Unmarshalling to malformed struct")
		}

		if !strings.Contains(filePath, agentConfigFolderPath) {
			assert.FailNow(suite.T(), "Incorrect config filepath unmarshalled")
		}

		return nil
	}

	// Act
	actualOutput, err := configMgr.IsConfigAvailable("")

	// Assert
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), expectedOutput, actualOutput, "config availability check failed")

	// Arrange
	expectedOutput = true
	fileExists = func(filePath string) bool {
		// No configuration already existing on device
		if !strings.Contains(filePath, agentConfigFolderPath) {
			return true
		}
		return false
	}

	unmarshalFile = func(filePath string, dest interface{}) (err error) {
		if _, ok := dest.(*appconfig.SsmagentConfig); !ok {
			assert.FailNow(suite.T(), "Unmarshalling to malformed struct")
		}

		if strings.Contains(filePath, agentConfigFolderPath) {
			assert.FailNow(suite.T(), "Incorrect config filepath unmarshalled")
		}

		return nil
	}

	// Act
	actualOutput, err = configMgr.IsConfigAvailable("testPath1")

	// Assert
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), expectedOutput, actualOutput, "config availability check failed")
}

func (suite *ConfigManagerTestSuite) TestConfigManager_IsConfigAvailable_InvalidPaths() {
	// Arrange
	configMgr := New()
	expectedOutput := false
	fileExists = func(filePath string) bool {
		if strings.Contains(filePath, agentConfigFolderPath) {
			// Custom configuration doesnt exist on device
			return false
		}
		return true
	}

	unmarshalFile = func(filePath string, dest interface{}) (err error) {
		if _, ok := dest.(*appconfig.SsmagentConfig); !ok {
			assert.FailNow(suite.T(), "Unmarshalling to malformed struct")
		}

		if strings.Contains(filePath, agentConfigFolderPath) {
			assert.FailNow(suite.T(), "Incorrect config filepath unmarshalled")
		}

		return fmt.Errorf("invalid file content")
	}

	// Act
	actualOutput, err := configMgr.IsConfigAvailable("")

	// Assert
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), expectedOutput, actualOutput, "config availability check failed")

	// Arrange
	expectedOutput = false
	fileExists = func(filePath string) bool {
		if strings.Contains(filePath, agentConfigFolderPath) {
			// Custom configuration doesnt exist on device
			return false
		}
		return true
	}

	unmarshalFile = func(filePath string, dest interface{}) (err error) {
		if _, ok := dest.(*appconfig.SsmagentConfig); !ok {
			assert.FailNow(suite.T(), "Unmarshalling to malformed struct")
		}

		if strings.Contains(filePath, agentConfigFolderPath) {
			assert.FailNow(suite.T(), "Incorrect config filepath unmarshalled")
		}

		return fmt.Errorf("invalid file content")
	}

	// Act
	actualOutput, err = configMgr.IsConfigAvailable("testPath1")

	// Assert
	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), expectedOutput, actualOutput, "config availability check failed")
}

func (suite *ConfigManagerTestSuite) TestConfigManager_ConfigureAgent_Success() {
	configMgr := New()
	osOpen = func(name string) (*os.File, error) {
		return &os.File{}, nil
	}
	makeDir = func(destinationDir string) (err error) {
		return nil
	}
	osCreate = func(name string) (*os.File, error) {
		return &os.File{}, nil
	}
	ioCopy = func(dst io.Writer, src io.Reader) (written int64, err error) {
		return 0, nil
	}
	actualOutput := configMgr.ConfigureAgent("testPath1")
	assert.Equal(suite.T(), nil, actualOutput, "config availability check failed")
}

func (suite *ConfigManagerTestSuite) TestConfigManager_ConfigureAgent_Failure() {
	configMgr := New()
	osOpen = func(name string) (*os.File, error) {
		return nil, fmt.Errorf("err1")
	}
	actualOutput := configMgr.ConfigureAgent("testPath1")
	assert.Equal(suite.T(), "err1", actualOutput.Error(), "config availability check succeeded")

	osOpen = func(name string) (*os.File, error) {
		return nil, nil
	}
	makeDir = func(destinationDir string) (err error) {
		return fmt.Errorf("err2")
	}
	actualOutput = configMgr.ConfigureAgent("testPath1")
	assert.Equal(suite.T(), "err2", actualOutput.Error(), "config availability check succeeded")

	makeDir = func(destinationDir string) (err error) {
		return nil
	}
	osCreate = func(name string) (*os.File, error) {
		return nil, fmt.Errorf("err3")
	}
	actualOutput = configMgr.ConfigureAgent("testPath1")
	assert.Equal(suite.T(), "err3", actualOutput.Error(), "config availability check succeeded")

	osCreate = func(name string) (*os.File, error) {
		return nil, nil
	}
	ioCopy = func(dst io.Writer, src io.Reader) (written int64, err error) {
		return 0, fmt.Errorf("err4")
	}
	actualOutput = configMgr.ConfigureAgent("testPath1")
	assert.Equal(suite.T(), "err4", actualOutput.Error(), "config availability check succeeded")
}

func (suite *ConfigManagerTestSuite) TestConfigManager_GetExistingAgentConfigData() {
	expectedOutput := []interface{}{"OnPrem", "EC2"}
	var agentIdentityConfig appconfig.IdentityCfg
	agentIdentityConfig.ConsumptionOrder = []string{"OnPrem", "EC2"}
	readAllText = func(filePath string) (text string, err error) {
		return jsonutil.Marshal(agentIdentityConfig)
	}
	actualOutput, err := getExistingAgentConfigData("testPath1")
	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), expectedOutput, actualOutput["ConsumptionOrder"], "config availability check succeeded")
}

func (suite *ConfigManagerTestSuite) TestConfigManager_CreateUpdateAgentConfigWithOnPremIdentity_FileExists() {
	makeDir = func(destinationDir string) (err error) {
		if destinationDir == agentConfigFolderPath {
			return nil
		}
		return fmt.Errorf("error while creating directory")
	}

	fileExists = func(filePath string) bool {
		return true
	}
	var agentConfig appconfig.SsmagentConfig
	var agentIdentityConfig appconfig.IdentityCfg
	agentIdentityConfig.ConsumptionOrder = []string{"OnPrem", "EC2"}
	agentConfig.Identity = agentIdentityConfig
	readAllText = func(filePath string) (text string, err error) {
		return jsonutil.Marshal(agentConfig)
	}
	expectedOutput := ""
	fileWrite = func(absolutePath, content string, perm os.FileMode) (result bool, err error) {
		expectedOutput = content
		return true, nil
	}
	configMgr := New()
	err := configMgr.CreateUpdateAgentConfigWithOnPremIdentity()

	output := make(map[string]interface{})
	err = jsonutil.Unmarshal(expectedOutput, &output)

	identityMap := output["Identity"].(map[string]interface{})
	assert.Equal(suite.T(), identityMap["ConsumptionOrder"], []interface{}{onprem.IdentityType})
	assert.Contains(suite.T(), expectedOutput, "Agent")
	assert.Nil(suite.T(), err)
}

func (suite *ConfigManagerTestSuite) TestConfigManager_CreateUpdateAgentConfigWithOnPremIdentity_FileNotExists() {
	makeDir = func(destinationDir string) (err error) {
		if destinationDir == agentConfigFolderPath {
			return nil
		}
		return fmt.Errorf("error while creating directory")
	}

	fileExists = func(filePath string) bool {
		return false
	}
	var agentConfig appconfig.SsmagentConfig
	var agentIdentityConfig appconfig.IdentityCfg
	agentIdentityConfig.ConsumptionOrder = []string{"OnPrem", "EC2"}
	agentConfig.Identity = agentIdentityConfig
	readAllText = func(filePath string) (text string, err error) {
		return jsonutil.Marshal(agentConfig)
	}
	expectedOutput := ""
	fileWrite = func(absolutePath, content string, perm os.FileMode) (result bool, err error) {
		expectedOutput = content
		return true, nil
	}
	configMgr := New()
	err := configMgr.CreateUpdateAgentConfigWithOnPremIdentity()

	output := make(map[string]interface{})
	err = jsonutil.Unmarshal(expectedOutput, &output)
	identityMap := output["Identity"].(map[string]interface{})
	assert.Equal(suite.T(), identityMap["ConsumptionOrder"], []interface{}{onprem.IdentityType})
	assert.NotContains(suite.T(), expectedOutput, "Agent")
	assert.Nil(suite.T(), err)
}

func (suite *ConfigManagerTestSuite) TestConfigManager_UpdateAgentConfigWithUseDualStackEndpoint_FileExists() {
	makeDir = func(destinationDir string) (err error) {
		return nil
	}
	fileExists = func(filePath string) bool {
		return true
	}

	existingConfig := map[string]interface{}{
		"Agent": map[string]interface{}{
			"Region":               "us-west-2",
			"UseDualStackEndpoint": false,
		},
		"Ssm": map[string]interface{}{
			"HealthFrequencyMinutes": 5,
		},
	}

	readAllText = func(filePath string) (text string, err error) {
		configData, _ := json.Marshal(existingConfig)
		return string(configData), nil
	}

	expectedOutput := ""
	fileWrite = func(absolutePath, content string, perm os.FileMode) (result bool, err error) {
		expectedOutput = content
		return true, nil
	}

	configMgr := New()
	err := configMgr.UpdateAgentConfigWithUseDualStackEndpoint()

	assert.NoError(suite.T(), err)

	var config map[string]interface{}
	err = json.Unmarshal([]byte(expectedOutput), &config)
	assert.NoError(suite.T(), err)

	agentConfig, ok := config["Agent"].(map[string]interface{})
	assert.True(suite.T(), ok)
	assert.Equal(suite.T(), true, agentConfig["UseDualStackEndpoint"])
	assert.Equal(suite.T(), "us-west-2", agentConfig["Region"]) // Existing values preserved

	// Verify other sections preserved
	ssmConfig, ok := config["Ssm"].(map[string]interface{})
	assert.True(suite.T(), ok)
	assert.Equal(suite.T(), float64(5), ssmConfig["HealthFrequencyMinutes"])
}

func (suite *ConfigManagerTestSuite) TestConfigManager_UpdateAgentConfigWithUseDualStackEndpoint_FileNotExists() {
	makeDir = func(destinationDir string) (err error) {
		return nil
	}
	fileExists = func(filePath string) bool {
		return false
	}
	expectedOutput := ""
	fileWrite = func(absolutePath, content string, perm os.FileMode) (result bool, err error) {
		expectedOutput = content
		return true, nil
	}

	configMgr := New()
	err := configMgr.UpdateAgentConfigWithUseDualStackEndpoint()

	assert.NoError(suite.T(), err)

	var config map[string]interface{}
	err = json.Unmarshal([]byte(expectedOutput), &config)
	assert.NoError(suite.T(), err)

	agentConfig, ok := config["Agent"].(map[string]interface{})
	assert.True(suite.T(), ok)
	assert.Equal(suite.T(), true, agentConfig["UseDualStackEndpoint"])
}

func TestConfigManagerTestSuite(t *testing.T) {
	suite.Run(t, new(ConfigManagerTestSuite))
}
