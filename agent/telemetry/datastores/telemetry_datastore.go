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

import "sync"

var lastEmittedDataStore *LastEmittedDataStore
var lock sync.RWMutex

type TelemetryDataStore interface {
	Read(k string) int64
	Write(k string, v int64)
}
type LastEmittedDataStore map[string]int64

func (l LastEmittedDataStore) Read(k string) int64 {
	lock.RLock()
	defer lock.RUnlock()
	return Read(l, k, 0)
}

func Read[K comparable, V any, L ~map[K]V](l L, k K, zeroValue V) V {
	val, ok := l[k]
	if !ok {
		return zeroValue
	}
	return val
}

func (l *LastEmittedDataStore) Write(k string, v int64) {
	lock.Lock()
	defer lock.Unlock()
	Write(l, k, v)
}

func Write[K comparable, V any, L ~map[K]V](l *L, k K, v V) {
	(*l)[k] = v
}

func NewLastEmittedDataStore() *LastEmittedDataStore {
	lsd := make(LastEmittedDataStore)
	return &lsd
}

var TelemetryLastEmittedDataStore = GetLastEmittedDataStore

func GetLastEmittedDataStore() *LastEmittedDataStore {
	lock.Lock()
	defer lock.Unlock()
	if lastEmittedDataStore == nil {
		lastEmittedDataStore = NewLastEmittedDataStore()
	}
	return lastEmittedDataStore
}

func ShutdownLastEmittedDataStore() {
	lock.Lock()
	defer lock.Unlock()
	lastEmittedDataStore = nil
}
