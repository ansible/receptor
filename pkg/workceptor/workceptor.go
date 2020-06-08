package workceptor

import (
	"fmt"
	"github.com/ghjm/sockceptor/pkg/controlsvc"
	"github.com/ghjm/sockceptor/pkg/netceptor"
	"io"
	"net"
	"strings"

	//    "github.com/ghjm/sockceptor/pkg/controlsvc"
	"github.com/ghjm/sockceptor/pkg/debug"
	//    "strings"
)

// WorkType represents a unique type of worker
type WorkType interface {
	Start(string) (string, error)
	List() ([]string, error)
	Status(string) (bool, bool, string, error)
	Cancel(string) error
	Get(string, string) (io.ReadCloser, error)
}

// Workceptor is the main object that handles unit-of-work management
type Workceptor struct {
	workTypes map[string]WorkType
}

// New constructs a new Workceptor instance
func New(nc *netceptor.Netceptor, cs *controlsvc.Server) (*Workceptor, error) {
	w := &Workceptor{
		workTypes: make(map[string]WorkType),
	}
	debug.Printf("Starting worker status services\n")
	err := cs.AddControlFunc("work", w.workFunc)
	if err != nil {
		return nil, fmt.Errorf("could not add work control function: %s", err)
	}
	return w, nil
}

var mainInstance *Workceptor

// MainInstance returns a global singleton instance of Workceptor
func MainInstance() *Workceptor {
	if mainInstance == nil {
		var err error
		mainInstance, err = New(netceptor.MainInstance, controlsvc.MainInstance())
		if err != nil {
			panic(err)
		}
	}
	return mainInstance
}

// RegisterWorker notifies the Workceptor of a new kind of work that can be done
func (w *Workceptor) RegisterWorker(service string, worker WorkType) error {
	_, ok := w.workTypes[service]
	if ok {
		return fmt.Errorf("worker ")
	}
	w.workTypes[service] = worker
	return nil
}

// Worker function called by the control service to process a "work" command
func (w *Workceptor) workFunc(conn net.Conn, params string) error {
	if len(params) == 0 {
		_ = controlsvc.Printf(conn, "Bad command. Use start, list, status or cancel.\n")
		return nil
	}
	tokens := strings.Split(params, " ")
	switch tokens[0] {
	case "start":
		if len(tokens) < 2 {
			return controlsvc.Printf(conn, "Must specify work type.\n")
		}
		workType := tokens[1]
		params := ""
		if len(tokens) > 2 {
			params = strings.Join(tokens[2:], " ")
		}
		wT, ok := w.workTypes[workType]
		if !ok {
			return controlsvc.Printf(conn, "Unknown work type %s.\n", workType)
		}
		ident, err := wT.Start(params)
		if err != nil {
			return controlsvc.Printf(conn, "Error starting work: %s.\n", err)
		}
		return controlsvc.Printf(conn, "%s\n", ident)
	case "list":
		err := controlsvc.Printf(conn, "%-10s %-10s %-8s %-8s %s\n", "Type", "Ident", "Done", "Success", "Status")
		if err != nil {
			return err
		}
		for workType := range w.workTypes {
			wT, ok := w.workTypes[workType]
			if !ok {
				return controlsvc.Printf(conn, "Unknown work type %s.\n", workType)
			}
			work, err := wT.List()
			if err != nil {
				return controlsvc.Printf(conn, "Error listing work: %s.\n", err)
			}
			for workItem := range work {
				exited, succeeded, status, err := wT.Status(work[workItem])
				if err != nil {
					return controlsvc.Printf(conn, "Error getting work status: %s.\n", err)
				}
				err = controlsvc.Printf(conn, "%-10s %-10s %-8t %-8t %s\n", workType, work[workItem], exited, succeeded, status)
				if err != nil {
					return err
				}
			}
		}
	case "status":
		if len(tokens) < 3 {
			return controlsvc.Printf(conn, "Must specify work type and identifier.\n")
		}
		workType := tokens[1]
		wT, ok := w.workTypes[workType]
		if !ok {
			return controlsvc.Printf(conn, "Unknown work type %s.\n", workType)
		}
		ident := tokens[2]
		exited, succeeded, status, err := wT.Status(ident)
		if err != nil {
			return controlsvc.Printf(conn, "Error getting work status: %s.\n", err)
		}
		return controlsvc.Printf(conn, "Done: %t, Success: %t, Status: %s\n", exited, succeeded, status)
	case "cancel":
		if len(tokens) < 3 {
			return controlsvc.Printf(conn, "Must specify work type and identifier.\n")
		}
		workType := tokens[1]
		wT, ok := w.workTypes[workType]
		if !ok {
			return controlsvc.Printf(conn, "Unknown work type %s.\n", workType)
		}
		ident := tokens[2]
		err := wT.Cancel(ident)
		if err != nil {
			return controlsvc.Printf(conn, "Error cancelling work: %s.\n", err)
		}
		return controlsvc.Printf(conn, "Cancelled %s\n", ident)
	case "get":
		if len(tokens) < 4 {
			return controlsvc.Printf(conn, "Must specify work type, identifier and stream.\n")
		}
		workType := tokens[1]
		wT, ok := w.workTypes[workType]
		if !ok {
			return controlsvc.Printf(conn, "Unknown work type %s.\n", workType)
		}
		ident := tokens[2]
		stream := tokens[3]
		iorc, err := wT.Get(ident, stream)
		if err != nil {
			return controlsvc.Printf(conn, "Error getting stream: %s.\n", err)
		}
		_, err = io.Copy(conn, iorc)
		if err != nil {
			return controlsvc.Printf(conn, "Error copying stream: %s.\n", err)
		}
		return controlsvc.Printf(conn, "--- End of Stream ---\n")
	}
	return nil
}
