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
package metric

import (
	"time"
)

type Kind string

const (
	Sum   Kind = "sum"
	Gauge Kind = "gauge"
)

type Unit string

const (
	// Time-based units
	UnitSeconds      Unit = "s"
	UnitMicroseconds Unit = "us"
	UnitMilliseconds Unit = "ms"

	// Size-based units
	UnitBytes     Unit = "By"
	UnitKilobytes Unit = "KiBy"
	UnitMegabytes Unit = "MiBy"
	UnitGigabytes Unit = "GiBy"
	UnitTerabytes Unit = "TiBy"

	// Bit-based units
	UnitBits     Unit = "bit"
	UnitKilobits Unit = "Kibit"
	UnitMegabits Unit = "Mibit"
	UnitGigabits Unit = "Gibit"
	UnitTerabits Unit = "Tibit"

	// Rate units
	UnitBytesPerSecond     Unit = "By/s"
	UnitKilobytesPerSecond Unit = "KiBy/s"
	UnitMegabytesPerSecond Unit = "MiBy/s"
	UnitGigabytesPerSecond Unit = "GiBy/s"
	UnitTerabytesPerSecond Unit = "TiBy/s"

	UnitBitsPerSecond     Unit = "bit/s"
	UnitKilobitsPerSecond Unit = "Kibit/s"
	UnitMegabitsPerSecond Unit = "Mibit/s"
	UnitGigabitsPerSecond Unit = "Gibit/s"
	UnitTerabitsPerSecond Unit = "Tibit/s"

	// Other units
	UnitPercent     Unit = "%"
	UnitCount       Unit = "1"
	UnitCountPerSec Unit = "1/s"
	UnitNone        Unit = ""
)

func (u Unit) String() string {
	return string(u)
}

type NamespaceMetrics[N int64 | float64] map[string][]Metric[N]

type Metric[N int64 | float64] struct {
	// Name is the name of the Instrument that created this data.
	Name       string
	Unit       Unit
	Kind       Kind
	DataPoints []DataPoint[N]
}

// DataPoint is a single data point in a timeseries.
type DataPoint[N int64 | float64] struct {
	// StartTime is when the timeseries was started. (optional)
	StartTime time.Time `json:",omitempty"`
	// Time is the time when the timeseries was recorded. (optional)
	EndTime time.Time `json:",omitempty"`
	// Value is the value of this data point.
	Value N
}

type Int64Counter Counter[int64]

type Counter[N int64 | float64] interface {
	// Add increments the counter by the specified amount
	Add(incr N) error
}

// Meter interface provides different "instruments" like counter, gauge, histogram etc.
type Meter interface {
	Int64Counter(name string, unit Unit) Int64Counter
}
