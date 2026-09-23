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
package utils

import (
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/mocks/context"

	"github.com/cihub/seelog"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

const (
	testingDir = "./testingvar"

	testCharset = "abcdefghijklmnopqrstuvwxyz" +
		"ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789🖥️🏢🚢!@#$%^&*"
)

var seededRand *rand.Rand = rand.New(rand.NewSource(time.Now().UnixNano()))

func randomString(length int) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = testCharset[seededRand.Intn(len(testCharset))]
	}
	return string(b)
}

type rollingUtilsTestSuite struct {
	suite.Suite

	ctx *context.Mock
}

// TestRollingUtilsSuite executes test suite
func TestRollingUtilsSuite(t *testing.T) {
	suite.Run(t, new(rollingUtilsTestSuite))
}

// SetupTest makes sure that all the components referenced in the
// test case are initialized before each test
func (suite *rollingUtilsTestSuite) SetupTest() {
	err := fileutil.MakeDirs(testingDir)
	require.NoError(suite.T(), err)

	suite.ctx = context.NewMockDefault()
}

func (suite *rollingUtilsTestSuite) TearDownTest() {
	testingDir := filepath.Clean(testingDir)
	fileutil.DeleteDirectory(testingDir)
}

//nolint:gocognit,funlen
func (suite *rollingUtilsTestSuite) TestRollingUtils() {
	baseDir := filepath.Join(testingDir, "logs")

	for i, testCase := range ruTestCases {
		suite.Run(fmt.Sprintf("Test case %v", i), func() {
			cleanupRollingUtilsTest()

			expectedNamepaceLogsMap := make(map[string][]string)
			writtenLineCount := 0

			namespaces := make([]string, 0, len(testCase.namespaces))

			// create namespaces
			for namespace, config := range testCase.namespaces {
				namespaces = append(namespaces, namespace)

				// create directory for the namespace
				namespaceDir := filepath.Join(baseDir, namespace)
				os.MkdirAll(namespaceDir, 0700)

				logs := make([]string, 0, config.linesToWrite)

				// prepare log lines
				for i := 0; i < config.linesToWrite; i++ {
					line := fmt.Sprintf("%v %v", randomString(rand.Intn(50)), i+1)

					logs = append(logs, line)
				}

				expectedNamepaceLogsMap[namespace] = logs

				// create rolling files. seelog will handle the rolling for us
				// a high maxFiles number will make sure that all files are kept on disk
				loggerConfig := getRollingUtilsTestLoggerConfig(namespaceDir, "logs", 100000000, 700)
				seelogger, err := seelog.LoggerFromConfigAsBytes(loggerConfig)
				require.NoError(suite.T(), err)
				for _, line := range logs {
					seelogger.Debug(line)
				}
				seelogger.Close()

				writtenLineCount += len(logs)
			}

			sort.Strings(namespaces)

			// remaining lines on disk
			remainingLineCount := writtenLineCount

			for remainingLineCount > 0 {
				fetchedNamespaceLines, err := ReadAndDeleteRollingLogs(suite.ctx.Log(), baseDir, "logs", testCase.chunkFetchLimit)

				fetchedLineCount := 0
				for _, lines := range fetchedNamespaceLines {
					fetchedLineCount += len(lines)
				}

				if remainingLineCount <= testCase.chunkFetchLimit {
					assert.Equal(suite.T(), remainingLineCount, fetchedLineCount)
					assert.Equal(suite.T(), io.EOF, err)
				} else {
					assert.Equal(suite.T(), testCase.chunkFetchLimit, fetchedLineCount)
					assert.NoError(suite.T(), err)
				}

				i := 0
				for i < fetchedLineCount {
					for _, namespace := range namespaces {
						expectedLines := expectedNamepaceLogsMap[namespace]

						// first line is the newest written line
						actualLines := fetchedNamespaceLines[namespace]

						for j := range actualLines {
							// assert that the fetched line matches what was written
							assert.Equal(suite.T(), expectedLines[len(expectedLines)-j-1], actualLines[j])

							i++

							if i >= fetchedLineCount {
								goto out
							}
						}
					}
				}
			out:

				// delete from expected lines map for the next fetch iteration
				toDeleteCount := fetchedLineCount
				for _, namespace := range namespaces {
					expectedLines := expectedNamepaceLogsMap[namespace]
					toDeleteFromThisNamespace := min(len(expectedLines), toDeleteCount)

					expectedNamepaceLogsMap[namespace] = expectedLines[:(len(expectedLines) - toDeleteFromThisNamespace)]

					toDeleteCount -= toDeleteFromThisNamespace

					if len(expectedNamepaceLogsMap[namespace]) == 0 {
						namespaceDir := filepath.Join(baseDir, namespace)
						files, err := os.ReadDir(namespaceDir)
						assert.NoError(suite.T(), err)
						assert.Equal(suite.T(), 1, len(files))

						// ensure that the basic "logs" file is present and is empty. We don't want the logger to throw an error
						// if it's writing to it
						fileInfo, err := os.Stat(filepath.Join(namespaceDir, "logs"))
						assert.NoError(suite.T(), err)
						assert.False(suite.T(), fileInfo.IsDir())
						assert.Equal(suite.T(), int64(0), fileInfo.Size())
					}
				}

				remainingLineCount -= fetchedLineCount
			}
		})
	}
}

func cleanupRollingUtilsTest() {
	testingDir := filepath.Clean(testingDir)
	fileutil.DeleteDirectory(testingDir)
}

func getRollingUtilsTestLoggerConfig(defaultLogDir, logFile string, maxRolls int, maxFileSize int64) []byte {
	logFilePath := filepath.Join(defaultLogDir, logFile)
	logConfig := `
<seelog type="sync" minlevel="trace">
    <outputs formatid="common"> `
	logConfig += `<rollingfile type="size" filename="` + logFilePath + `" maxsize="` + strconv.FormatInt(maxFileSize, 10) + `" maxrolls="` + strconv.Itoa(maxRolls) + `"/>
    </outputs>
    <formats>
        <format id="common" format="%Msg%n"/>
    </formats>
</seelog>
`
	return []byte(logConfig)
}

// test config for a specific namespace
type ruNamespaceTestConfig struct {
	linesToWrite int
}

// test case config for rolling log collector
type ruTestCase struct {
	chunkFetchLimit int
	namespaces      map[string]*ruNamespaceTestConfig
}

func createRollingUtilsNamespaceTestConfig(linesToWrite int) *ruNamespaceTestConfig {
	return &ruNamespaceTestConfig{linesToWrite}
}

func createRollingUtilsTestCase(
	chunkFetchLimit int,
	namespaces map[string]*ruNamespaceTestConfig) *ruTestCase {
	return &ruTestCase{chunkFetchLimit, namespaces}
}

var ruTestCases = []*ruTestCase{
	createRollingUtilsTestCase(100, map[string]*ruNamespaceTestConfig{"namespace1": createRollingUtilsNamespaceTestConfig(0)}),

	createRollingUtilsTestCase(100, map[string]*ruNamespaceTestConfig{"namespace1": createRollingUtilsNamespaceTestConfig(100)}),

	createRollingUtilsTestCase(2, map[string]*ruNamespaceTestConfig{"namespace1": createRollingUtilsNamespaceTestConfig(1),
		"namespace2": createRollingUtilsNamespaceTestConfig(1)}),

	createRollingUtilsTestCase(500, map[string]*ruNamespaceTestConfig{"namespace1": createRollingUtilsNamespaceTestConfig(100),
		"namespace2": createRollingUtilsNamespaceTestConfig(100)}),

	createRollingUtilsTestCase(100, map[string]*ruNamespaceTestConfig{"namespace1": createRollingUtilsNamespaceTestConfig(100),
		"namespace2": createRollingUtilsNamespaceTestConfig(100)}),

	createRollingUtilsTestCase(150, map[string]*ruNamespaceTestConfig{"namespace1": createRollingUtilsNamespaceTestConfig(100),
		"namespace2": createRollingUtilsNamespaceTestConfig(100)}),

	createRollingUtilsTestCase(30, map[string]*ruNamespaceTestConfig{"namespace1": createRollingUtilsNamespaceTestConfig(100),
		"namespace2": createRollingUtilsNamespaceTestConfig(100)}),
}
