package channel

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/log"
	logmocks "github.com/aws/amazon-ssm-agent/agent/mocks/log"
	commProtocol "github.com/aws/amazon-ssm-agent/common/channel/protocol"
	"github.com/aws/amazon-ssm-agent/common/channel/protocol/mocks"
	"github.com/aws/amazon-ssm-agent/common/channel/utils"
	"github.com/aws/amazon-ssm-agent/common/identity"
	identityMocks "github.com/aws/amazon-ssm-agent/common/identity/mocks"
	"github.com/aws/amazon-ssm-agent/common/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type FileChannelTestSuite struct {
	suite.Suite
	mockLog log.T
}

// Execute the test suite
func TestFileChannelTestSuite(t *testing.T) {
	suite.Run(t, new(FileChannelTestSuite))
}

func (suite *FileChannelTestSuite) SetupTest() {
	mockLog := logmocks.NewMockLog()
	suite.mockLog = mockLog
	mockSurvey := &mocks.ISurvey{}
	getSurveyInstance = func(log log.T, identity identity.IAgentIdentity) commProtocol.ISurvey {
		return mockSurvey
	}
	mockRespondent := &mocks.IRespondent{}
	getRespondentInstance = func(log log.T, identity identity.IAgentIdentity) commProtocol.IRespondent {
		return mockRespondent
	}
}

func (suite *FileChannelTestSuite) TestInitialize_Success() {
	identityMock := &identityMocks.IAgentIdentity{}
	mockRespondent := &mocks.IRespondent{}
	getRespondentInstance = func(log log.T, identity identity.IAgentIdentity) commProtocol.IRespondent {
		return mockRespondent
	}
	fileChannelConn := NewFileChannel(suite.mockLog, identityMock)
	fileChannelConn.Initialize(utils.Respondent)
	isInitialized := fileChannelConn.IsChannelInitialized()
	assert.True(suite.T(), isInitialized, "Initialization failed for respondent")

	mockSurvey := &mocks.ISurvey{}
	getSurveyInstance = func(log log.T, identity identity.IAgentIdentity) commProtocol.ISurvey {
		return mockSurvey
	}
	fileChannelConn = NewFileChannel(suite.mockLog, identityMock)
	fileChannelConn.Initialize(utils.Surveyor)
	isInitialized = fileChannelConn.IsChannelInitialized()
	assert.True(suite.T(), isInitialized, "Initialization failed for surveyor")
}

func (suite *FileChannelTestSuite) TestInitialize_Failure() {
	identityMock := &identityMocks.IAgentIdentity{}
	fileChannelConn := NewFileChannel(suite.mockLog, identityMock)
	fileChannelConn.Initialize("")
	isInitialized := fileChannelConn.IsChannelInitialized()
	assert.False(suite.T(), isInitialized, "Initialization success")
}

func (suite *FileChannelTestSuite) TestRespondentDial_Success() {
	identityMock := &identityMocks.IAgentIdentity{}
	mockRespondent := &mocks.IRespondent{}
	mockRespondent.On("Dial", mock.Anything).Return(nil)
	getRespondentInstance = func(log log.T, identity identity.IAgentIdentity) commProtocol.IRespondent {
		return mockRespondent
	}

	fileChannelConn := NewFileChannel(suite.mockLog, identityMock)
	fileChannelConn.Initialize(utils.Respondent)
	isInitialized := fileChannelConn.IsChannelInitialized()
	assert.True(suite.T(), isInitialized, "Initialization failed for respondent")
	fileChannelConn.Dial("test")
	isDialSuccessFul := fileChannelConn.IsDialSuccessful()
	assert.True(suite.T(), isDialSuccessFul, "Dialing unsuccessful for respondent")
	isListenSuccessFul := fileChannelConn.IsListenSuccessful()
	assert.False(suite.T(), isListenSuccessFul, "Listening successful for respondent")
}

func (suite *FileChannelTestSuite) TestSurveyorListen_Success() {
	identityMock := &identityMocks.IAgentIdentity{}
	mockSurvey := &mocks.ISurvey{}
	mockSurvey.On("Listen", mock.Anything).Return(nil)
	getSurveyInstance = func(log log.T, identity identity.IAgentIdentity) commProtocol.ISurvey {
		return mockSurvey
	}
	fileChannelConn := NewFileChannel(suite.mockLog, identityMock)
	fileChannelConn.Initialize(utils.Surveyor)
	isInitialized := fileChannelConn.IsChannelInitialized()
	assert.True(suite.T(), isInitialized, "Initialization failed for surveyor")
	fileChannelConn.Listen("test")
	isListenSuccessFul := fileChannelConn.IsListenSuccessful()
	assert.True(suite.T(), isListenSuccessFul, "Listening unsuccessful for surveyor")
	isDialSuccess := fileChannelConn.IsDialSuccessful()
	assert.False(suite.T(), isDialSuccess, "Dialing successful for surveyor")
}

func (suite *FileChannelTestSuite) TestRespondentDial_Failed() {
	identityMock := &identityMocks.IAgentIdentity{}
	mockRespondent := &mocks.IRespondent{}
	mockRespondent.On("Dial", mock.Anything).Return(nil)
	getRespondentInstance = func(log log.T, identity identity.IAgentIdentity) commProtocol.IRespondent {
		return mockRespondent
	}

	fileChannelConn := NewNamedPipeChannel(suite.mockLog, identityMock)
	fileChannelConn.Initialize(utils.Respondent)
	isInitialized := fileChannelConn.IsChannelInitialized()
	assert.True(suite.T(), isInitialized, "Initialization failed for respondent")
	fileChannelConn.Dial("test")
	isDialSuccessFul := fileChannelConn.IsDialSuccessful()
	assert.False(suite.T(), isDialSuccessFul, "Dialing successful for respondent")
	isListenSuccessFul := fileChannelConn.IsListenSuccessful()
	assert.False(suite.T(), isListenSuccessFul, "Listening successful for respondent")
}

func (suite *FileChannelTestSuite) TestSurveyorListen_Failed() {
	identityMocks := &identityMocks.IAgentIdentity{}
	mockSurvey := &mocks.ISurvey{}
	mockSurvey.On("Listen", mock.Anything).Return(nil)
	getSurveyInstance = func(log log.T, identity identity.IAgentIdentity) commProtocol.ISurvey {
		return mockSurvey
	}
	fileChannelConn := NewNamedPipeChannel(suite.mockLog, identityMocks)
	fileChannelConn.Initialize(utils.Surveyor)
	isInitialized := fileChannelConn.IsChannelInitialized()
	assert.True(suite.T(), isInitialized, "Initialization failed for surveyor")
	fileChannelConn.Listen("test")
	isListenSuccessful := fileChannelConn.IsListenSuccessful()
	assert.False(suite.T(), isListenSuccessful, "Listening successful for surveyor")
	isDialSuccessful := fileChannelConn.IsDialSuccessful()
	assert.False(suite.T(), isDialSuccessful, "Dialing successful for surveyor")
}

func (suite *FileChannelTestSuite) TestSend_TableDriven() {
	testCases := []struct {
		name          string
		initialize    bool
		setupChannel  func(IChannel)
		expectedError error
	}{
		{
			name:          "not initialized",
			initialize:    false,
			setupChannel:  func(fc IChannel) {},
			expectedError: ErrIPCChannelClosed,
		},
		{
			name:          "initialized but no listen/dial",
			initialize:    true,
			setupChannel:  func(fc IChannel) {},
			expectedError: ErrDialListenUnSuccessful,
		},
		{
			name:          "success after listen",
			initialize:    true,
			setupChannel:  func(fc IChannel) { fc.Listen("test") },
			expectedError: nil,
		},
		{
			name:          "success after dial",
			initialize:    true,
			setupChannel:  func(fc IChannel) { fc.Dial("test") },
			expectedError: nil,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			identityMock := &identityMocks.IAgentIdentity{}
			mockProtocol := &mocks.ISurvey{}
			testMessage := &message.Message{}

			mockProtocol.On("Send", testMessage).Return(nil)
			mockProtocol.On("Listen", mock.Anything).Return(nil)
			mockProtocol.On("Dial", mock.Anything).Return(nil)

			getSurveyInstance = func(log log.T, identity identity.IAgentIdentity) commProtocol.ISurvey {
				return mockProtocol
			}

			fileChannelConn := NewFileChannel(suite.mockLog, identityMock)
			if tc.initialize {
				fileChannelConn.Initialize(utils.Surveyor)
			}

			tc.setupChannel(fileChannelConn)
			err := fileChannelConn.Send(testMessage)
			assert.Equal(suite.T(), tc.expectedError, err)
		})
	}
}

func (suite *FileChannelTestSuite) TestClose() {
	testCases := []struct {
		name          string
		expectedError error
	}{
		{
			name:          "successful close",
			expectedError: nil,
		},
		{
			name:          "failed close",
			expectedError: fmt.Errorf("close error"),
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			identityMock := &identityMocks.IAgentIdentity{}
			mockProtocol := &mocks.ISurvey{}

			mockProtocol.On("Close").Return(tc.expectedError)

			getSurveyInstance = func(log log.T, identity identity.IAgentIdentity) commProtocol.ISurvey {
				return mockProtocol
			}

			fileChannelConn := NewFileChannel(suite.mockLog, identityMock)
			fileChannelConn.Initialize(utils.Surveyor)

			err := fileChannelConn.Close()

			if tc.expectedError == nil {
				assert.Nil(suite.T(), err)
			} else {
				assert.Error(suite.T(), err)
				assert.Equal(suite.T(), tc.expectedError.Error(), err.Error())
			}

			mockProtocol.AssertExpectations(suite.T())
		})
	}
}

func (suite *FileChannelTestSuite) TestSetOption() {
	testCases := []struct {
		name          string
		optionName    string
		optionValue   interface{}
		expectedError error
	}{
		{
			name:          "successful setOption",
			optionName:    "testOption",
			optionValue:   "testValue",
			expectedError: nil,
		},
		{
			name:          "failed setOption",
			optionName:    "testOption",
			optionValue:   "testValue",
			expectedError: fmt.Errorf("setOption error"),
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			identityMock := &identityMocks.IAgentIdentity{}
			mockProtocol := &mocks.ISurvey{}

			mockProtocol.On("SetOption", tc.optionName, tc.optionValue).Return(tc.expectedError)

			getSurveyInstance = func(log log.T, identity identity.IAgentIdentity) commProtocol.ISurvey {
				return mockProtocol
			}

			fileChannelConn := NewFileChannel(suite.mockLog, identityMock)
			fileChannelConn.Initialize(utils.Surveyor)

			err := fileChannelConn.SetOption(tc.optionName, tc.optionValue)

			if tc.expectedError == nil {
				assert.Nil(suite.T(), err)
			} else {
				assert.Error(suite.T(), err)
				assert.Equal(suite.T(), tc.expectedError.Error(), err.Error())
			}

			mockProtocol.AssertExpectations(suite.T())
		})
	}
}

func (suite *FileChannelTestSuite) TestRecv() {
	testCases := []struct {
		name          string
		initialize    bool
		setupChannel  func(IChannel)
		mockResponse  []byte
		mockError     error
		expectedData  []byte
		expectedError error
	}{
		{
			name:          "channel not initialized",
			initialize:    false, // Don't initialize the channel
			setupChannel:  func(fc IChannel) {},
			mockResponse:  nil,
			mockError:     nil,
			expectedData:  nil,
			expectedError: ErrIPCChannelClosed,
		},
		{
			name:          "neither listen nor dial called",
			initialize:    true,
			setupChannel:  func(fc IChannel) {},
			mockResponse:  nil,
			mockError:     nil,
			expectedData:  nil,
			expectedError: ErrDialListenUnSuccessful,
		},
		{
			name:       "successful receive after listen",
			initialize: true,
			setupChannel: func(fc IChannel) {
				// Don't need to mock Listen here since it's handled in setupMock
			},
			mockResponse:  []byte("test message"),
			mockError:     nil,
			expectedData:  []byte("test message"),
			expectedError: nil,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			identityMock := &identityMocks.IAgentIdentity{}
			mockProtocol := &mocks.ISurvey{}

			if tc.name == "successful receive after listen" {
				mockProtocol.On("Listen", mock.Anything).Return(nil).Once()
				mockProtocol.On("Recv").Return(tc.mockResponse, tc.mockError).Once()
			}

			getSurveyInstance = func(log log.T, identity identity.IAgentIdentity) commProtocol.ISurvey {
				return mockProtocol
			}

			fileChannelConn := NewFileChannel(suite.mockLog, identityMock)

			// Only initialize if the test case requires it
			if tc.initialize {
				fileChannelConn.Initialize(utils.Surveyor)
			}

			// For the success case, we need to call Listen first
			if tc.name == "successful receive after listen" {
				err := fileChannelConn.Listen("test")
				assert.NoError(suite.T(), err)
			}

			data, err := fileChannelConn.Recv()

			assert.Equal(suite.T(), tc.expectedError, err)
			assert.Equal(suite.T(), tc.expectedData, data)
			mockProtocol.AssertExpectations(suite.T())
		})
	}
}

func (suite *FileChannelTestSuite) TestSurveyorListen_ErrorPath() {
	identityMock := &identityMocks.IAgentIdentity{}
	mockSurvey := &mocks.ISurvey{}
	expectedErr := errors.New("listen error")
	mockSurvey.On("Listen", mock.Anything).Return(expectedErr)

	getSurveyInstance = func(log log.T, identity identity.IAgentIdentity) commProtocol.ISurvey {
		return mockSurvey
	}

	fileChannelConn := NewFileChannel(suite.mockLog, identityMock)
	fileChannelConn.Initialize(utils.Surveyor)

	err := fileChannelConn.Listen("test")

	assert.Equal(suite.T(), expectedErr, err, "Expected error was not returned")
	assert.False(suite.T(), fileChannelConn.IsListenSuccessful(), "IsListenSuccessful should be false when error occurs")
	mockSurvey.AssertExpectations(suite.T())
}

func (suite *FileChannelTestSuite) TestSurveyorDial_ErrorPath() {
	identityMock := &identityMocks.IAgentIdentity{}
	mockSurvey := &mocks.ISurvey{}
	expectedErr := errors.New("dial error")
	mockSurvey.On("Dial", mock.Anything).Return(expectedErr)

	getSurveyInstance = func(log log.T, identity identity.IAgentIdentity) commProtocol.ISurvey {
		return mockSurvey
	}

	fileChannelConn := NewFileChannel(suite.mockLog, identityMock)
	fileChannelConn.Initialize(utils.Surveyor)

	err := fileChannelConn.Dial("test")

	assert.Equal(suite.T(), expectedErr, err, "Expected error was not returned")
	assert.False(suite.T(), fileChannelConn.IsDialSuccessful(), "IsDialSuccessful should be false when error occurs")
	mockSurvey.AssertExpectations(suite.T())
}
