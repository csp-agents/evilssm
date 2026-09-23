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
package rolling

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/mocks/context"
	dynamicconfiguration "github.com/aws/amazon-ssm-agent/agent/telemetry/dynamic_configuration"
	"github.com/aws/amazon-ssm-agent/common/telemetry/telemetrylog"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

const (
	testingDir = "./testingvar"

	// all tests rely on this
	testLogEntrySize = 122
)

type rollingLogCollectorTestSuite struct {
	suite.Suite

	backupGetBaseLogStoreDir func() string

	testLogEntry telemetrylog.Entry
}

// TestRollingLogCollectorSuite executes test suite
func TestRollingLogCollectorSuite(t *testing.T) {
	suite.Run(t, new(rollingLogCollectorTestSuite))
}

// SetupTest makes sure that all the components referenced in the
// test case are initialized before each test
func (suite *rollingLogCollectorTestSuite) SetupTest() {
	err := fileutil.MakeDirs(testingDir)
	assert.NoError(suite.T(), err)

	// we will repeatedly write the same log entry
	suite.testLogEntry = telemetrylog.Entry{
		Time:     time.Date(2025, time.January, 5, 13, 34, 23, 5672, time.UTC),
		Severity: telemetrylog.ERROR,
		Body:     "qiuyeuq%dqw%vkjd%%qi+-s$`dwew`jd%ssdqwsd\"\\'''\\'\n",
	}

	jsonEntry, _ := json.Marshal(suite.testLogEntry)

	require.Equal(suite.T(), testLogEntrySize, len(jsonEntry))

	// mock the function which returns the base logs directory
	mockGetBaseLogStoreDir := func() string {
		return filepath.Join(testingDir, "logs")
	}
	suite.backupGetBaseLogStoreDir = getBaseLogStoreDir
	getBaseLogStoreDir = mockGetBaseLogStoreDir
}

func (suite *rollingLogCollectorTestSuite) TestCorrectDataIsWritten() {
	// maxRolls = 1  each file 1MB
	dynamicconfiguration.MaxRolls = func(string) int { return 10 }
	dynamicconfiguration.MaxRollSize = func(string) int64 { return 1024 * 1024 }

	defer func() {
		dynamicconfiguration.MaxRolls = dynamicconfiguration.GetMaxRolls
		dynamicconfiguration.MaxRollSize = dynamicconfiguration.GetMaxRollSize
	}()
	testCollector := NewRollingLogCollector(context.NewMockDefault(), "logs")
	defer testCollector.Close()

	_, err := testCollector.getLogCollector("testNamespace")
	require.NoError(suite.T(), err)

	namespaceDir := filepath.Join(testingDir, "logs", "testNamespace")

	for i := 0; i < 100; i++ {
		testCollector.CollectLog("testNamespace", suite.testLogEntry)
	}
	testCollector.Flush()

	// assert that only 1 files was written
	files, err := os.ReadDir(namespaceDir)
	require.NoError(suite.T(), err)
	require.Len(suite.T(), files, 1)

	// read the written data back
	file, err := os.Open(filepath.Join(namespaceDir, "logs"))
	if err != nil {
		suite.T().Fatal(err)
	}
	defer file.Close()

	// Create a new scanner
	scanner := bufio.NewScanner(file)

	// Loop through the file and read each line
	countEntries := 0
	for scanner.Scan() {
		line := scanner.Text()

		var writtenLogEntry telemetrylog.Entry
		err = json.Unmarshal([]byte(line), &writtenLogEntry)
		assert.NoError(suite.T(), err)

		// assert that the written log entry is the same as the one we wrote
		require.Equal(suite.T(), suite.testLogEntry, writtenLogEntry)

		countEntries++
	}

	// Check for errors during the scan
	assert.NoError(suite.T(), scanner.Err())

	assert.Equal(suite.T(), 100, countEntries)
}

func (suite *rollingLogCollectorTestSuite) TearDownTest() {
	getBaseLogStoreDir = suite.backupGetBaseLogStoreDir

	testingDir := filepath.Clean(testingDir)
	fileutil.DeleteDirectory(testingDir)
}

func (suite *rollingLogCollectorTestSuite) TestFetchAndDrop() {
	// maxRolls = 1  each file 1MB
	dynamicconfiguration.MaxRolls = func(string) int { return 10 }
	dynamicconfiguration.MaxRollSize = func(string) int64 { return 1024 * 1024 }

	defer func() {
		dynamicconfiguration.MaxRolls = dynamicconfiguration.GetMaxRolls
		dynamicconfiguration.MaxRollSize = dynamicconfiguration.GetMaxRollSize
	}()
	testCollector := NewRollingLogCollector(context.NewMockDefault(), "testlogs")
	defer testCollector.Close()

	startTimeStamp := time.Now().UTC()

	for i := 0; i < 100; i++ {
		logEntry := telemetrylog.Entry{
			Time:     startTimeStamp.Add(time.Duration(i) * time.Second),
			Severity: telemetrylog.ERROR,
			Body:     "qiuyeuq%dqw%vkjd%%qi+-s$`dwew`jd%ssdqwsd\"\\'''\\'\n" + strconv.Itoa(i),
		}
		testCollector.CollectLog("testNamespace", logEntry)
		testCollector.CollectLog("testNamespace2", logEntry)
	}
	testCollector.Flush()

	fetchedLogs, err := testCollector.FetchAndDrop(1000000)
	assert.ErrorIs(suite.T(), io.EOF, err)

	// 2 namespaces
	assert.Len(suite.T(), fetchedLogs, 2)
	assert.Len(suite.T(), fetchedLogs["testNamespace"], 100)
	assert.Len(suite.T(), fetchedLogs["testNamespace2"], 100)

	for i := 0; i < 100; i++ {
		logEntry := telemetrylog.Entry{
			Time:     startTimeStamp.Add(time.Duration(i) * time.Second),
			Severity: telemetrylog.ERROR,
			Body:     "qiuyeuq%dqw%vkjd%%qi+-s$`dwew`jd%ssdqwsd\"\\'''\\'\n" + strconv.Itoa(i),
		}

		// the logs are oldest first
		assert.Equal(suite.T(), logEntry, fetchedLogs["testNamespace"][100-i-1])
		assert.Equal(suite.T(), logEntry, fetchedLogs["testNamespace2"][100-i-1])
	}
}

func (suite *rollingLogCollectorTestSuite) TestFetchAndDropPartial() {
	// maxRolls = 1  each file 1MB
	dynamicconfiguration.MaxRolls = func(string) int { return 10 }
	dynamicconfiguration.MaxRollSize = func(string) int64 { return 1024 * 1024 }

	defer func() {
		dynamicconfiguration.MaxRolls = dynamicconfiguration.GetMaxRolls
		dynamicconfiguration.MaxRollSize = dynamicconfiguration.GetMaxRollSize
	}()
	testCollector := NewRollingLogCollector(context.NewMockDefault(), "logs")
	defer testCollector.Close()

	startTimeStamp := time.Now().UTC()

	for i := 0; i < 100; i++ {
		logEntry := telemetrylog.Entry{
			Time:     startTimeStamp.Add(time.Duration(i) * time.Second),
			Severity: telemetrylog.ERROR,
			Body:     "qiuyeuq%dqw%vkjd%%qi+-s$`dwew`jd%ssdqwsd\"\\'''\\'\n" + strconv.Itoa(i),
		}
		testCollector.CollectLog("testNamespace", logEntry)
		testCollector.CollectLog("testNamespace2", logEntry)
	}
	testCollector.Flush()

	fetchedLogs, err := testCollector.FetchAndDrop(50)
	assert.NoError(suite.T(), err) // has more logs

	// 1 namespace
	assert.Len(suite.T(), fetchedLogs, 1)
	assert.Len(suite.T(), fetchedLogs["testNamespace"], 50)

	// the logs are oldest first
	for i := 0; i < 50; i++ {
		logEntry := telemetrylog.Entry{
			Time:     startTimeStamp.Add(time.Duration(i+50) * time.Second),
			Severity: telemetrylog.ERROR,
			Body:     "qiuyeuq%dqw%vkjd%%qi+-s$`dwew`jd%ssdqwsd\"\\'''\\'\n" + strconv.Itoa(i+50),
		}

		assert.Equal(suite.T(), logEntry, fetchedLogs["testNamespace"][50-i-1])
	}
}

type rollingCollectorTester struct {
	testLogEntry telemetrylog.Entry
	testCases    []*namespacedCollectorTestCase
	t            *testing.T
}

func (suite *rollingLogCollectorTestSuite) TestRollingLogCollector() {
	suite.T().Logf("Starting rolling file writer tests")

	tester := &rollingCollectorTester{suite.testLogEntry, rollingfileWriterTests, suite.T()}

	tester.test()
}

func (tester *rollingCollectorTester) test() {
	cleanupRollingCollectorTest()

	for i, tc := range tester.testCases {
		tester.testCase(tc, i)
	}
}

func cleanupRollingCollectorTest() {
	testingDir := filepath.Clean(testingDir)
	fileutil.DeleteDirectory(testingDir)
}

func (tester *rollingCollectorTester) listAllFilesInNamespaceDir(namespace string) ([]string, error) {
	var p []string

	visit := func(path string, f os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !f.IsDir() {
			abs, err := filepath.Abs(path)
			if err != nil {
				return fmt.Errorf("filepath.Abs failed for %s", path)
			}

			p = append(p, abs)
		}

		return nil
	}

	err := filepath.Walk(filepath.Join(getBaseLogStoreDir(), namespace), visit)
	if err != nil {
		return nil, err
	}

	return p, nil
}

func (tester *rollingCollectorTester) testCase(testCase *namespacedCollectorTestCase, testNum int) {
	defer cleanupRollingCollectorTest()
	dynamicconfiguration.MaxRolls = testCase.getMaxRolls
	dynamicconfiguration.MaxRollSize = testCase.getMaxRollSize

	defer func() {
		dynamicconfiguration.MaxRolls = dynamicconfiguration.GetMaxRolls
		dynamicconfiguration.MaxRollSize = dynamicconfiguration.GetMaxRollSize
	}()
	tester.t.Logf("Start test  [%v]\n", testNum)

	rlc := NewRollingLogCollector(context.NewMockDefault(), "testlogs")
	defer rlc.Close()

	for namespace, namespaceTestConfig := range testCase.namespaces {
		tester.t.Logf("Start namespace test [%v] [%v]\n", testNum, namespace)

		// create pre-existing files. The collector should not delete these files
		for _, filePath := range namespaceTestConfig.files {
			filePath = filepath.Join(getBaseLogStoreDir(), namespace, filePath)

			dir, _ := filepath.Split(filePath)

			var err error

			if dir != "" {
				err = fileutil.MakeDirs(dir)
				if err != nil {
					tester.t.Error(err)
					return
				}
			}

			fi, err := os.Create(filePath)
			if err != nil {
				tester.t.Error(err)
				return
			}

			err = fi.Close()
			if err != nil {
				tester.t.Error(err)
				return
			}
		}

		for i := 0; i < namespaceTestConfig.writeCount; i++ {
			err := rlc.CollectLog(namespace, tester.testLogEntry)

			if err != nil {
				tester.t.Error(err)
				return
			}
		}

		// Flush for testing
		rlc.Flush()

		files, err := tester.listAllFilesInNamespaceDir(namespace)
		if err != nil {
			tester.t.Error(err)
			return
		}

		tester.t.Logf("Found files in namespace dir '%v' : %v", namespace, files)

		tester.checkRequiredFilesExist(namespace, namespaceTestConfig, files)
		tester.checkJustRequiredFilesExist(namespace, namespaceTestConfig, files)
	}

	// assert that only expected folders are present
	tester.verifyNamespaceDirectories(testCase)
}

// verifyNamespaceDirectories verifies that the directories for
// given namespaces are present and only they are present
func (tester *rollingCollectorTester) verifyNamespaceDirectories(testCase *namespacedCollectorTestCase) {
	files, err := os.ReadDir(getBaseLogStoreDir())
	if err != nil {
		tester.t.Error(err)
		return
	}

	dirs := make([]string, 0)
	for _, file := range files {
		if file.IsDir() {
			dirs = append(dirs, file.Name())
		}
	}

	sort.Strings(dirs)
	namespaces := make([]string, 0, len(testCase.namespaces))
	for k := range testCase.namespaces {
		namespaces = append(namespaces, k)
	}
	sort.Strings(namespaces)

	assert.Equal(tester.t, namespaces, dirs)
}

func (tester *rollingCollectorTester) checkRequiredFilesExist(namespace string, testConfig *collectorTestConfig, files []string) {
	var found bool
	for _, expected := range testConfig.resFiles {
		expected = filepath.Join(getBaseLogStoreDir(), namespace, expected)

		found = false
		exAbs, err := filepath.Abs(expected)
		if err != nil {
			tester.t.Errorf("filepath.Abs failed for %s", expected)
			continue
		}

		for _, f := range files {
			if af, e := filepath.Abs(f); e == nil {
				if exAbs == af {
					found = true
					break
				}
			} else {
				tester.t.Errorf("filepath.Abs failed for %s", f)
			}
		}

		if !found {
			tester.t.Errorf("expected file: %s doesn't exist. Got %v\n", exAbs, files)
		}
	}
}

func (tester *rollingCollectorTester) checkJustRequiredFilesExist(namespace string, testConfig *collectorTestConfig, files []string) {
	for _, f := range files {
		found := false
		for _, expected := range testConfig.resFiles {
			expected = filepath.Join(getBaseLogStoreDir(), namespace, expected)

			exAbs, err := filepath.Abs(expected)
			if err != nil {
				tester.t.Errorf("filepath.Abs failed for %s", expected)
			} else if exAbs == f {
				found = true
				break
			}
		}

		if !found {
			tester.t.Errorf("unexpected file: %v", f)
		}
	}
}

// test config for a specific namespace
type collectorTestConfig struct {
	// pre-existing files
	files []string

	// number of log entries to write to this namespace
	writeCount int

	// expected files
	resFiles []string
}

// test case config for rolling log collector
type namespacedCollectorTestCase struct {
	namespaces     map[string]*collectorTestConfig
	getMaxRolls    func(string) int
	getMaxRollSize func(string) int64
}

func createCollectorTestConfig(
	files []string,
	writeCount int,
	resFiles []string) *collectorTestConfig {
	return &collectorTestConfig{files, writeCount, resFiles}
}

func createRollingSizeFileWriterTestCase(
	maxRolls int, maxRollSize int64, namespaces map[string]*collectorTestConfig) *namespacedCollectorTestCase {
	return &namespacedCollectorTestCase{namespaces, func(string) int { return maxRolls }, func(string) int64 { return maxRollSize }}
}

var rollingfileWriterTests = []*namespacedCollectorTestCase{

	createRollingSizeFileWriterTestCase(
		10, 600, map[string]*collectorTestConfig{"namespace1": createCollectorTestConfig([]string{"randomfile.testlogs"}, 0, []string{"randomfile.testlogs"})}),

	createRollingSizeFileWriterTestCase(
		5, 500, map[string]*collectorTestConfig{"namespace1": createCollectorTestConfig([]string{"randomfile"}, 1, []string{"randomfile", "testlogs"})}),

	createRollingSizeFileWriterTestCase(
		5, 500, map[string]*collectorTestConfig{"namespace1": createCollectorTestConfig([]string{"randomfile.testlogs"}, 1, []string{"randomfile.testlogs", "testlogs"})}),

	// Each line is of 123 chars. Each file has ceil(500/123) = 5 log entries
	// 100 entries. So 20 files total including last file of which last 5 are kept in history
	createRollingSizeFileWriterTestCase(
		5, 500, map[string]*collectorTestConfig{"namespace1": createCollectorTestConfig([]string{"randomfile.testlogs"}, 100, []string{"randomfile.testlogs", "testlogs", "testlogs.15", "testlogs.16", "testlogs.17", "testlogs.18", "testlogs.19"})}),

	createRollingSizeFileWriterTestCase(
		5, 500, map[string]*collectorTestConfig{
			// Each line is of 123 chars. Each file has ceil(500/123) = 5 log entries
			// 32 entries. So 7 files total including last file out of which last 5 are kept in history
			"namespace1": createCollectorTestConfig([]string{"randomfile.testlogs"}, 32, []string{"randomfile.testlogs", "testlogs", "testlogs.2", "testlogs.3", "testlogs.4", "testlogs.5", "testlogs.6"}),

			// Each line is of 123 chars. Each file has ceil(500/123) = 5 log entries
			// 76 entries. So 16 files total including last file out of which last 5 are kept in history
			"namespace2": createCollectorTestConfig([]string{"randomfile.testlogs"}, 76, []string{"randomfile.testlogs", "testlogs", "testlogs.11", "testlogs.12", "testlogs.13", "testlogs.14", "testlogs.15"})}),

	// Each line is of 123 chars. Each file has ceil(500/123) = 5 log entries
	// 100 entries. So 20 files total including last file of which last 5 are kept in history
	// Plus, testlogs.5 is considered a part of roll history  so the counting starts from there
	createRollingSizeFileWriterTestCase(
		5, 500, map[string]*collectorTestConfig{"namespace1": createCollectorTestConfig([]string{"testlogs.5"}, 100, []string{"testlogs", "testlogs.20", "testlogs.21", "testlogs.22", "testlogs.23", "testlogs.24"})}),
}
