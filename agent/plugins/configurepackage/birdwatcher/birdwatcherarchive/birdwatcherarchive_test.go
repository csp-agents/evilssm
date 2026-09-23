// Copyright 2018 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

// Package birdwatcherarchive contains the struct that is called when the package information is stored in birdwatcher
package birdwatcherarchive

import (
	"fmt"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/birdwatcher"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/birdwatcher/archive"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/birdwatcher/facade"
	mockpackage "github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/packageservice/mock"
	"github.com/stretchr/testify/assert"
)

func TestGetResourceVersion(t *testing.T) {

	data := []struct {
		name    string
		version string
	}{
		{
			"PVDriver",
			"latest",
		},
		{
			"PVDriver",
			"",
		},
		{
			"PVDriver",
			"1.2.3.4",
		},
	}

	for _, testdata := range data {
		t.Run(testdata.name, func(t *testing.T) {

			mockBWFacade := facade.FacadeStub{}

			context := make(map[string]string)
			context["packageName"] = testdata.name
			context["packageVersion"] = testdata.version
			context["manifest"] = "manifest"
			bwArchive := New(&mockBWFacade, context)

			name, version := bwArchive.GetResourceVersion(testdata.name, testdata.version)
			assert.Equal(t, name, testdata.name)
			if testdata.version == "" {
				assert.Equal(t, version, "latest")
			} else {
				assert.Equal(t, version, testdata.version)
			}

		})
	}
}

func TestArchiveName(t *testing.T) {
	facadeSession := facade.FacadeStub{}
	context := make(map[string]string)
	context["packageName"] = "name"
	context["packageVersion"] = "version"
	context["manifest"] = "manifest"
	testArchive := New(&facadeSession, context)

	assert.Equal(t, archive.PackageArchiveBirdwatcher, testArchive.Name())

}

func TestSetResource(t *testing.T) {
	// Setup test data
	packageName := "TestPackage"
	version := "1.0.0"
	manifestInput := &birdwatcher.Manifest{
		PackageArn: "arn:aws:ssm:us-east-1:123456789012:package/test-package",
		Version:    "1.0.0",
	}
	expectedPackageArn := "arn:aws:ssm:us-east-1:123456789012:package/test-package"
	expectedManifestVersion := "1.0.0"

	// Setup
	mockBWFacade := facade.FacadeStub{}
	context := make(map[string]string)
	context["packageName"] = packageName
	context["packageVersion"] = version
	context["manifest"] = "manifest"

	// Create PackageArchive instance
	bwArchive := New(&mockBWFacade, context)

	// Perform SetResource
	bwArchive.SetResource(packageName, version, manifestInput)

	// Verify the resource was set correctly
	key := archive.FormKey(packageName, version)
	localManifest, exists := bwArchive.(*PackageArchive).localManifests[key]

	assert.True(t, exists, "Local manifest should exist for the key")
	assert.Equal(t, expectedPackageArn, localManifest.packageArn, "Package ARN should match")
	assert.Equal(t, expectedManifestVersion, localManifest.manifestVersion, "Manifest version should match")
}

func TestSetResourceMultipleCalls(t *testing.T) {
	// Test setting resources multiple times to ensure proper handling
	mockBWFacade := facade.FacadeStub{}
	context := make(map[string]string)
	context["packageName"] = "MultiPackage"
	context["packageVersion"] = "1.0.0"
	context["manifest"] = "manifest"

	bwArchive := New(&mockBWFacade, context)

	// First resource setting
	firstManifest := &birdwatcher.Manifest{
		PackageArn: "arn:aws:ssm:us-east-1:123456789012:package/multi-1",
		Version:    "1.0.0",
	}
	bwArchive.SetResource("MultiPackage", "1.0.0", firstManifest)

	// Second resource setting
	secondManifest := &birdwatcher.Manifest{
		PackageArn: "arn:aws:ssm:us-east-1:123456789012:package/multi-2",
		Version:    "2.0.0",
	}
	bwArchive.SetResource("MultiPackage", "2.0.0", secondManifest)

	// Verify both resources are stored correctly
	key1 := archive.FormKey("MultiPackage", "1.0.0")
	key2 := archive.FormKey("MultiPackage", "2.0.0")

	localManifests := bwArchive.(*PackageArchive).localManifests

	assert.Contains(t, localManifests, key1, "First manifest key should exist")
	assert.Contains(t, localManifests, key2, "Second manifest key should exist")
	assert.Equal(t, "arn:aws:ssm:us-east-1:123456789012:package/multi-1", localManifests[key1].packageArn)
	assert.Equal(t, "arn:aws:ssm:us-east-1:123456789012:package/multi-2", localManifests[key2].packageArn)
}
func TestGetResourceArn_ExistingManifest(t *testing.T) {
	// Test data
	packageName := "TestPackage"
	version := "1.0.0"
	expectedARN := "arn:aws:ssm:us-east-1:123456789012:package/TestPackage/1.0.0"

	// Setup mock facade and context
	mockBWFacade := facade.FacadeStub{}
	context := map[string]string{
		"packageName":    packageName,
		"packageVersion": version,
		"manifest":       "test-manifest",
	}

	// Create archive with predefined manifest
	bwArchive := New(&mockBWFacade, context)

	// Set resource with a specific ARN
	bwArchive.SetResource(packageName, version, &birdwatcher.Manifest{
		PackageArn: expectedARN,
		Version:    version,
	})

	// Verify ARN retrieval
	retrievedARN := bwArchive.GetResourceArn(packageName, version)
	assert.Equal(t, expectedARN, retrievedARN)
}

func TestGetResourceArn_NonExistentManifest(t *testing.T) {
	// Test data
	packageName := "SomePackage"
	version := ""

	// Setup mock facade
	mockBWFacade := facade.FacadeStub{}
	context := make(map[string]string)

	// Create archive
	bwArchive := New(&mockBWFacade, context)

	// Verify empty ARN for non-existent manifest
	retrievedARN := bwArchive.GetResourceArn(packageName, version)
	assert.Empty(t, retrievedARN)
}

func TestGetFileDownloadLocation(t *testing.T) {
	testCases := []struct {
		name           string
		file           *archive.File
		packageName    string
		version        string
		expectedResult string
		expectedError  bool
	}{
		{
			name:           "Nil file input",
			file:           nil,
			packageName:    "TestPackage",
			version:        "1.0.0",
			expectedResult: "",
			expectedError:  true,
		},
		{
			name: "Empty download location",
			file: &archive.File{
				Info: birdwatcher.FileInfo{
					DownloadLocation: "",
				},
			},
			packageName:    "TestPackage",
			version:        "1.0.0",
			expectedResult: "",
			expectedError:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			facadeSession := facade.FacadeStub{}
			context := make(map[string]string)
			context["packageName"] = tc.packageName
			context["packageVersion"] = tc.version
			context["manifest"] = "manifest"

			testArchive := New(&facadeSession, context)

			// Execute
			result, err := testArchive.GetFileDownloadLocation(tc.file, tc.packageName, tc.version)

			// Assert
			if tc.expectedError {
				assert.Error(t, err)
				assert.Empty(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedResult, result)
			}
		})
	}
}

func TestDeleteCachedManifest(t *testing.T) {
	testCases := []struct {
		name            string
		packageArn      string
		version         string
		mockDeleteError error
		expectError     bool
	}{
		{
			name:            "Successful Manifest Deletion",
			packageArn:      "arn:aws:ssm:us-east-1:123456789012:package/test-package",
			version:         "1.0.0",
			mockDeleteError: nil,
			expectError:     false,
		},
		{
			name:            "Manifest Deletion with Error",
			packageArn:      "arn:aws:ssm:us-east-1:123456789012:package/error-package",
			version:         "2.0.0",
			mockDeleteError: fmt.Errorf("delete manifest error"),
			expectError:     true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create mock manifest cache
			mockCache := &mockpackage.ManifestCache{}
			mockCache.On("DeleteManifest", tc.packageArn, tc.version).Return(tc.mockDeleteError)

			// Create PackageArchive with mock cache
			mockFacade := facade.FacadeStub{}
			context := make(map[string]string)
			bwArchive := New(&mockFacade, context)
			bwArchive.SetManifestCache(mockCache)

			// Call DeleteCachedManifest
			err := bwArchive.DeleteCachedManifest(tc.packageArn, tc.version)

			// Verify expectations
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Verify mock interactions
			mockCache.AssertExpectations(t)
		})
	}
}

func TestWriteManifestToCache(t *testing.T) {
	// Create mock manifest cache
	mockCache := &mockpackage.ManifestCache{}

	// Setup test data
	packageArn := "arn:aws:ssm:us-east-1:123456789012:package/test-package"
	version := "1.0.0"
	manifest := []byte(`{"name": "test-manifest"}`)

	// Setup mock behavior
	mockCache.On("WriteManifest",
		packageArn,
		version,
		manifest).
		Return(nil)

	// Create PackageArchive with mock cache
	facadeSession := facade.FacadeStub{}
	context := make(map[string]string)
	bwArchive := New(&facadeSession, context)
	bwArchive.SetManifestCache(mockCache)

	// Execute WriteManifestToCache
	err := bwArchive.WriteManifestToCache(packageArn, version, manifest)

	// Assertions
	assert.NoError(t, err, "Did not expect an error when writing manifest to cache")

	// Verify mock expectations
	mockCache.AssertExpectations(t)
}
