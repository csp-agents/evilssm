package rundaemon

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestValidateDaemonInput_SuccessfulValidation tests successful input validation
func TestValidateDaemonInput_SuccessfulValidation(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "testdata")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	packageLocation := filepath.Join(tempDir, "CheckMyHash.txt")
	_, err = os.Create(packageLocation)
	if err != nil {
		t.Fatal(err)
	}

	// Test cases for valid inputs
	testCases := []struct {
		name        string
		input       ConfigureDaemonPluginInput
		expectError bool
	}{
		{
			name: "Valid Start Action",
			input: ConfigureDaemonPluginInput{
				Name:            "valid_daemon",
				Action:          "Start",
				PackageLocation: packageLocation,
				Command:         "./start_script.sh",
			},
			expectError: false,
		},
		{
			name: "Valid Name with Hyphen",
			input: ConfigureDaemonPluginInput{
				Name:            "valid-daemon_name",
				Action:          "Start",
				PackageLocation: packageLocation,
				Command:         "./start_script.sh",
			},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateDaemonInput(tc.input)
			fmt.Printf("Package location: %s", tc.input.PackageLocation)
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidateDaemonInput_InvalidInputs tests various invalid input scenarios
func TestValidateDaemonInput_InvalidInputs(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "testdata")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	packageLocation := filepath.Join(tempDir, "CheckMyHash.txt")
	_, err = os.Create(packageLocation)
	if err != nil {
		t.Fatal(err)
	}

	testCases := []struct {
		name        string
		input       ConfigureDaemonPluginInput
		expectedErr string
	}{
		{
			name: "Empty Daemon Name",
			input: ConfigureDaemonPluginInput{
				Name:            "",
				Action:          "Start",
				PackageLocation: packageLocation,
				Command:         "./start_script.sh",
			},
			expectedErr: "daemon name is missing",
		}, {
			name: "Invalid Input period",
			input: ConfigureDaemonPluginInput{
				Name:            "test..value",
				Action:          "Start",
				PackageLocation: packageLocation,
				Command:         "./start_script.sh",
			},
			expectedErr: "Invalid daemon name, must start with letter or _; end with letter, number, or _; and contain only letters, numbers, -, _, or single . characters",
		}, {
			name: "Invalid Input Special Character",
			input: ConfigureDaemonPluginInput{
				Name:            "test$value",
				Action:          "Start",
				PackageLocation: packageLocation,
				Command:         "./start_script.sh",
			},
			expectedErr: "Invalid daemon name, must start with letter or _; end with letter, number, or _; and contain only letters, numbers, -, _, or single . characters",
		},
		{
			name: "Invalid Daemon Name",
			input: ConfigureDaemonPluginInput{
				Name:            "invalid daemon!name",
				Action:          "Start",
				PackageLocation: packageLocation,
				Command:         "./start_script.sh",
			},
			expectedErr: "Invalid daemon name",
		},
		{
			name: "Empty Package Location",
			input: ConfigureDaemonPluginInput{
				Name:            "valid_daemon",
				Action:          "Start",
				PackageLocation: "",
				Command:         "./start_script.sh",
			},

			expectedErr: "daemon location is missing",
		},
		{
			name: "Missing Command for Start Action",
			input: ConfigureDaemonPluginInput{
				Name:            "valid_daemon",
				Action:          "Start",
				PackageLocation: packageLocation,
				Command:         "",
			},
			expectedErr: "daemon launch command is missing",
		}, {
			name: "Incorrect Package Location",
			input: ConfigureDaemonPluginInput{
				Name:            "valid_daemon",
				Action:          "Start",
				PackageLocation: filepath.Join("testdata", "invalid.txt"),
				Command:         "./start_script.sh",
			},
			expectedErr: "daemon location does not exist",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateDaemonInput(tc.input)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.expectedErr)
		})
	}
}
