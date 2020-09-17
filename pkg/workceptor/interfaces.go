package workceptor

// WorkUnit represents a local unit of work
type WorkUnit interface {
	SetParams(params map[string]string) error
	ID() string
	UnitDir() string
	StatusFileName() string
	StdoutFileName() string
	Save() error
	Load() error
	UpdateBasicStatus(state int, detail string, stdoutSize int64)
	UpdateFullStatus(statusFunc func(*StatusFileData))
	LastUpdateError() error
	Status() *StatusFileData
	Start() error
	Restart() error
	Cancel() error
	Release(force bool) error
}

// NewWorkerFunc represents a factory of WorkUnit instances
type NewWorkerFunc func(w *Workceptor, unitID string, workType string) WorkUnit

// StatusFileData is the structure of the JSON data saved to a status file.
// This struct should only contain value types, except for ExtraData.
type StatusFileData struct {
	State      int
	Detail     string
	StdoutSize int64
	WorkType   string
	ExtraData  interface{}
}
