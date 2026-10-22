//go:build !no_workceptor
// +build !no_workceptor

package workceptor

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ansible/receptor/pkg/logger"
	"k8s.io/client-go/tools/remotecommand"
)

// Stubs by interface embedding rather than gomock: this is an internal test, and importing
// mock_workceptor from package workceptor would be an import cycle. Only the methods the retry
// path touches are implemented; anything else would panic, which is the intent.
type stubNetceptorForStdinRetry struct {
	NetceptorForWorkceptor
}

func (s *stubNetceptorForStdinRetry) GetLogger() *logger.ReceptorLogger {
	return logger.NewReceptorLogger("stdin-retry-test")
}

type stubKubeAPIForStdinRetry struct {
	KubeAPIer
	failures int
	calls    int
	err      error
}

func (s *stubKubeAPIForStdinRetry) StreamWithContext(_ context.Context, _ remotecommand.Executor, _ remotecommand.StreamOptions) error {
	s.calls++
	if s.calls <= s.failures {
		return s.err
	}

	return nil
}

func newKubeUnitForStdinRetry(api KubeAPIer) *KubeUnit {
	ctx, cancel := context.WithCancel(context.Background())

	return &KubeUnit{
		BaseWorkUnitForWorkUnit: &BaseWorkUnit{
			ctx:    ctx,
			cancel: cancel,
			w: &Workceptor{
				nc: &stubNetceptorForStdinRetry{},
			},
		},
		KubeAPIWrapperInstance: api,
	}
}

// The error the API server returns while the kubelet on a freshly joined node has no serving
// certificate yet. It is transient, so the stream has to be retried for long enough to outlast it.
var errDialingBackend = errors.New("error dialing backend: remote error: tls: internal error")

func TestStreamStdinWithRetrySucceedsAfterTransientFailures(t *testing.T) {
	t.Setenv("RECEPTOR_KUBE_TIMEOUT_START", "1ms")
	t.Setenv("RECEPTOR_KUBE_RETRY_COUNT", "10")

	api := &stubKubeAPIForStdinRetry{failures: 4, err: errDialingBackend}
	kw := newKubeUnitForStdinRetry(api)

	if err := kw.streamStdinWithRetry(nil, nil, "ns", "pod"); err != nil {
		t.Fatalf("expected the stream to succeed once the transient error clears, got: %s", err)
	}

	if api.calls != 5 {
		t.Errorf("expected 5 attempts (4 failures then success), got %d", api.calls)
	}
}

func TestStreamStdinWithRetryReturnsErrorWhenRetriesExhausted(t *testing.T) {
	t.Setenv("RECEPTOR_KUBE_TIMEOUT_START", "1ms")
	t.Setenv("RECEPTOR_KUBE_RETRY_COUNT", "3")

	api := &stubKubeAPIForStdinRetry{failures: 99, err: errDialingBackend}
	kw := newKubeUnitForStdinRetry(api)

	err := kw.streamStdinWithRetry(nil, nil, "ns", "pod")
	if err == nil {
		t.Fatal("expected an error once the retries are exhausted")
	}

	if api.calls != 3 {
		t.Errorf("expected exactly RECEPTOR_KUBE_RETRY_COUNT attempts, got %d", api.calls)
	}
}

// The point of the change: the wait between attempts grows, so RECEPTOR_KUBE_RETRY_COUNT can span a
// certificate window measured in tens of seconds. With a fixed delay it could not, at any count.
func TestStreamStdinWithRetryBacksOff(t *testing.T) {
	t.Setenv("RECEPTOR_KUBE_TIMEOUT_START", "10ms")
	t.Setenv("RECEPTOR_KUBE_RETRY_COUNT", "5")

	api := &stubKubeAPIForStdinRetry{failures: 99, err: errDialingBackend}
	kw := newKubeUnitForStdinRetry(api)

	start := time.Now()
	_ = kw.streamStdinWithRetry(nil, nil, "ns", "pod")
	elapsed := time.Since(start)

	// Five attempts wait four times, multipliers 1,2,3,5 against a 10ms base: 110ms. A fixed 10ms delay would be 40ms.
	if elapsed < 100*time.Millisecond {
		t.Errorf("expected the delays to grow across attempts, total wait was only %s", elapsed)
	}
}

// Nothing follows the last attempt, so there is no wait after it, and cancelling the work unit ends a wait
// in progress rather than leaving the goroutine asleep.
func TestStreamStdinWithRetryStopsWaitingWhenCancelled(t *testing.T) {
	t.Setenv("RECEPTOR_KUBE_TIMEOUT_START", "10s")
	t.Setenv("RECEPTOR_KUBE_RETRY_COUNT", "5")

	api := &stubKubeAPIForStdinRetry{failures: 99, err: errDialingBackend}
	kw := newKubeUnitForStdinRetry(api)
	time.AfterFunc(20*time.Millisecond, kw.GetCancel())

	start := time.Now()
	err := kw.streamStdinWithRetry(nil, nil, "ns", "pod")
	elapsed := time.Since(start)

	if !errors.Is(err, errDialingBackend) {
		t.Errorf("expected the last stream error, got: %v", err)
	}
	if api.calls != 1 {
		t.Errorf("expected cancellation to end the first wait, got %d attempts", api.calls)
	}
	if elapsed > time.Second {
		t.Errorf("expected cancellation to end the wait promptly, took %s", elapsed)
	}
}

func TestStreamStdinWithRetryDoesNotWaitAfterTheLastAttempt(t *testing.T) {
	t.Setenv("RECEPTOR_KUBE_TIMEOUT_START", "10s")
	t.Setenv("RECEPTOR_KUBE_RETRY_COUNT", "1")

	api := &stubKubeAPIForStdinRetry{failures: 99, err: errDialingBackend}
	kw := newKubeUnitForStdinRetry(api)

	start := time.Now()
	_ = kw.streamStdinWithRetry(nil, nil, "ns", "pod")
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("expected no wait after the only attempt, took %s", elapsed)
	}
}
