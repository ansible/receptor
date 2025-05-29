package utils_test

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/ansible/receptor/pkg/utils"
)

// Time for a job to stop running.
const jobFinishTimeout = 500 * time.Millisecond

type fields struct {
	Ctx         context.Context
	JcCancel    context.CancelFunc
	Wg          *sync.WaitGroup
	JcRunning   bool
	RunningLock *sync.Mutex
}

func setupGoodFields() fields {
	goodCtx, goodCancel := context.WithCancel(context.Background())
	goodFields := &fields{
		Ctx:         goodCtx,
		JcCancel:    goodCancel,
		Wg:          &sync.WaitGroup{},
		JcRunning:   true,
		RunningLock: &sync.Mutex{},
	}

	return *goodFields
}

func TestJobContextRunning(t *testing.T) {
	tests := []struct {
		name   string
		fields fields
		want   bool
	}{
		{
			name:   "Positive",
			fields: setupGoodFields(),
			want:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mw := &utils.JobContext{
				Ctx:         tt.fields.Ctx,
				JcCancel:    tt.fields.JcCancel,
				Wg:          tt.fields.Wg,
				JcRunning:   tt.fields.JcRunning,
				RunningLock: tt.fields.RunningLock,
			}
			if got := mw.Running(); got != tt.want {
				t.Errorf("JobContext.Running() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestJobContextCancel(t *testing.T) {
	tests := []struct {
		name   string
		fields fields
	}{
		{
			name:   "Positive",
			fields: setupGoodFields(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mw := &utils.JobContext{
				Ctx:         tt.fields.Ctx,
				JcCancel:    tt.fields.JcCancel,
				Wg:          tt.fields.Wg,
				JcRunning:   tt.fields.JcRunning,
				RunningLock: tt.fields.RunningLock,
			}
			mw.Cancel()
		})
	}
}

func TestJobContextValue(t *testing.T) {
	type args struct {
		key interface{}
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		want   interface{}
	}{
		{
			name:   "Positive",
			fields: setupGoodFields(),
			want:   nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mw := &utils.JobContext{
				Ctx:         tt.fields.Ctx,
				JcCancel:    tt.fields.JcCancel,
				Wg:          tt.fields.Wg,
				JcRunning:   tt.fields.JcRunning,
				RunningLock: tt.fields.RunningLock,
			}
			if got := mw.Value(tt.args.key); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("JobContext.Value() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestJobContextDeadline(t *testing.T) {
	tests := []struct {
		name     string
		fields   fields
		wantTime time.Time
		wantOk   bool
	}{
		{
			name:     "Positive",
			fields:   setupGoodFields(),
			wantTime: time.Time{},
			wantOk:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mw := &utils.JobContext{
				Ctx:         tt.fields.Ctx,
				JcCancel:    tt.fields.JcCancel,
				Wg:          tt.fields.Wg,
				JcRunning:   tt.fields.JcRunning,
				RunningLock: tt.fields.RunningLock,
			}
			gotTime, gotOk := mw.Deadline()
			if !reflect.DeepEqual(gotTime, tt.wantTime) {
				t.Errorf("JobContext.Deadline() gotTime = %v, want %v", gotTime, tt.wantTime)
			}
			if gotOk != tt.wantOk {
				t.Errorf("JobContext.Deadline() gotOk = %v, want %v", gotOk, tt.wantOk)
			}
		})
	}
}

func TestJobContextErr(t *testing.T) {
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{
			name:    "Positive",
			fields:  setupGoodFields(),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mw := &utils.JobContext{
				Ctx:         tt.fields.Ctx,
				JcCancel:    tt.fields.JcCancel,
				Wg:          tt.fields.Wg,
				JcRunning:   tt.fields.JcRunning,
				RunningLock: tt.fields.RunningLock,
			}
			if err := mw.Err(); (err != nil) != tt.wantErr {
				t.Errorf("JobContext.Err() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestJobContextWait(t *testing.T) {
	tests := []struct {
		name   string
		fields fields
	}{
		{
			name:   "Positive",
			fields: setupGoodFields(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mw := &utils.JobContext{
				Ctx:         tt.fields.Ctx,
				JcCancel:    tt.fields.JcCancel,
				Wg:          tt.fields.Wg,
				JcRunning:   tt.fields.JcRunning,
				RunningLock: tt.fields.RunningLock,
			}
			mw.Wait()
		})
	}
}

func TestJobContext_NewJob(t *testing.T) {
	t.Run("Start new job when none running", func(t *testing.T) {
		jc := &utils.JobContext{}
		success := jc.NewJob(context.Background(), 1, true)
		if !success {
			t.Errorf("Expected to start new job, but failed")
		}
		if !jc.Running() {
			t.Errorf("Expected job context to be running")
		}
		jc.WorkerDone()
		jc.Wait()
	})

	t.Run("Prevent new job when already running and returnIfRunning=true, meaning don't replace", func(t *testing.T) {
		jc := &utils.JobContext{}
		ctx := context.Background()
		started := jc.NewJob(ctx, 1, true)
		if !started {
			t.Fatalf("Failed to start initial job")
		}
		defer jc.WorkerDone()
		replaced := jc.NewJob(ctx, 1, true)
		if replaced {
			t.Errorf("Expected NewJob to return false when already running and returnIfRunning=true")
		}
	})

	t.Run("Replace running job when returnIfRunning=false, meaning replace the workers with new ones.", func(t *testing.T) {
		jc := &utils.JobContext{}
		ctx := context.Background()

		if !jc.NewJob(ctx, 1, true) {
			t.Fatalf("Failed to start initial job")
		}
		initialWg := jc.Wg

		jc.WorkerDone()
		jc.Wait() // but not for finished job
		if !jc.Running() {
			t.Errorf("Expected job context to be running after initial job done")
		}

		replaced := jc.NewJob(ctx, 2, false) // Start a new job with 2 workers, specifying that you want them replaced.
		if !replaced {
			t.Errorf("Expected NewJob to replace current job when returnIfRunning=false")
		}

		if jc.Wg == initialWg {
			t.Errorf("Expected WaitGroup to be replaced on new job, but it's the same instance")
		}

		jc.WorkerDone()
		jc.WorkerDone()
		jc.Wait()
		if err := WaitUntilFinished(jc, jobFinishTimeout); err != nil {
			t.Fatalf("JobContext did not finish in time: %v", err)
		}

		if jc.Running() {
			t.Errorf("Expected job context to not be running after workers done")
		}
	})
}

func TestWaitUntilFinishedTimeout(t *testing.T) {
	jc := &utils.JobContext{}

	startHungJob(jc)

	timeout := 10 * time.Millisecond
	start := time.Now()
	err := WaitUntilFinished(jc, timeout)
	elapsed := time.Since(start)

	if err == nil || err.Error() != "timeout" {
		t.Errorf("expected timeout error, got: %v", err)
	}

	if elapsed < timeout {
		t.Errorf("expected to wait at least %v, waited only %v", timeout, elapsed)
	}
}

func startHungJob(jc *utils.JobContext) {
	ctx := context.Background()
	ok := jc.NewJob(ctx, 1, false) // spawn 1 worker, allow override = false
	if !ok {
		panic("failed to start hung job")
	}

	go func() {
		// Simulate a never-ending job (never calls WorkerDone)
		select {} // or: <-ctx.Done()
	}()
}

// WaitUntilFinished waits until the JobContext is no longer running, or until timeout.
// It polls the Running() state using a small delay to avoid a tight loop.
// In order to simulate a hung job, we do not call mw.Wait() here, as that would block indefinitely.
// If the job is not finished within the timeout, it returns an error.
// The thought being that this could be used to record jobs that do not finish in a reasonable time.
// These jobs may be stuck and could be recorded in a database or log messages for later investigation.
// Out of scope for this unit test ticket so recording here for later work.
func WaitUntilFinished(mw *utils.JobContext, timeout time.Duration) error {
	done := make(chan struct{})
	go func() {
		for mw.Running() {
			time.Sleep(1 * time.Millisecond)
		}
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("timeout")
	}
}
