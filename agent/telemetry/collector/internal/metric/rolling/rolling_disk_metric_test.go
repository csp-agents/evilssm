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
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/mocks/context"
	dynamicconfiguration "github.com/aws/amazon-ssm-agent/agent/telemetry/dynamic_configuration"
	"github.com/aws/amazon-ssm-agent/common/telemetry/metric"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

const (
	testingDir = "./testingvar"

	// all tests rely on this
	testMetricJsonSize = 163
)

type rollingDiskMetricCollectorTestSuite struct {
	suite.Suite

	backupGetBaseMetricStoreDir func() string

	testMetric metric.Metric[float64]
}

// TestChannelSuite executes test suite
func TestRollingDiskMetricCollectorSuite(t *testing.T) {
	suite.Run(t, new(rollingDiskMetricCollectorTestSuite))
}

// SetupTest makes sure that all the components referenced in the
// test case are initialized before each test
func (suite *rollingDiskMetricCollectorTestSuite) SetupTest() {
	err := fileutil.MakeDirs(testingDir)
	assert.NoError(suite.T(), err)

	// we will repeatedly write the same metric
	suite.testMetric = metric.Metric[float64]{
		Name: "test_metric",
		Unit: "ms",
		Kind: metric.Sum,
		DataPoints: []metric.DataPoint[float64]{{
			StartTime: time.Date(2025, time.January, 5, 13, 34, 23, 5672, time.UTC),
			EndTime:   time.Date(2025, time.January, 5, 13, 34, 23, 5672, time.UTC),
			Value:     10,
		}},
	}

	jsonEntry, _ := json.Marshal(suite.testMetric)

	require.Equal(suite.T(), testMetricJsonSize, len(jsonEntry))

	// mock the function which returns the base metrics directory
	mockGetBaseMetricsStoreDir := func() string {
		return filepath.Join(testingDir, "metrics")
	}
	suite.backupGetBaseMetricStoreDir = getBaseMetricsStoreDir
	getBaseMetricsStoreDir = mockGetBaseMetricsStoreDir
}

func (suite *rollingDiskMetricCollectorTestSuite) TestCorrectDataIsWritten() {
	// maxRolls = 1  each file 1MB
	dynamicconfiguration.MaxRolls = func(string) int { return 10 }
	dynamicconfiguration.MaxRollSize = func(string) int64 { return 1024 * 1024 }

	defer func() {
		dynamicconfiguration.MaxRolls = dynamicconfiguration.GetMaxRolls
		dynamicconfiguration.MaxRollSize = dynamicconfiguration.GetMaxRollSize
	}()
	testCollector := NewRollingDiskMetricCollector(context.NewMockDefault(), "metrics")
	defer testCollector.Close()

	_, err := testCollector.getMetricCollector("testNamespace")
	require.NoError(suite.T(), err)

	namespaceDir := filepath.Join(testingDir, "metrics", "testNamespace")

	for i := 0; i < 100; i++ {
		testCollector.CollectMetric("testNamespace", suite.testMetric)
	}
	testCollector.Flush()

	// assert that only 1 files was written
	files, err := os.ReadDir(namespaceDir)
	require.NoError(suite.T(), err)
	require.Len(suite.T(), files, 1)

	// read the written data back
	file, err := os.Open(filepath.Join(namespaceDir, "metrics"))
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

		var writtenMetric metric.Metric[float64]
		err = json.Unmarshal([]byte(line), &writtenMetric)
		assert.NoError(suite.T(), err)

		// assert that the written metric is the same as the one we wrote
		require.Equal(suite.T(), suite.testMetric, writtenMetric)

		countEntries++
	}

	// Check for errors during the scan
	assert.NoError(suite.T(), scanner.Err())

	assert.Equal(suite.T(), 100, countEntries)
}

func (suite *rollingDiskMetricCollectorTestSuite) TearDownTest() {
	getBaseMetricsStoreDir = suite.backupGetBaseMetricStoreDir

	testingDir := filepath.Clean(testingDir)
	fileutil.DeleteDirectory(testingDir)
}

func (suite *rollingDiskMetricCollectorTestSuite) TestFetchAndDrop() {
	// maxRolls = 1  each file 1MB
	dynamicconfiguration.MaxRolls = func(string) int { return 10 }
	dynamicconfiguration.MaxRollSize = func(string) int64 { return 1024 * 1024 }

	defer func() {
		dynamicconfiguration.MaxRolls = dynamicconfiguration.GetMaxRolls
		dynamicconfiguration.MaxRollSize = dynamicconfiguration.GetMaxRollSize
	}()
	testCollector := NewRollingDiskMetricCollector(context.NewMockDefault(), "metrics")
	defer testCollector.Close()

	startTimeStamp := time.Now().UTC()

	for i := 0; i < 100; i++ {
		metric := metric.Metric[float64]{
			Name: "test_metric",
			Unit: "ms",
			Kind: metric.Sum,
			DataPoints: []metric.DataPoint[float64]{{
				StartTime: startTimeStamp.Add(time.Duration(i) * time.Second),
				EndTime:   startTimeStamp.Add(time.Duration(i) * time.Second),
				Value:     float64(i),
			}},
		}
		testCollector.CollectMetric("testNamespace", metric)
		testCollector.CollectMetric("testNamespace2", metric)
	}
	testCollector.Flush()

	fetchedMetrics, err := testCollector.FetchAndDrop(1000000)
	assert.ErrorIs(suite.T(), io.EOF, err)

	// 2 namespaces
	assert.Len(suite.T(), fetchedMetrics, 2)
	assert.Len(suite.T(), fetchedMetrics["testNamespace"], 100)
	assert.Len(suite.T(), fetchedMetrics["testNamespace2"], 100)

	for i := 0; i < 100; i++ {
		metric := metric.Metric[float64]{
			Name: "test_metric",
			Unit: "ms",
			Kind: metric.Sum,
			DataPoints: []metric.DataPoint[float64]{{
				StartTime: startTimeStamp.Add(time.Duration(i) * time.Second),
				EndTime:   startTimeStamp.Add(time.Duration(i) * time.Second),
				Value:     float64(i),
			}},
		}
		// the logs are oldest first
		assert.Equal(suite.T(), metric, fetchedMetrics["testNamespace"][100-i-1])
		assert.Equal(suite.T(), metric, fetchedMetrics["testNamespace2"][100-i-1])
	}
}

func (suite *rollingDiskMetricCollectorTestSuite) TestFetchAndDropPartial() {
	// maxRolls = 1  each file 1MB
	dynamicconfiguration.MaxRolls = func(string) int { return 10 }
	dynamicconfiguration.MaxRollSize = func(string) int64 { return 1024 * 1024 }

	defer func() {
		dynamicconfiguration.MaxRolls = dynamicconfiguration.GetMaxRolls
		dynamicconfiguration.MaxRollSize = dynamicconfiguration.GetMaxRollSize
	}()
	testCollector := NewRollingDiskMetricCollector(context.NewMockDefault(), "metrics")
	defer testCollector.Close()

	startTimeStamp := time.Now().UTC()

	for i := 0; i < 100; i++ {
		metric := metric.Metric[float64]{
			Name: "test_metric",
			Unit: "ms",
			Kind: metric.Sum,
			DataPoints: []metric.DataPoint[float64]{{
				StartTime: startTimeStamp.Add(time.Duration(i) * time.Second),
				EndTime:   startTimeStamp.Add(time.Duration(i) * time.Second),
				Value:     float64(i),
			}},
		}
		testCollector.CollectMetric("testNamespace", metric)
		testCollector.CollectMetric("testNamespace2", metric)
	}
	testCollector.Flush()

	fetchedMetrics, err := testCollector.FetchAndDrop(50)
	assert.NoError(suite.T(), err) // has more metrics

	// 1 namespace
	assert.Len(suite.T(), fetchedMetrics, 1)
	assert.Len(suite.T(), fetchedMetrics["testNamespace"], 50)

	// the metrics are oldest first
	for i := 0; i < 50; i++ {
		metric := metric.Metric[float64]{
			Name: "test_metric",
			Unit: "ms",
			Kind: metric.Sum,
			DataPoints: []metric.DataPoint[float64]{{
				StartTime: startTimeStamp.Add(time.Duration(i+50) * time.Second),
				EndTime:   startTimeStamp.Add(time.Duration(i+50) * time.Second),
				Value:     float64(i + 50),
			}},
		}
		// the logs are oldest first
		assert.Equal(suite.T(), metric, fetchedMetrics["testNamespace"][50-i-1])
	}
}

type rollingDiskMetricCollectorTester struct {
	testMetric metric.Metric[float64]
	testCases  []*namespacedDiskMetricCollectorTestCase
	t          *testing.T
}

func (suite *rollingDiskMetricCollectorTestSuite) TestRollingDiskMetricCollector() {
	suite.T().Logf("Starting rolling file writer tests")

	tester := &rollingDiskMetricCollectorTester{suite.testMetric, rollingMetricFileWriterTests, suite.T()}

	tester.test()
}

func (tester *rollingDiskMetricCollectorTester) test() {
	cleanupRollingCollectorTest()

	for i, tc := range tester.testCases {
		tester.testCase(tc, i)
	}
}

func cleanupRollingCollectorTest() {
	testingDir := filepath.Clean(testingDir)
	fileutil.DeleteDirectory(testingDir)
}

func (tester *rollingDiskMetricCollectorTester) listAllFilesInNamespaceDir(namespace string) ([]string, error) {
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

	err := filepath.Walk(filepath.Join(getBaseMetricsStoreDir(), namespace), visit)
	if err != nil {
		return nil, err
	}

	return p, nil
}

func (tester *rollingDiskMetricCollectorTester) testCase(testCase *namespacedDiskMetricCollectorTestCase, testNum int) {
	defer cleanupRollingCollectorTest()

	dynamicconfiguration.MaxRolls = testCase.getMaxRolls
	dynamicconfiguration.MaxRollSize = testCase.getMaxRollSize

	defer func() {
		dynamicconfiguration.MaxRolls = dynamicconfiguration.GetMaxRolls
		dynamicconfiguration.MaxRollSize = dynamicconfiguration.GetMaxRollSize
	}()
	tester.t.Logf("Start test  [%v]\n", testNum)

	rlc := NewRollingDiskMetricCollector(context.NewMockDefault(), "testmetrics")
	defer rlc.Close()

	for namespace, namespaceTestConfig := range testCase.namespaces {
		tester.t.Logf("Start namespace test [%v] [%v]\n", testNum, namespace)

		// create pre-existing files. The collector should not delete these files
		for _, filePath := range namespaceTestConfig.files {
			filePath = filepath.Join(getBaseMetricsStoreDir(), namespace, filePath)

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
			err := rlc.CollectMetric(namespace, tester.testMetric)

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
func (tester *rollingDiskMetricCollectorTester) verifyNamespaceDirectories(testCase *namespacedDiskMetricCollectorTestCase) {
	files, err := os.ReadDir(getBaseMetricsStoreDir())
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

func (tester *rollingDiskMetricCollectorTester) checkRequiredFilesExist(namespace string, testConfig *diskMetricCollectorTestConfig, files []string) {
	var found bool
	for _, expected := range testConfig.resFiles {
		expected = filepath.Join(getBaseMetricsStoreDir(), namespace, expected)

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

func (tester *rollingDiskMetricCollectorTester) checkJustRequiredFilesExist(namespace string, testConfig *diskMetricCollectorTestConfig, files []string) {
	for _, f := range files {
		found := false
		for _, expected := range testConfig.resFiles {
			expected = filepath.Join(getBaseMetricsStoreDir(), namespace, expected)

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
type diskMetricCollectorTestConfig struct {
	// pre-existing files
	files []string

	// number of metrics to write to this namespace
	writeCount int

	// expected files
	resFiles []string
}

// test case config for rolling collector
type namespacedDiskMetricCollectorTestCase struct {
	getMaxRolls    func(string) int
	getMaxRollSize func(string) int64
	namespaces     map[string]*diskMetricCollectorTestConfig
}

func createMetricCollectorTestConfig(
	files []string,
	writeCount int,
	resFiles []string) *diskMetricCollectorTestConfig {
	return &diskMetricCollectorTestConfig{files, writeCount, resFiles}
}

func createMetricRollingSizeFileWriterTestCase(
	maxRolls int,
	maxRollSize int64,
	namespaces map[string]*diskMetricCollectorTestConfig) *namespacedDiskMetricCollectorTestCase {
	var getMaxRolls = func(string) int { return maxRolls }
	var getMaxRollSize = func(string) int64 { return maxRollSize }
	return &namespacedDiskMetricCollectorTestCase{getMaxRolls, getMaxRollSize, namespaces}
}

var rollingMetricFileWriterTests = []*namespacedDiskMetricCollectorTestCase{
	createMetricRollingSizeFileWriterTestCase(10, 600,
		map[string]*diskMetricCollectorTestConfig{"namespace1": createMetricCollectorTestConfig([]string{"randomfile.testmetrics"}, 0, []string{"randomfile.testmetrics"})}),

	createMetricRollingSizeFileWriterTestCase(5, 500,
		map[string]*diskMetricCollectorTestConfig{"namespace1": createMetricCollectorTestConfig([]string{"randomfile"}, 1, []string{"randomfile", "testmetrics"})}),

	createMetricRollingSizeFileWriterTestCase(5, 500,
		map[string]*diskMetricCollectorTestConfig{"namespace1": createMetricCollectorTestConfig([]string{"randomfile.testmetrics"}, 1, []string{"randomfile.testmetrics", "testmetrics"})}),

	// Each line is of 164 chars. Each file has ceil(500/164) = 4 entries
	// 100 entries. So 25 files total including last file of which last 5 are kept in history
	createMetricRollingSizeFileWriterTestCase(5, 500,
		map[string]*diskMetricCollectorTestConfig{"namespace1": createMetricCollectorTestConfig([]string{"randomfile.testmetrics"}, 100, []string{"randomfile.testmetrics", "testmetrics", "testmetrics.20", "testmetrics.21", "testmetrics.22", "testmetrics.23", "testmetrics.24"})}),

	createMetricRollingSizeFileWriterTestCase(5, 500,
		map[string]*diskMetricCollectorTestConfig{
			// Each line is of 164 chars. Each file has ceil(500/164) = 4 entries
			// 32 entries. So 8 files total including last file out of which last 5 are kept in history
			"namespace1": createMetricCollectorTestConfig([]string{"randomfile.testmetrics"}, 32, []string{"randomfile.testmetrics", "testmetrics", "testmetrics.3", "testmetrics.4", "testmetrics.5", "testmetrics.6", "testmetrics.7"}),

			// Each line is of 164 chars. Each file has ceil(500/164) = 4 entries
			// 76 entries. So 19 files total including last file out of which last 5 are kept in history
			"namespace2": createMetricCollectorTestConfig([]string{"randomfile.testmetrics"}, 76, []string{"randomfile.testmetrics", "testmetrics", "testmetrics.14", "testmetrics.15", "testmetrics.16", "testmetrics.17", "testmetrics.18"})}),

	// Each line is of 164 chars. Each file has ceil(500/164) = 4 entries
	// 100 entries. So 25 files total including last file of which last 5 are kept in history
	// Plus, testmetrics.5 is considered a part of roll history  so the counting starts from there
	createMetricRollingSizeFileWriterTestCase(5, 500,
		map[string]*diskMetricCollectorTestConfig{"namespace1": createMetricCollectorTestConfig([]string{"testmetrics.5"}, 100, []string{"testmetrics", "testmetrics.25", "testmetrics.26", "testmetrics.27", "testmetrics.28", "testmetrics.29"})}),
}
