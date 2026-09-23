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
package inmemory

import "time"

// timeSpan represents a timespan where startTime is inclusive and endTime is exclusive
type timeSpan struct {
	startTime time.Time
	endTime   time.Time
}

// contains checks if the given timestamp falls within this timeSpan
func (ts timeSpan) contains(t time.Time) bool {
	return (t.Equal(ts.startTime) || t.After(ts.startTime)) && t.Before(ts.endTime)
}
