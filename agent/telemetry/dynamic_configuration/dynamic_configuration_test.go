package dynamicconfiguration

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	logmocks "github.com/aws/amazon-ssm-agent/agent/mocks/log"

	"github.com/stretchr/testify/assert"
)

const (
	testingDir         = "./testingvar"
	fakeConfigFilePath = ("./testingvar/dynamic_config.json")
)

func Refresh() {
	EvictCache()
	testingDir := filepath.Clean(testingDir)
	fileutil.DeleteDirectory(testingDir)
}

func Setup() {
	Refresh()
}

func Teardown() {
	Refresh()
	// Needed to make sure watchers left behind by previous tests do not react to stale events and pollute cache
	// TODO: Find better way to do this. Maybe add explicit stopper on watcher, to be called only during tests
	time.Sleep(5 * time.Second)
}

func TestNewTelemetryDynamicConfigurationPopulatesDefaults(t *testing.T) {
	// Act
	Setup()
	defer Teardown()

	log := logmocks.NewMockLog()

	NewTelemetryDynamicConfiguration(log, false, fakeConfigFilePath)
	dynamicConfig := GetCachedDynamicConfiguration()

	assert.Equal(t, getDefaultConfiguration().TelemetryDisabledTill, dynamicConfig["default"].TelemetryDisabledTill)
	assert.Equal(t, getDefaultConfiguration().PercentageLimit, dynamicConfig["default"].PercentageLimit)
	assert.Equal(t, getDefaultConfiguration().MaxRolls, dynamicConfig["default"].MaxRolls)
	assert.Equal(t, getDefaultConfiguration().MaxRollSize, dynamicConfig["default"].MaxRollSize)
	assert.Equal(t, getDefaultConfiguration().ExportPeriod, dynamicConfig["default"].ExportPeriod)
}

func TestNewTelemetryDynamicConfigurationReadsPreExistingConfigFile(t *testing.T) {
	// Act
	Setup()
	defer Teardown()

	log := logmocks.NewMockLog()
	os.MkdirAll(filepath.Clean(testingDir), 0700)
	os.WriteFile(
		filepath.Clean(fakeConfigFilePath),
		[]byte(`{
	    "default": {
	        "telemetryDisabledTillEpoch": 10,
	        "percentageLimit": 50,
	        "maxRolls": 15,
	        "maxRollSize": 500,
	        "exportPeriodMinutes": 15
	    }
	}`),
		0600,
	)

	NewTelemetryDynamicConfiguration(log, false, filepath.Clean(fakeConfigFilePath))
	dynamicConfig := GetCachedDynamicConfiguration()
	assert.Equal(t, int64(10), dynamicConfig["default"].TelemetryDisabledTill)
	assert.Equal(t, float64(50), dynamicConfig["default"].PercentageLimit)
	assert.Equal(t, 15, dynamicConfig["default"].MaxRolls, 15)
	assert.Equal(t, int64(500), dynamicConfig["default"].MaxRollSize)
	assert.Equal(t, 15, dynamicConfig["default"].ExportPeriod)
}

func TestNewTelemetryDynamicConfigurationWatchesSimpleConfigFileUpdates(t *testing.T) {
	// Act
	Setup()
	defer Teardown()

	log := logmocks.NewMockLog()
	os.MkdirAll(filepath.Clean(testingDir), 0700)
	os.WriteFile(filepath.Clean(fakeConfigFilePath), []byte(`{"default":{"telemetryDisabledTillEpoch":10,"percentageLimit":50,"maxRolls":15,"maxRollSize":500,"exportPeriodMinutes":15}}`), 0600)

	NewTelemetryDynamicConfiguration(log, true, filepath.Clean(fakeConfigFilePath))

	os.WriteFile(fakeConfigFilePath, []byte(`{"default":{"telemetryDisabledTillEpoch":55,"percentageLimit":12,"maxRolls":17,"maxRollSize":502,"exportPeriodMinutes":20}}`), 0600)

	time.Sleep(5 * time.Second)

	dynamicConfig := GetCachedDynamicConfiguration()
	assert.Equal(t, int64(55), dynamicConfig["default"].TelemetryDisabledTill)
	assert.Equal(t, float64(12), dynamicConfig["default"].PercentageLimit)
	assert.Equal(t, 17, dynamicConfig["default"].MaxRolls)
	assert.Equal(t, int64(502), dynamicConfig["default"].MaxRollSize)
	assert.Equal(t, 20, dynamicConfig["default"].ExportPeriod)
}

func TestNewTelemetryDynamicConfigurationWatchesConfigFileUpdatesWithNewNamespaces(t *testing.T) {
	// Act
	Setup()
	defer Teardown()

	log := logmocks.NewMockLog()
	os.MkdirAll(filepath.Clean(testingDir), 0700)
	err := os.WriteFile(filepath.Clean(fakeConfigFilePath), []byte(`{"default":{"telemetryDisabledTillEpoch":10,"percentageLimit":50,"maxRolls":15,"maxRollSize":500,"exportPeriodMinutes":15}}`), 0600)
	if err != nil {
		log.Errorf("Error while opening config file: %v", err)
	}

	NewTelemetryDynamicConfiguration(log, true, filepath.Clean(fakeConfigFilePath))

	os.WriteFile(filepath.Clean(fakeConfigFilePath), []byte(`{
     "default": {
         "telemetryDisabledTillEpoch": 55,
         "percentageLimit": 12,
         "maxRolls": 17,
         "maxRollSize": 502,
         "exportPeriodMinutes": 20
     },
     "distributor": {
         "telemetryDisabledTillEpoch": 89,
         "percentageLimit": 82,
         "maxRolls": 87,
         "maxRollSize": 902,
         "exportPeriodMinutes": 20
     }
 }`), 0600)

	time.Sleep(5 * time.Second)

	dynamicConfig := GetCachedDynamicConfiguration()

	assert.Equal(t, int64(55), dynamicConfig["default"].TelemetryDisabledTill)
	assert.Equal(t, float64(12), dynamicConfig["default"].PercentageLimit)
	assert.Equal(t, 17, dynamicConfig["default"].MaxRolls)
	assert.Equal(t, int64(502), dynamicConfig["default"].MaxRollSize)
	assert.Equal(t, 20, dynamicConfig["default"].ExportPeriod)

	assert.Equal(t, int64(89), dynamicConfig["distributor"].TelemetryDisabledTill)
	assert.Equal(t, float64(82), dynamicConfig["distributor"].PercentageLimit)
	assert.Equal(t, 87, dynamicConfig["distributor"].MaxRolls)
	assert.Equal(t, int64(902), dynamicConfig["distributor"].MaxRollSize)
	assert.Equal(t, 20, dynamicConfig["distributor"].ExportPeriod)
}

func TestMaxRollsWhenConfigInitialisedWithoutNamespace(t *testing.T) {
	// Act
	Setup()
	defer Teardown()
	log := logmocks.NewMockLog()

	os.MkdirAll(filepath.Clean(testingDir), 0700)
	err := os.WriteFile(filepath.Clean(fakeConfigFilePath), []byte(`{"default":{"telemetryDisabledTillEpoch":10,"percentageLimit":50,"maxRolls":15,"maxRollSize":500,"exportPeriodMinutes":15}}`), 0600)
	if err != nil {
		log.Errorf("Error while opening config file: %v", err)
	}

	NewTelemetryDynamicConfiguration(log, true, filepath.Clean(fakeConfigFilePath))

	assert.Equal(t, 15, MaxRolls("ssmagent"))
}

func TestMaxRollSizeWhenConfigInitialisedWithoutNamespace(t *testing.T) {
	// Act
	Setup()
	defer Teardown()
	log := logmocks.NewMockLog()

	os.MkdirAll(filepath.Clean(testingDir), 0700)
	err := os.WriteFile(filepath.Clean(fakeConfigFilePath), []byte(`{"default":{"telemetryDisabledTillEpoch":10,"percentageLimit":50,"maxRolls":15,"maxRollSize":500,"exportPeriodMinutes":15}}`), 0600)
	if err != nil {
		log.Errorf("Error while opening config file: %v", err)
	}

	NewTelemetryDynamicConfiguration(log, true, filepath.Clean(fakeConfigFilePath))

	assert.Equal(t, int64(500), MaxRollSize("ssmagent"))
}

func TestPercentageLimitWhenConfigInitialisedWithoutNamespace(t *testing.T) {
	// Act
	Setup()
	defer Teardown()
	log := logmocks.NewMockLog()

	os.MkdirAll(filepath.Clean(testingDir), 0700)
	err := os.WriteFile(filepath.Clean(fakeConfigFilePath), []byte(`{"default":{"telemetryDisabledTillEpoch":10,"percentageLimit":50,"maxRolls":15,"maxRollSize":500,"exportPeriodMinutes":15}}`), 0600)
	if err != nil {
		log.Errorf("Error while opening config file: %v", err)
	}

	NewTelemetryDynamicConfiguration(log, true, filepath.Clean(fakeConfigFilePath))

	assert.Equal(t, float64(50), PercentageLimit("ssmagent"))
}

func TestExportPeriodWhenConfigInitialisedWithoutNamespace(t *testing.T) {
	// Act
	Setup()
	defer Teardown()
	log := logmocks.NewMockLog()

	os.MkdirAll(filepath.Clean(testingDir), 0700)
	err := os.WriteFile(filepath.Clean(fakeConfigFilePath), []byte(`{"default":{"telemetryDisabledTillEpoch":10,"percentageLimit":50,"maxRolls":15,"maxRollSize":500,"exportPeriodMinutes":15}}`), 0600)
	if err != nil {
		log.Errorf("Error while opening config file: %v", err)
	}

	NewTelemetryDynamicConfiguration(log, true, filepath.Clean(fakeConfigFilePath))

	assert.Equal(t, 15, ExportPeriod("ssmagent"))
}

func TestTelemetryDisabledTillWhenConfigInitialisedWithoutNamespace(t *testing.T) {
	// Act
	Setup()
	defer Teardown()
	log := logmocks.NewMockLog()

	os.MkdirAll(filepath.Clean(testingDir), 0700)
	err := os.WriteFile(filepath.Clean(fakeConfigFilePath), []byte(`{"default":{"telemetryDisabledTillEpoch":10,"percentageLimit":50,"maxRolls":15,"maxRollSize":500,"exportPeriodMinutes":15}}`), 0600)
	if err != nil {
		log.Errorf("Error while opening config file: %v", err)
	}

	NewTelemetryDynamicConfiguration(log, true, filepath.Clean(fakeConfigFilePath))

	assert.Equal(t, int64(10), TelemetryDisabledTill("ssmagent"))
}

func TestDefaultWhenConfigNotInitialised(t *testing.T) {
	// Act
	Setup()
	defer Teardown()

	assert.Equal(t, getDefaultConfiguration().MaxRolls, MaxRolls("ssmagent"))
	assert.Equal(t, getDefaultConfiguration().MaxRollSize, MaxRollSize("ssmagent"))
	assert.Equal(t, getDefaultConfiguration().PercentageLimit, PercentageLimit("ssmagent"))
	assert.Equal(t, getDefaultConfiguration().TelemetryDisabledTill, TelemetryDisabledTill("ssmagent"))
	assert.Equal(t, getDefaultConfiguration().ExportPeriod, ExportPeriod("ssmagent"))
}

func TestInitSavesConfigToDiskWhenNotPresentInitially(t *testing.T) {
	// Act
	Setup()
	defer Teardown()
	log := logmocks.NewMockLog()

	oldDynamicConfigFolderPath := GetDynamicConfigFolderPath
	GetDynamicConfigFolderPath = func() string {
		return testingDir
	}
	defer func() {
		GetDynamicConfigFolderPath = oldDynamicConfigFolderPath
	}()
	NewTelemetryDynamicConfiguration(log, true, filepath.Clean(fakeConfigFilePath))

	assert.FileExists(t, filepath.Clean(fakeConfigFilePath))
}

func TestMaxRollsUsesDefaultWhenNoConfigFileOnDisk(t *testing.T) {
	// Act
	Setup()
	defer Teardown()
	log := logmocks.NewMockLog()

	NewTelemetryDynamicConfiguration(log, true, filepath.Clean(fakeConfigFilePath))

	assert.Equal(t, getDefaultConfiguration().MaxRolls, MaxRolls("distributor"))
}

func TestMaxRollSizeUsesDefaultWhenNoConfigFileOnDisk(t *testing.T) {
	// Act
	Setup()
	defer Teardown()
	log := logmocks.NewMockLog()

	NewTelemetryDynamicConfiguration(log, true, filepath.Clean(fakeConfigFilePath))

	assert.Equal(t, getDefaultConfiguration().MaxRollSize, MaxRollSize("distributor"))
}

func TestPercentageLimitUsesDefaultWhenNoConfigFileOnDisk(t *testing.T) {
	// Act
	Setup()
	defer Teardown()
	log := logmocks.NewMockLog()

	NewTelemetryDynamicConfiguration(log, true, filepath.Clean(fakeConfigFilePath))

	assert.Equal(t, getDefaultConfiguration().PercentageLimit, PercentageLimit("distributor"))
}

func TestTelemetryDisabledTillUsesDefaultWhenNoConfigFileOnDisk(t *testing.T) {
	// Act
	Setup()
	defer Teardown()
	log := logmocks.NewMockLog()

	NewTelemetryDynamicConfiguration(log, true, filepath.Clean(fakeConfigFilePath))

	assert.Equal(t, getDefaultConfiguration().TelemetryDisabledTill, TelemetryDisabledTill("distributor"))
}

func TestExportPeriodUsesDefaultWhenNoConfigFileOnDisk(t *testing.T) {
	// Act
	Setup()
	defer Teardown()
	log := logmocks.NewMockLog()

	NewTelemetryDynamicConfiguration(log, true, filepath.Clean(fakeConfigFilePath))

	assert.Equal(t, getDefaultConfiguration().ExportPeriod, ExportPeriod("distributor"))
}

func TestMaxRollsWhenConfigInitialisedWithNamespace(t *testing.T) {
	// Act
	Setup()
	defer Teardown()
	log := logmocks.NewMockLog()

	os.MkdirAll(filepath.Clean(testingDir), 0700)
	err := os.WriteFile(filepath.Clean(fakeConfigFilePath), []byte(`{
		"default": {
			"telemetryDisabledTillEpoch": 55,
			"percentageLimit": 12,
			"maxRolls": 17,
			"maxRollSize": 502,
			"exportPeriodMinutes": 20
		},
		"distributor": {
			"telemetryDisabledTillEpoch": 89,
			"percentageLimit": 82,
			"maxRolls": 87,
			"maxRollSize": 902,
			"exportPeriodMinutes": 20
		}
	}`), 0600)
	if err != nil {
		log.Errorf("Error while opening config file: %v", err)
	}

	NewTelemetryDynamicConfiguration(log, true, filepath.Clean(fakeConfigFilePath))

	assert.Equal(t, 87, MaxRolls("distributor"))
}

func TestMaxRollSizeWhenConfigInitialisedWithNamespace(t *testing.T) {
	// Act
	Setup()
	defer Teardown()
	log := logmocks.NewMockLog()

	os.MkdirAll(filepath.Clean(testingDir), 0700)
	err := os.WriteFile(filepath.Clean(fakeConfigFilePath), []byte(`{
		"default": {
			"telemetryDisabledTillEpoch": 55,
			"percentageLimit": 12,
			"maxRolls": 17,
			"maxRollSize": 502,
			"exportPeriodMinutes": 20
		},
		"distributor": {
			"telemetryDisabledTillEpoch": 89,
			"percentageLimit": 82,
			"maxRolls": 87,
			"maxRollSize": 902,
			"exportPeriodMinutes": 20
		}
	}`), 0600)
	if err != nil {
		log.Errorf("Error while opening config file: %v", err)
	}

	NewTelemetryDynamicConfiguration(log, true, filepath.Clean(fakeConfigFilePath))

	assert.Equal(t, int64(902), MaxRollSize("distributor"))
}

func TestPercentageLimitWhenConfigInitialisedWithNamespace(t *testing.T) {
	// Act
	Setup()
	defer Teardown()
	log := logmocks.NewMockLog()

	os.MkdirAll(filepath.Clean(testingDir), 0700)
	err := os.WriteFile(filepath.Clean(fakeConfigFilePath), []byte(`{
		"default": {
			"telemetryDisabledTillEpoch": 55,
			"percentageLimit": 12,
			"maxRolls": 17,
			"maxRollSize": 502,
			"exportPeriodMinutes": 20
		},
		"distributor": {
			"telemetryDisabledTillEpoch": 89,
			"percentageLimit": 82,
			"maxRolls": 87,
			"maxRollSize": 902,
			"exportPeriodMinutes": 20
		}
	}`), 0600)
	if err != nil {
		log.Errorf("Error while opening config file: %v", err)
	}

	NewTelemetryDynamicConfiguration(log, true, filepath.Clean(fakeConfigFilePath))

	assert.Equal(t, float64(82), PercentageLimit("distributor"))
}

func TestPercentageLimitPrecisionWhenConfigInitialisedWithNamespace(t *testing.T) {
	// Act
	Setup()
	defer Teardown()
	log := logmocks.NewMockLog()

	os.MkdirAll(filepath.Clean(testingDir), 0700)
	err := os.WriteFile(filepath.Clean(fakeConfigFilePath), []byte(`{
		"default": {
			"telemetryDisabledTillEpoch": 55,
			"percentageLimit": 0.03412569,
			"maxRolls": 17,
			"maxRollSize": 502,
			"exportPeriodMinutes": 20
		},
		"distributor": {
			"telemetryDisabledTillEpoch": 89,
			"percentageLimit": 0.06412569,
			"maxRolls": 87,
			"maxRollSize": 902,
			"exportPeriodMinutes": 20
		}
	}`), 0600)
	if err != nil {
		log.Errorf("Error while opening config file: %v", err)
	}

	NewTelemetryDynamicConfiguration(log, true, filepath.Clean(fakeConfigFilePath))

	assert.Equal(t, float64(0.06412569), PercentageLimit("distributor"))
}

func TestExportPeriodWhenConfigInitialisedWithNamespace(t *testing.T) {
	// Act
	Setup()
	defer Teardown()
	log := logmocks.NewMockLog()

	os.MkdirAll(filepath.Clean(testingDir), 0700)
	err := os.WriteFile(filepath.Clean(fakeConfigFilePath), []byte(`{
		"default": {
			"telemetryDisabledTillEpoch": 55,
			"percentageLimit": 12,
			"maxRolls": 17,
			"maxRollSize": 502,
			"exportPeriodMinutes": 20
		},
		"distributor": {
			"telemetryDisabledTillEpoch": 89,
			"percentageLimit": 82,
			"maxRolls": 87,
			"maxRollSize": 902,
			"exportPeriodMinutes": 20
		}
	}`), 0600)
	if err != nil {
		log.Errorf("Error while opening config file: %v", err)
	}

	NewTelemetryDynamicConfiguration(log, true, filepath.Clean(fakeConfigFilePath))

	assert.Equal(t, 20, ExportPeriod("distributor"))
}

func TestTelemetryDisabledTillWhenConfigInitialisedWithNamespace(t *testing.T) {
	// Act
	Setup()
	defer Teardown()
	log := logmocks.NewMockLog()

	os.MkdirAll(filepath.Clean(testingDir), 0700)
	err := os.WriteFile(filepath.Clean(fakeConfigFilePath), []byte(`{
		"default": {
			"telemetryDisabledTillEpoch": 55,
			"percentageLimit": 12,
			"maxRolls": 17,
			"maxRollSize": 502,
			"exportPeriodMinutes": 20
		},
		"distributor": {
			"telemetryDisabledTillEpoch": 89,
			"percentageLimit": 82,
			"maxRolls": 87,
			"maxRollSize": 902,
			"exportPeriodMinutes": 20
		}
	}`), 0600)
	if err != nil {
		log.Errorf("Error while opening config file: %v", err)
	}

	NewTelemetryDynamicConfiguration(log, true, filepath.Clean(fakeConfigFilePath))

	assert.Equal(t, int64(89), TelemetryDisabledTill("distributor"))
}
