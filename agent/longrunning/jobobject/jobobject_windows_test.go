package jobobject

import (
	"errors"
	"fmt"
	"syscall"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestSetInformationJobObject tests various scenarios for the setInformationJobObject function
func TestSetInformationJobObject(t *testing.T) {
	testCases := []struct {
		name           string
		setupFunc      func() (syscall.Handle, *JobObjectExtendedLimit)
		expectedError  bool
		errorValidator func(error) bool
	}{
		{
			name: "Successful Job Object Configuration",
			setupFunc: func() (syscall.Handle, *JobObjectExtendedLimit) {
				// Create a real job object
				jobHandle, err := windows.CreateJobObject(nil, nil)
				if err != nil {
					return 0, nil
				}

				handle := syscall.Handle(jobHandle)

				jobInfo := &JobObjectExtendedLimit{
					BasicLimitInformation: JobObjectBasicLimit{
						LimitFlags: jobObjectLimitkillonClose,
					},
				}
				return handle, jobInfo
			},
			expectedError: false,
		},
		{
			name: "Invalid Job Handle",
			setupFunc: func() (syscall.Handle, *JobObjectExtendedLimit) {
				// Use an invalid handle
				invalidHandle := syscall.Handle(0)

				jobInfo := &JobObjectExtendedLimit{
					BasicLimitInformation: JobObjectBasicLimit{
						LimitFlags: jobObjectLimitkillonClose,
					},
				}
				return invalidHandle, jobInfo
			},
			expectedError: true,
			errorValidator: func(err error) bool {
				return err != nil
			},
		},
		{
			name: "Nil Job Info",
			setupFunc: func() (syscall.Handle, *JobObjectExtendedLimit) {
				mockJobHandle := syscall.Handle(uintptr(unsafe.Pointer(new(int))))
				return mockJobHandle, nil
			},
			expectedError: true,
			errorValidator: func(err error) bool {
				return err != nil
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			jobHandle, jobInfo := tc.setupFunc()

			var err error
			if jobInfo != nil {
				err = setInformationJobObject(
					jobHandle,
					JobObjectExtendedLimitInformation,
					uintptr(unsafe.Pointer(jobInfo)),
					uint32(unsafe.Sizeof(*jobInfo)),
				)
			} else {
				err = setInformationJobObject(
					jobHandle,
					JobObjectExtendedLimitInformation,
					0,
					0,
				)
			}

			if tc.expectedError {
				assert.Error(t, err, "Expected an error")
				if tc.errorValidator != nil {
					assert.True(t, tc.errorValidator(err), "Error validation failed")
				}
			} else {
				assert.NoError(t, err, "Unexpected error")
			}
		})
	}

}

func TestAttachProcessToJobObject(t *testing.T) {
	testCases := []struct {
		name           string
		pid            uint32
		mockSetup      func(*MockSyscall)
		expectedError  bool
		errorValidator func(error) bool
	}{

		{
			name: "OpenProcess Failure",
			pid:  5678,
			mockSetup: func(ms *MockSyscall) {
				ms.On("OpenProcess",
					uint32(processSetQuotaAccess|processTerminateAccess),
					childprocessNotInheritHandle,
					uint32(5678)).
					Return(syscall.Handle(0), errors.New("process open failed"))
			},
			expectedError: true,
			errorValidator: func(err error) bool {
				return err.Error() == "process open failed"
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockSyscall := new(MockSyscall)

			// Setup mock expectations
			if tc.mockSetup != nil {
				tc.mockSetup(mockSyscall)
			}

			// Temporarily replace syscall functions
			originalOpenProcess := openProcessFn
			originalCloseHandle := closeHandleFn
			defer func() {
				openProcessFn = originalOpenProcess
				closeHandleFn = originalCloseHandle
			}()

			openProcessFn = mockSyscall.OpenProcess
			closeHandleFn = mockSyscall.CloseHandle

			// Set a mock job object handle
			SSMjobObject = syscall.Handle(999)

			// Call the function under test
			err := AttachProcessToJobObject(tc.pid)
			fmt.Printf("err %v", err)

			// Validate results
			if tc.expectedError {
				assert.Error(t, err)
				if tc.errorValidator != nil {
					assert.True(t, tc.errorValidator(err))
				}
			} else {
				assert.NoError(t, err)
			}

			// Verify mock expectations
			mockSyscall.AssertExpectations(t)
		})
	}
}

// MockSyscall is a mock for syscall operations
type MockSyscall struct {
	mock.Mock
}

func (m *MockSyscall) OpenProcess(desiredAccess uint32, inheritHandle bool, processId uint32) (syscall.Handle, error) {
	args := m.Called(desiredAccess, inheritHandle, processId)
	// Convert the first argument to Handle using syscall.Handle directly
	if h, ok := args.Get(0).(syscall.Handle); ok {
		return h, args.Error(1)
	}
	return syscall.Handle(0), args.Error(1)
}

func (m *MockSyscall) CloseHandle(handle syscall.Handle) error {
	args := m.Called(handle)
	return args.Error(0)
}

// Add AssignProcessToJobObject mock if not already present
func (m *MockSyscall) AssignProcessToJobObject(job syscall.Handle, process syscall.Handle) (uintptr, uintptr, error) {
	args := m.Called(job, process)
	return args.Get(0).(uintptr), args.Get(1).(uintptr), args.Error(2)
}
