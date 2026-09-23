// Copyright 2022 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

//go:build darwin
// +build darwin

package darwinec2detector

import "github.com/aws/amazon-ssm-agent/agent/log"

const Name = "Darwin"

type darwinEc2Detector struct{}

func (*darwinEc2Detector) IsEc2(_ log.T) (bool, []string) {
	// Darwin is only supported in EC2, assume we are on EC2
	return true, nil
}

func (*darwinEc2Detector) GetName() string {
	return Name
}

func New() *darwinEc2Detector {
	return &darwinEc2Detector{}
}
