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
package datastores

import (
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/mocks/context"

	"github.com/stretchr/testify/assert"

	"github.com/stretchr/testify/suite"
)

type telemetryDatastoreTestSuite struct {
	suite.Suite
	ctx *context.Mock
}

func (suite *telemetryDatastoreTestSuite) SetupTest() {
	suite.ctx = context.NewMockDefault()
}

// TestControlChannelExporterSuite executes test suite
func TestTelemetryDatastoreSuite(t *testing.T) {
	suite.Run(t, new(telemetryDatastoreTestSuite))
}

func (suite *telemetryDatastoreTestSuite) TestGetLastEmittedDataStoreActsAsSingleton() {
	defer ShutdownLastEmittedDataStore()
	initialLastEmittedDataStore := GetLastEmittedDataStore()
	finalLastEmittedDataStore := GetLastEmittedDataStore()
	assert.Same(suite.T(), initialLastEmittedDataStore, finalLastEmittedDataStore)
}

func (suite *telemetryDatastoreTestSuite) TestReadDefault() {
	defer ShutdownLastEmittedDataStore()
	dataStore := GetLastEmittedDataStore()
	assert.Equal(suite.T(), int64(0), dataStore.Read("testNamespace"))
}

func (suite *telemetryDatastoreTestSuite) TestWrite() {
	defer ShutdownLastEmittedDataStore()
	dataStore := GetLastEmittedDataStore()
	dataStore.Write("testNamespace", 15)
	assert.Equal(suite.T(), int64(15), dataStore.Read("testNamespace"))
}

func (suite *telemetryDatastoreTestSuite) TestLastWriteWins() {
	defer ShutdownLastEmittedDataStore()
	dataStore := GetLastEmittedDataStore()
	dataStore.Write("testNamespace", 15)
	dataStore.Write("testNamespace", 35)
	assert.Equal(suite.T(), int64(35), dataStore.Read("testNamespace"))
}
