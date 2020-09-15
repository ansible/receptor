package controlsvc

import (
	"github.com/project-receptor/receptor/pkg/netceptor"
	"io"
)

// ControlCommandType is a type of command that can be run from the control service
type ControlCommandType interface {
	InitFromString(string) (ControlCommand, error)
	InitFromJSON(map[string]interface{}) (ControlCommand, error)
}

// ControlCommand is an instance of a command that is being run from the control service
type ControlCommand interface {
	ControlFunc(*netceptor.Netceptor, ControlFuncOperations) (map[string]interface{}, error)
}

// ControlFuncOperations provides callbacks for control services to take actions
type ControlFuncOperations interface {
	BridgeConn(message string, bc io.ReadWriteCloser, bcName string) error
	ReadFromConn(message string, out io.Writer) error
	WriteToConn(message string, in chan []byte) error
	Close() error
}
