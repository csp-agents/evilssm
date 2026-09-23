package channel

import (
	"fmt"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/log"
	logmocks "github.com/aws/amazon-ssm-agent/agent/mocks/log"
	channelmocks "github.com/aws/amazon-ssm-agent/common/channel/mocks"
	"github.com/aws/amazon-ssm-agent/common/channel/utils"
	identityMocks "github.com/aws/amazon-ssm-agent/common/identity/mocks"
	"github.com/aws/amazon-ssm-agent/common/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.nanomsg.org/mangos/v3/protocol"
)

type NamedPipeChannelTestSuite struct {
	suite.Suite
	mockLog log.T
}

// Execute the test suite
func TestNamedPipeChannelTestSuite(t *testing.T) {
	suite.Run(t, new(NamedPipeChannelTestSuite))
}

func (suite *NamedPipeChannelTestSuite) SetupTest() {
	mockLog := logmocks.NewMockLog()
	suite.mockLog = mockLog
	getSurveyorSocket = func() (protocol.Socket, error) {
		return &channelmocks.Socket{}, nil
	}
	getRespondentSocket = func() (protocol.Socket, error) {
		return &channelmocks.Socket{}, nil
	}
}

func (suite *NamedPipeChannelTestSuite) TestInitialize_Success() {
	identity := &identityMocks.IAgentIdentity{}

	namedPipeChannelConn := NewNamedPipeChannel(suite.mockLog, identity)
	namedPipeChannelConn.Initialize(utils.Respondent)
	isInitialized := namedPipeChannelConn.IsChannelInitialized()
	assert.True(suite.T(), isInitialized, "Initialization failed for respondent")

	namedPipeChannelConn = NewNamedPipeChannel(suite.mockLog, identity)
	namedPipeChannelConn.Initialize(utils.Respondent)
	isInitialized = namedPipeChannelConn.IsChannelInitialized()
	assert.True(suite.T(), isInitialized, "Initialization failed for surveyor")
}

func (suite *NamedPipeChannelTestSuite) TestInitialize_Failure() {
	identity := &identityMocks.IAgentIdentity{}

	getSurveyorSocket = func() (protocol.Socket, error) {
		return &channelmocks.Socket{}, fmt.Errorf("error")
	}
	getRespondentSocket = func() (protocol.Socket, error) {
		return &channelmocks.Socket{}, fmt.Errorf("error")
	}

	namedPipeChannelConn := NewNamedPipeChannel(suite.mockLog, identity)
	namedPipeChannelConn.Initialize(utils.Respondent)
	isInitialized := namedPipeChannelConn.IsChannelInitialized()
	assert.False(suite.T(), isInitialized, "Initialization success for respondent")

	namedPipeChannelConn = NewNamedPipeChannel(suite.mockLog, identity)
	namedPipeChannelConn.Initialize(utils.Respondent)
	isInitialized = namedPipeChannelConn.IsChannelInitialized()
	assert.False(suite.T(), isInitialized, "Initialization success for surveyor")
}

func (suite *NamedPipeChannelTestSuite) TestRespondentDial_Success() {
	identity := &identityMocks.IAgentIdentity{}
	socket := &channelmocks.Socket{}
	socket.On("Dial", mock.Anything).Return(nil)
	getRespondentSocket = func() (protocol.Socket, error) {
		return socket, nil
	}

	namedPipeChannelConn := NewNamedPipeChannel(suite.mockLog, identity)
	namedPipeChannelConn.Initialize(utils.Respondent)
	isInitialized := namedPipeChannelConn.IsChannelInitialized()
	assert.True(suite.T(), isInitialized, "Initialization failed for respondent")
	namedPipeChannelConn.Dial("test")
	isDialSuccessFul := namedPipeChannelConn.IsDialSuccessful()
	assert.True(suite.T(), isDialSuccessFul, "Dialing unsuccessful for respondent")
	isListenSuccessFul := namedPipeChannelConn.IsListenSuccessful()
	assert.False(suite.T(), isListenSuccessFul, "Listening successful for respondent")
}

func (suite *NamedPipeChannelTestSuite) TestSurveyorListen_Success() {
	identity := &identityMocks.IAgentIdentity{}
	socket := &channelmocks.Socket{}
	socket.On("Listen", mock.Anything).Return(nil)
	getSurveyorSocket = func() (protocol.Socket, error) {
		return socket, nil
	}
	namedPipeChannelConn := NewNamedPipeChannel(suite.mockLog, identity)
	namedPipeChannelConn.Initialize(utils.Surveyor)
	isInitialized := namedPipeChannelConn.IsChannelInitialized()
	assert.True(suite.T(), isInitialized, "Initialization failed for surveyor")
	namedPipeChannelConn.Listen("test")
	isListenSuccessFul := namedPipeChannelConn.IsListenSuccessful()
	assert.True(suite.T(), isListenSuccessFul, "Listening unsuccessful for surveyor")
	isDialSuccess := namedPipeChannelConn.IsDialSuccessful()
	assert.False(suite.T(), isDialSuccess, "Dialing successful for surveyor")
}

func (suite *NamedPipeChannelTestSuite) TestRespondentDial_Failed() {
	identity := &identityMocks.IAgentIdentity{}
	socket := &channelmocks.Socket{}
	socket.On("Dial", mock.Anything).Return(fmt.Errorf("error"))
	getRespondentSocket = func() (protocol.Socket, error) {
		return socket, nil
	}

	namedPipeChannelConn := NewNamedPipeChannel(suite.mockLog, identity)
	namedPipeChannelConn.Initialize(utils.Respondent)
	isInitialized := namedPipeChannelConn.IsChannelInitialized()
	assert.True(suite.T(), isInitialized, "Initialization failed for respondent")
	namedPipeChannelConn.Dial("test")
	isDialSuccessFul := namedPipeChannelConn.IsDialSuccessful()
	assert.False(suite.T(), isDialSuccessFul, "Dialing successful for respondent")
	isListenSuccessFul := namedPipeChannelConn.IsListenSuccessful()
	assert.False(suite.T(), isListenSuccessFul, "Listening successful for respondent")
}

func (suite *NamedPipeChannelTestSuite) TestSurveyorListen_Failed() {
	identity := &identityMocks.IAgentIdentity{}
	socket := &channelmocks.Socket{}
	socket.On("Listen", mock.Anything).Return(fmt.Errorf("error"))
	getSurveyorSocket = func() (protocol.Socket, error) {
		return socket, nil
	}
	namedPipeChannelConn := NewNamedPipeChannel(suite.mockLog, identity)
	namedPipeChannelConn.Initialize(utils.Surveyor)
	isInitialized := namedPipeChannelConn.IsChannelInitialized()
	assert.True(suite.T(), isInitialized, "Initialization failed for surveyor")
	namedPipeChannelConn.Listen("test")
	isListenSuccessful := namedPipeChannelConn.IsListenSuccessful()
	assert.False(suite.T(), isListenSuccessful, "Listening successful for surveyor")
	isDialSuccessful := namedPipeChannelConn.IsDialSuccessful()
	assert.False(suite.T(), isDialSuccessful, "Dialing successful for surveyor")
}

func TestNamedPipeChannel_Close_ChannelNotInitialized(t *testing.T) {
	// Create a channel with nil socket (uninitialized state)
	channel := &namedPipeChannel{
		socket: nil,
	}

	// Call Close() and verify it returns ErrIPCChannelClosed
	err := channel.Close()

	assert.Error(t, err)
	assert.Equal(t, ErrIPCChannelClosed, err)
}

func TestNamedPipeChannel_Recv_ErrorPaths(t *testing.T) {
	tests := []struct {
		name          string
		setupChannel  func() *namedPipeChannel
		expectedError error
	}{
		{
			name: "channel not initialized",
			setupChannel: func() *namedPipeChannel {
				return &namedPipeChannel{
					socket: nil,
				}
			},
			expectedError: ErrIPCChannelClosed,
		},
		{
			name: "dial and listen unsuccessful",
			setupChannel: func() *namedPipeChannel {
				return &namedPipeChannel{
					socket: &channelmocks.Socket{},
				}
			},
			expectedError: ErrDialListenUnSuccessful,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			channel := tt.setupChannel()

			data, err := channel.Recv()

			assert.Nil(t, data)
			assert.Error(t, err)
			assert.Equal(t, tt.expectedError, err)
		})
	}
}

func TestNamedPipeChannel_SetOption_ChannelNotInitialized(t *testing.T) {
	channel := &namedPipeChannel{
		socket: nil,
	}

	err := channel.SetOption("some_option", "some_value")

	assert.Error(t, err)
	assert.Equal(t, ErrIPCChannelClosed, err)
}

func TestNamedPipeChannel_Listen_ChannelNotInitialized(t *testing.T) {
	channel := &namedPipeChannel{
		socket: nil,
	}

	err := channel.Listen("test-address")

	assert.Error(t, err)
	assert.Equal(t, ErrIPCChannelClosed, err)
}

func TestNamedPipeChannel_Dial_ChannelNotInitialized(t *testing.T) {
	channel := &namedPipeChannel{
		socket: nil,
	}

	err := channel.Dial("test-address")

	assert.Error(t, err)
	assert.Equal(t, ErrIPCChannelClosed, err)
}

func TestNamedPipeChannel_Send(t *testing.T) {
	tests := []struct {
		name               string
		message            *message.Message
		isInitialized      bool
		isListenSuccessful bool
		isDialSuccessful   bool
		socketSendError    error
		expectedError      error
		setupMock          func(*channelmocks.Socket)
	}{
		{
			name: "neither listen nor dial successful",
			message: &message.Message{
				Topic: "test-topic",
			},
			isInitialized:      true,
			isListenSuccessful: false,
			isDialSuccessful:   false,
			expectedError:      ErrDialListenUnSuccessful,
			setupMock:          func(m *channelmocks.Socket) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLog := logmocks.NewMockLog()
			mockSocket := &channelmocks.Socket{}

			tt.setupMock(mockSocket)

			channel := &namedPipeChannel{
				log:    mockLog,
				socket: mockSocket,
			}

			err := channel.Send(tt.message)

			t.Logf("Test: %s, Error: %v", tt.name, err)

			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedError.Error(), err.Error())
			} else {
				assert.NoError(t, err)
			}

			mockSocket.AssertExpectations(t)
		})
	}
}

func TestNamedPipeChannel_Send_ErrorPaths(t *testing.T) {
	tests := []struct {
		name          string
		message       *message.Message
		setupChannel  func() *namedPipeChannel
		expectedError error
	}{
		{
			name: "channel not initialized",
			message: &message.Message{
				Topic: "test-topic",
			},
			setupChannel: func() *namedPipeChannel {
				return &namedPipeChannel{
					socket: nil,
				}
			},
			expectedError: ErrIPCChannelClosed,
		},
		{
			name: "neither listen nor dial successful",
			message: &message.Message{
				Topic: "test-topic",
			},
			setupChannel: func() *namedPipeChannel {
				return &namedPipeChannel{
					socket: &channelmocks.Socket{},
				}
			},
			expectedError: ErrDialListenUnSuccessful,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			channel := tt.setupChannel()

			err := channel.Send(tt.message)

			assert.Error(t, err)
			assert.Equal(t, tt.expectedError, err)
		})
	}
}
