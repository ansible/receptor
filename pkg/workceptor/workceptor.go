package workceptor

import (
	"fmt"
	"github.com/ghjm/sockceptor/pkg/controlsock"
	"github.com/ghjm/sockceptor/pkg/netceptor"
	"strings"

	//    "github.com/ghjm/sockceptor/pkg/controlsock"
	"github.com/ghjm/sockceptor/pkg/debug"
	//    "strings"
)

// WorkType represents a unique type of worker
type WorkType interface {
	Start(string) (string, error)
	List() ([]string, error)
	Status(string) (bool, bool, string, error)
	Cancel(string) error
}

// Workceptor is the main object that handles unit-of-work management
type Workceptor struct {
	workTypes map[string]WorkType
	nc        *netceptor.Netceptor
	li        *netceptor.Listener
}

// New constructs a new Workceptor instance
func New(nc *netceptor.Netceptor) (*Workceptor, error) {
	w := &Workceptor{
		workTypes: make(map[string]WorkType),
		nc:        nc,
	}
	debug.Printf("Starting worker status services\n")
	err := controlsock.MainInstance().AddControlFunc("work", w.workFunc)
	if err != nil {
		return nil, fmt.Errorf("could not add work control function: %s", err)
	}
	w.li, err = w.nc.Listen("workstat")
	if err != nil {
		return nil, fmt.Errorf("could not listen for workstat service: %s", err)
	}
	go w.runWorkService()
	return w, nil
}

var mainInstance *Workceptor

// MainInstance returns a global singleton instance of Workceptor
func MainInstance() *Workceptor {
	if mainInstance == nil {
		var err error
		mainInstance, err = New(netceptor.MainInstance)
		if err != nil {
			panic(err)
		}
	}
	return mainInstance
}

func (w *Workceptor) runWorkService() {
	ws := controlsock.NewServer(false, nil)
	_ = ws.AddControlFunc("work", w.workFunc)
	for {
		conn, err := w.li.Accept()
		if err != nil {
			debug.Printf("Error accepting connection on work service: %s\n", err)
			return
		}
		go ws.RunSockServer(conn)
	}
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

// Worker function called by the control socket and workstat service to process a single command
func (w *Workceptor) workFunc(cs controlsock.Sock, params string) error {
	if len(params) == 0 {
		_ = cs.Printf("Bad command. Use start, list, status or cancel.\n")
		return nil
	}
	tokens := strings.Split(params, " ")
	switch tokens[0] {
	case "start":
		if len(tokens) < 2 {
			return cs.Printf("Must specify work type.\n")
		}
		workType := tokens[1]
		params := ""
		if len(tokens) > 2 {
			params = strings.Join(tokens[2:], " ")
		}
		wT, ok := w.workTypes[workType]
		if !ok {
			return cs.Printf("Unknown work type %s.\n", workType)
		}
		ident, err := wT.Start(params)
		if err != nil {
			return cs.Printf("Error starting work: %s.\n", err)
		}
		return cs.Printf("%s\n", ident)
	case "list":
		err := cs.Printf("%-10s %-10s %-8s %-8s %s\n", "Type", "Ident", "Done", "Success", "Status")
		if err != nil {
			return err
		}
		for workType := range w.workTypes {
			wT, ok := w.workTypes[workType]
			if !ok {
				return cs.Printf("Unknown work type %s.\n", workType)
			}
			work, err := wT.List()
			if err != nil {
				return cs.Printf("Error listing work: %s.\n", err)
			}
			for workItem := range work {
				exited, succeeded, status, err := wT.Status(work[workItem])
				if err != nil {
					return cs.Printf("Error getting work status: %s.\n", err)
				}
				err = cs.Printf("%-10s %-10s %-8t %-8t %s\n", workType, work[workItem], exited, succeeded, status)
				if err != nil {
					return err
				}
			}
		}
	case "status":
		if len(tokens) < 3 {
			return cs.Printf("Must specify work type and identifier.")
		}
		workType := tokens[1]
		wT, ok := w.workTypes[workType]
		if !ok {
			return cs.Printf("Unknown work type %s.", workType)
		}
		ident := tokens[2]
		exited, succeeded, status, err := wT.Status(ident)
		if err != nil {
			return cs.Printf("Error getting work status: %s.\n", err)
		}
		return cs.Printf("Done: %t, Success: %t, Status: %s\n", exited, succeeded, status)
	case "cancel":
		if len(tokens) < 3 {
			return cs.Printf("Must specify work type and identifier.")
		}
		workType := tokens[1]
		wT, ok := w.workTypes[workType]
		if !ok {
			return cs.Printf("Unknown work type %s.", workType)
		}
		ident := tokens[2]
		err := wT.Cancel(ident)
		if err != nil {
			return cs.Printf("Error cancelling work: %s.\n", err)
		}
		return cs.Printf("Cancelled %s\n", ident)
	}
	return nil
}
