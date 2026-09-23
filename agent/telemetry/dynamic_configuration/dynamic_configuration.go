package dynamicconfiguration

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"sync"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
)

// loaded dynamic configuration
var loadedDynamicConfiguration *NamespaceConfiguration

var lock sync.RWMutex

type NamespaceConfiguration map[string]DynamicConfiguration

type DynamicConfiguration struct {
	TelemetryDisabledTill int64   `json:"telemetryDisabledTillEpoch"`
	PercentageLimit       float64 `json:"percentageLimit"`
	MaxRolls              int     `json:"maxRolls"`
	MaxRollSize           int64   `json:"maxRollSize"`
	ExportPeriod          int     `json:"exportPeriodMinutes"`
}

func NewTelemetryDynamicConfiguration(log log.T, useWatcher bool, configFilePath string) NamespaceConfiguration {
	if !isLoaded() {
		dynamicConfiguration := initDynamicConfiguration(log, useWatcher, configFilePath)
		err := cache(dynamicConfiguration)
		if err != nil {
			log.Errorf("Error caching dynamic configuration: %v", err)
		}
	}
	return GetCachedDynamicConfiguration()
}

// check if dynamic configuration has been loaded into in-memory cache
func isLoaded() bool {
	lock.RLock()
	defer lock.RUnlock()
	return loadedDynamicConfiguration != nil
}

// cache the loaded dynamicConfiguration
func cache(dynamicConfiguration NamespaceConfiguration) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic in cache function: %v", r)
		}
	}()

	lock.Lock()
	defer lock.Unlock()
	loadedDynamicConfiguration = &dynamicConfiguration
	return nil
}

func EvictCache() {
	lock.Lock()
	defer lock.Unlock()
	loadedDynamicConfiguration = nil
}

func getDefaultConfiguration() DynamicConfiguration {
	return DynamicConfiguration{
		TelemetryDisabledTill: 0,
		PercentageLimit:       1,
		MaxRolls:              10,
		MaxRollSize:           2048,
		ExportPeriod:          5,
	}
}

func getConfiguration(namespace string) DynamicConfiguration {
	if !isLoaded() {
		return populateDynamicConfigurationWithDefaults()["default"]
	}

	namespacedConfiguration, ok := GetCachedDynamicConfiguration()[namespace]
	if ok {
		return namespacedConfiguration
	}

	defaultConfiguration, ok := GetCachedDynamicConfiguration()["default"]
	if !ok {
		return populateDynamicConfigurationWithDefaults()["default"]
	}

	return defaultConfiguration
}

var MaxRolls = GetMaxRolls

func GetMaxRolls(namespace string) int {
	return getConfiguration(namespace).MaxRolls
}

var MaxRollSize = GetMaxRollSize

func GetMaxRollSize(namespace string) int64 {
	return getConfiguration(namespace).MaxRollSize
}

var ExportPeriod = GetExportPeriod

func GetExportPeriod(namespace string) int {
	return getConfiguration(namespace).ExportPeriod
}

var PercentageLimit = GetPercentageLimit

func GetPercentageLimit(namespace string) float64 {
	return getConfiguration(namespace).PercentageLimit
}

var TelemetryDisabledTill = GetTelemetryDisabledTill

func GetTelemetryDisabledTill(namespace string) int64 {
	return getConfiguration(namespace).TelemetryDisabledTill
}

// return the cached dynamic configuration
func GetCachedDynamicConfiguration() NamespaceConfiguration {
	lock.RLock()
	defer lock.RUnlock()
	return *loadedDynamicConfiguration
}

func readCurrentDynamicConfiguration(log log.T, configFilePath string) (configMap NamespaceConfiguration, err error) {
	lock.RLock()
	defer lock.RUnlock()

	configBytes, err := os.ReadFile(configFilePath)
	if err != nil {
		log.Warnf("Unable to reading dynamic configuration file:", err)
		return nil, err
	}
	// Parse the JSON data into a map
	err = json.Unmarshal(configBytes, &configMap)
	if err != nil {
		log.Errorf("Error unmarshalling dynamic configuration:", err)
		return nil, err
	}
	return
}

func populateDynamicConfigurationWithDefaults() NamespaceConfiguration {
	configMap := make(NamespaceConfiguration)
	configMap["default"] = getDefaultConfiguration()
	return configMap
}

var GetDynamicConfigFolderPath = func() string {
	return appconfig.DynamicConfigFolderPath
}

func saveDynamicConfiguration(log log.T, configMap NamespaceConfiguration, configFilePath string) (err error) {
	err = os.MkdirAll(GetDynamicConfigFolderPath(), 0700)
	if err != nil {
		log.Errorf("Failed to create directory: %v", err)
		return
	}

	configBytes, err := json.Marshal(configMap)
	if err != nil {
		log.Errorf("Error marshaling dynamic configuration map:", err)
		return
	}
	err = os.WriteFile(configFilePath, configBytes, 0600)
	if err != nil {
		log.Errorf("Error saving dynamic configuration file to disk:", err)
		return
	}
	return nil
}

func hardenDynamicConfigFolderPermissions(log log.T) {
	// Skip on windows as the permission system is different
	// And already hardened recursively at SSMDataPath level
	if runtime.GOOS == "windows" {
		return
	}
	folderPath := GetDynamicConfigFolderPath()
	info, err := os.Stat(folderPath)
	if err != nil {
		log.Errorf("Failed to stat dynamic config folder: %v", err)
		return
	}

	currentPerms := info.Mode().Perm()
	if currentPerms != 0700 {
		log.Warnf("Updating dynamic config folder permissions from %o to 0700", currentPerms)
		err = os.Chmod(folderPath, 0700)
		if err != nil {
			log.Errorf("Failed to harden dynamic config folder permissions: %v", err)
		}
	}
}

func getInitialDynamicConfiguration(log log.T, configFilePath string) NamespaceConfiguration {
	dynamicConfiguration, err := readCurrentDynamicConfiguration(log, configFilePath)
	if dynamicConfiguration == nil || err != nil {
		// Either file does not exist or corrupted config failed to be deserialized to in-memory map
		dynamicConfiguration = populateDynamicConfigurationWithDefaults()
		saveDynamicConfiguration(log, dynamicConfiguration, configFilePath)
	} else {
		hardenDynamicConfigFolderPermissions(log)
	}
	return dynamicConfiguration
}

func logDynamicConfiguration(log log.T, dynamicConfiguration NamespaceConfiguration) {
	for namespace, config := range dynamicConfiguration {
		configJSON, err := json.Marshal(config)
		if err != nil {
			log.Errorf("Failed to marshal configuration for namespace %s: %v", namespace, err)
			continue
		}
		log.Infof("Configuration for namespace %s: %s", namespace, string(configJSON))
	}
}

func replaceDynamicConfiguration(log log.T, configFilePath string) {
	log.Debug("Dynamic configuration file updated. Replacing old configuration by updating cache")

	dynamicConfiguration, err := readCurrentDynamicConfiguration(log, configFilePath)
	if err != nil {
		log.Errorf("Failed to read dynamic configuration file, skipping cache update: %v", err)
		return
	}

	logDynamicConfiguration(log, dynamicConfiguration)

	err = cache(dynamicConfiguration)
	if err != nil {
		log.Errorf("Failed to cache new dynamic configuration: %v", err)
	}
}

// initDynamicConfiguration initializes a new DynamicConfiguration object based on current configurations and starts file watcher on the configurations file
func initDynamicConfiguration(log log.T, useWatcher bool, configFilePath string) (dynamicConfiguration NamespaceConfiguration) {
	// This function is called only once throughout lifetime of process, when telemetry is initialized and cache is empty
	// Fetching from disk and/or populating cache with default values and writing to disk is mandatory
	// First we attempt to fetch from disk because any stale overridden config is better/newer than default config
	// If file does not exist on disk or if disk read failed, we populate it with default config instead of retrying as configs are not super-critical and default configs are a safe start
	defer func() {
		if msg := recover(); msg != nil {
			log.Errorf("Panic while initializing DynamicConfiguration: %v", msg)
		}
	}()

	dynamicConfiguration = getInitialDynamicConfiguration(log, configFilePath)
	log.Info("Initial telemetry configuration retrieved:")
	logDynamicConfiguration(log, dynamicConfiguration)

	if useWatcher {
		// Start the config file watcher
		startWatcher(log, configFilePath)
	}
	return
}

// startWatcher starts the file watcher on the dynamic configuration file path
func startWatcher(log log.T, filePath string) {
	fileWatcher := NewFileWatcher(log, filePath, replaceDynamicConfiguration)
	// Start the file watcher
	fileWatcher.Start()
}
