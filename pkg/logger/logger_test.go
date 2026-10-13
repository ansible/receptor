package logger

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
	"testing"
)

func TestGetLogLevelByName(t *testing.T) {
	receptorLogger := NewReceptorLogger("")

	testCases := []struct {
		name  string
		error bool
	}{
		{name: "error"},
		{name: "warning"},
		{name: "info"},
		{name: "debug"},
		{name: "trace"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := receptorLogger.GetLogLevelByName(testCase.name)
			if err != nil {
				t.Error(err)
			}
		})
	}
}

func TestGetLogLevelByNameWithError(t *testing.T) {
	receptorLogger := NewReceptorLogger("")
	_, err := receptorLogger.GetLogLevelByName("does not exist")
	if err == nil {
		t.Error("should have error")
	}
}

func TestLogLevelToName(t *testing.T) {
	receptorLogger := NewReceptorLogger("")

	testCases := []struct {
		level int
	}{
		{level: 1},
		{level: 2},
		{level: 3},
		{level: 4},
		{level: 5},
	}

	for _, testCase := range testCases {
		name := fmt.Sprintf("level: %d", testCase.level)
		t.Run(name, func(t *testing.T) {
			_, err := receptorLogger.LogLevelToName(testCase.level)
			if err != nil {
				t.Error(err)
			}
		})
	}
}

func TestLogLevelToNameWithError(t *testing.T) {
	receptorLogger := NewReceptorLogger("")
	_, err := receptorLogger.LogLevelToName(123)
	if err == nil {
		t.Error("should have error")
	}
}

func TestLoglevelCfgInit(t *testing.T) {
	testCases := []struct {
		name      string
		level     string
		wantLevel int
		wantErr   bool
	}{
		{"error", "error", ErrorLevel, false},
		{"warning", "warning", WarningLevel, false},
		{"info", "info", InfoLevel, false},
		{"debug", "DEBUG", DebugLevel, false},
		{"trace", "trace", TraceLevel, false},
		{"invalid", "garbage", 0, true},
		{"empty", "", 0, true},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			cfg := LoglevelCfg{Level: tt.level}
			err := cfg.Init()
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for level %q, got nil", tt.level)
				}

				return
			}
			if err != nil {
				t.Fatalf("unexpected error for level %q: %v", tt.level, err)
			}
			got := GetLogLevel()
			if got != tt.wantLevel {
				t.Errorf("expected log level %d, got %d", tt.wantLevel, got)
			}
			SetGlobalLogLevel(InfoLevel) // Reset to default after test
		})
	}
}

func TestSanitizedLog(t *testing.T) {
	// Save and restore global logger and log level
	origLogger := GetLogLevel()
	defer SetGlobalLogLevel(origLogger)
	SetGlobalLogLevel(DebugLevel)

	testCases := []struct {
		name                  string
		level                 int
		format                string
		args                  []interface{}
		expectContains        []string
		expectNoNewlineInside bool
		registerLogger        bool
		loggerCheck           func(t *testing.T, level int, format string, v ...interface{})
	}{
		{
			name:                  "valid level with newline",
			level:                 InfoLevel,
			format:                "hello\nworld %d",
			args:                  []interface{}{42},
			expectContains:        []string{"INFO", "helloworld 42"},
			expectNoNewlineInside: true,
			registerLogger:        false,
		},
		{
			name:                  "invalid log level triggers error",
			level:                 9999,
			format:                "bad\nmessage %d",
			args:                  []interface{}{7},
			expectContains:        []string{"Log entry received with invalid level: badmessage 7"},
			expectNoNewlineInside: true,
			registerLogger:        false,
		},
		{
			name:                  "no prefix",
			level:                 InfoLevel,
			format:                "noprefix %s",
			args:                  []interface{}{"ok"},
			expectContains:        []string{"INFO", "noprefix ok"},
			expectNoNewlineInside: false,
			registerLogger:        false,
		},
		{
			name:                  "logger function registered",
			level:                 InfoLevel,
			format:                "delegated %d",
			args:                  []interface{}{1},
			expectContains:        []string{},
			expectNoNewlineInside: false,
			registerLogger:        true,
			loggerCheck: func(t *testing.T, level int, format string, v ...interface{}) {
				if level != InfoLevel || format != "delegated %d" || v[0].(int) != 1 {
					t.Errorf("logger delegate called with wrong args: %v %v %v", level, format, v)
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			oldLogger := GetRegisteredLogger()
			defer RegisterLogger(oldLogger)
			var called bool
			if tc.registerLogger {
				RegisterLogger(func(level int, format string, v ...interface{}) {
					called = true
					if tc.loggerCheck != nil {
						tc.loggerCheck(t, level, format, v...)
					}
				})
			} else {
				RegisterLogger(nil)
			}

			var buf bytes.Buffer
			rl := NewReceptorLogger("")
			rl.SetOutput(&buf)

			rl.sanitizedLog(tc.level, tc.format, tc.args...)

			got := buf.String()

			for _, substr := range tc.expectContains {
				if !strings.Contains(got, substr) {
					t.Errorf("expected log output to contain %q, got: %q", substr, got)
				}
			}

			if tc.expectNoNewlineInside {
				if strings.Contains(strings.TrimSuffix(got, "\n"), "\n") {
					t.Errorf("expected embedded newlines to be removed from log message, got: %q", got)
				}
			}

			if tc.registerLogger && !called {
				t.Error("expected registered logger function to be called")
			}
		})
	}
}

func TestLogWithInvalidLevel(t *testing.T) {
	origLevel := GetLogLevel()
	defer SetGlobalLogLevel(origLevel)
	SetGlobalLogLevel(DebugLevel)

	origLogger := GetRegisteredLogger()
	defer RegisterLogger(origLogger)
	RegisterLogger(nil)

	var buf bytes.Buffer
	rl := NewReceptorLogger("")
	rl.SetOutput(&buf)

	rl.log(9999, "bad\nmessage %d", 7)

	got := buf.String()
	if !strings.Contains(got, "Log entry received with invalid level: badmessage 7") {
		t.Errorf("expected sanitized invalid level error, got: %q", got)
	}
	if strings.Contains(strings.TrimSuffix(got, "\n"), "\n") {
		t.Errorf("expected embedded newlines removed, got: %q", got)
	}
}

func TestDebugPayload(t *testing.T) {
	var logBuffer bytes.Buffer
	SetGlobalLogLevel(4)
	receptorLogger := NewReceptorLogger("")
	receptorLogger.SetOutput(&logBuffer)
	payload := "Testing debugPayload"
	workUnitID := "1234"
	connectionType := "unix socket"

	debugPayloadTestCases := []struct {
		name           string
		debugPayload   int
		payload        string
		workUnitID     string
		connectionType string
		expectedLog    string
	}{
		{name: "debugPayload no log", debugPayload: 0, payload: "", workUnitID: "", connectionType: "", expectedLog: ""},
		{name: "debugPayload log level 1", debugPayload: 1, payload: "", workUnitID: "", connectionType: connectionType, expectedLog: fmt.Sprintf("PACKET TRACING ENABLED: Reading from %v", connectionType)},
		{name: "debugPayload log level 2 with workUnitID", debugPayload: 2, payload: "", workUnitID: workUnitID, connectionType: connectionType, expectedLog: fmt.Sprintf("PACKET TRACING ENABLED: Reading from %v with work unit %v", connectionType, workUnitID)},
		{name: "debugPayload log level 2 without workUnitID", debugPayload: 2, payload: "", workUnitID: "", connectionType: connectionType, expectedLog: fmt.Sprintf("PACKET TRACING ENABLED: Reading from %v", connectionType)},
		{name: "debugPayload log level 3 with workUnitID", debugPayload: 3, payload: payload, workUnitID: workUnitID, connectionType: connectionType, expectedLog: fmt.Sprintf("PACKET TRACING ENABLED: Reading from %v with work unit %v with a payload of: %v", connectionType, workUnitID, payload)},
		{name: "debugPayload log level 3 without workUnitID", debugPayload: 3, payload: payload, workUnitID: "", connectionType: connectionType, expectedLog: fmt.Sprintf("PACKET TRACING ENABLED: Reading from %v, work unit not created yet with a payload of: %v", connectionType, payload)},
		{name: "debugPayload log level 3 without workUnitID and payload is new line", debugPayload: 3, payload: "\n", workUnitID: "", connectionType: connectionType, expectedLog: fmt.Sprintf("PACKET TRACING ENABLED: Reading from %v, work unit not created yet with a payload of: %v", connectionType, "\n")},
		{name: "debugPayload log level 3 without workUnitID or payload", debugPayload: 3, payload: "", workUnitID: "", connectionType: connectionType, expectedLog: fmt.Sprintf("PACKET TRACING ENABLED: Reading from %v, work unit not created yet with a payload of: %v", connectionType, "")},
	}

	for _, testCase := range debugPayloadTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			receptorLogger.DebugPayload(testCase.debugPayload, testCase.payload, testCase.workUnitID, testCase.connectionType)
			testOutput := logBuffer.Bytes()

			if !strings.Contains(string(testOutput), testCase.expectedLog) {
				t.Errorf("failed to log correctly, expected: %v got %v", testCase.expectedLog, string(testOutput))
			}
			logBuffer.Reset()
		})
	}
}

func TestGetLogLevel(t *testing.T) {
	origLevel := GetLogLevel()
	defer SetGlobalLogLevel(origLevel)

	for _, level := range []int{ErrorLevel, WarningLevel, InfoLevel, DebugLevel, TraceLevel} {
		SetGlobalLogLevel(level)
		if got := GetLogLevel(); got != level {
			t.Errorf("GetLogLevel() = %d, want %d", got, level)
		}
	}
}

func TestLogMethods(t *testing.T) {
	origLevel := GetLogLevel()
	defer SetGlobalLogLevel(origLevel)

	origLogger := GetRegisteredLogger()
	defer RegisterLogger(origLogger)
	RegisterLogger(nil)

	type logCall func(rl *ReceptorLogger, msg string)

	cases := []struct {
		name      string
		minLevel  int
		levelName string
		call      logCall
	}{
		{"Warning", WarningLevel, "WARNING", func(rl *ReceptorLogger, msg string) { rl.Warning("%s", msg) }},
		{"Info", InfoLevel, "INFO", func(rl *ReceptorLogger, msg string) { rl.Info("%s", msg) }},
		{"Debug", DebugLevel, "DEBUG", func(rl *ReceptorLogger, msg string) { rl.Debug("%s", msg) }},
		{"Trace", TraceLevel, "TRACE", func(rl *ReceptorLogger, msg string) { rl.Trace("%s", msg) }},
	}

	for _, tc := range cases {
		t.Run(tc.name+"/emits at level", func(t *testing.T) {
			SetGlobalLogLevel(tc.minLevel)
			var buf bytes.Buffer
			rl := NewReceptorLogger("")
			rl.SetOutput(&buf)
			tc.call(rl, "hello")
			got := buf.String()
			if !strings.Contains(got, tc.levelName) {
				t.Errorf("expected %q in output, got: %q", tc.levelName, got)
			}
			if !strings.Contains(got, "hello") {
				t.Errorf("expected message body in output, got: %q", got)
			}
		})

		t.Run(tc.name+"/suppressed below level", func(t *testing.T) {
			SetGlobalLogLevel(tc.minLevel - 1)
			var buf bytes.Buffer
			rl := NewReceptorLogger("")
			rl.SetOutput(&buf)
			tc.call(rl, "hello")
			if buf.Len() != 0 {
				t.Errorf("expected no output below level, got: %q", buf.String())
			}
		})
	}
}

func TestSanitizedLogMethods(t *testing.T) {
	origLevel := GetLogLevel()
	defer SetGlobalLogLevel(origLevel)

	origLogger := GetRegisteredLogger()
	defer RegisterLogger(origLogger)
	RegisterLogger(nil)

	type logCall func(rl *ReceptorLogger, msg string)

	cases := []struct {
		name      string
		minLevel  int
		levelName string
		call      logCall
	}{
		{"SanitizedWarning", WarningLevel, "WARNING", func(rl *ReceptorLogger, msg string) { rl.SanitizedWarning("%s", msg) }},
		{"SanitizedInfo", InfoLevel, "INFO", func(rl *ReceptorLogger, msg string) { rl.SanitizedInfo("%s", msg) }},
		{"SanitizedDebug", DebugLevel, "DEBUG", func(rl *ReceptorLogger, msg string) { rl.SanitizedDebug("%s", msg) }},
		{"SanitizedTrace", TraceLevel, "TRACE", func(rl *ReceptorLogger, msg string) { rl.SanitizedTrace("%s", msg) }},
	}

	for _, tc := range cases {
		t.Run(tc.name+"/emits at level", func(t *testing.T) {
			SetGlobalLogLevel(tc.minLevel)
			var buf bytes.Buffer
			rl := NewReceptorLogger("")
			rl.SetOutput(&buf)
			tc.call(rl, "hello")
			got := buf.String()
			if !strings.Contains(got, tc.levelName) {
				t.Errorf("expected %q in output, got: %q", tc.levelName, got)
			}
			if !strings.Contains(got, "hello") {
				t.Errorf("expected message body in output, got: %q", got)
			}
		})

		t.Run(tc.name+"/suppressed below level", func(t *testing.T) {
			SetGlobalLogLevel(tc.minLevel - 1)
			var buf bytes.Buffer
			rl := NewReceptorLogger("")
			rl.SetOutput(&buf)
			tc.call(rl, "hello")
			if buf.Len() != 0 {
				t.Errorf("expected no output below level, got: %q", buf.String())
			}
		})

		t.Run(tc.name+"/strips embedded newlines", func(t *testing.T) {
			SetGlobalLogLevel(tc.minLevel)
			var buf bytes.Buffer
			rl := NewReceptorLogger("")
			rl.SetOutput(&buf)
			tc.call(rl, "line1\nline2")
			got := strings.TrimSuffix(buf.String(), "\n")
			if strings.Contains(got, "\n") {
				t.Errorf("expected embedded newlines stripped, got: %q", buf.String())
			}
		})
	}
}

func TestSetShowTrace(t *testing.T) {
	origLevel := GetLogLevel()
	defer SetGlobalLogLevel(origLevel)

	SetGlobalLogLevel(InfoLevel)
	rl := NewReceptorLogger("")
	rl.SetShowTrace(true)
	if GetLogLevel() != TraceLevel {
		t.Errorf("expected TraceLevel after SetShowTrace(true), got %d", GetLogLevel())
	}
}

func TestTraceCfgPrepare(t *testing.T) {
	origLevel := GetLogLevel()
	defer SetGlobalLogLevel(origLevel)

	cfg := TraceCfg{}
	if err := cfg.Prepare(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if GetLogLevel() != TraceLevel {
		t.Errorf("expected TraceLevel after TraceCfg.Prepare(), got %d", GetLogLevel())
	}
}

// syncBuffer is a concurrency-safe writer for capturing log output.
type syncBuffer struct {
	m   sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.m.Lock()
	defer b.m.Unlock()

	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.m.Lock()
	defer b.m.Unlock()

	return b.buf.String()
}

func TestLoggerAppendsSuffix(t *testing.T) {
	origLevel := GetLogLevel()
	defer SetGlobalLogLevel(origLevel)
	SetGlobalLogLevel(DebugLevel)

	var buf bytes.Buffer
	rl := NewReceptorLogger(`{"node_id":"control-a"}`)
	rl.SetOutput(&buf)

	rl.Info("plain line\n")
	rl.SanitizedInfo("sanitized line\n")

	got := buf.String()
	if strings.Count(got, `"node_id":"control-a"`) != 2 {
		t.Errorf("expected node_id on every line, got: %q", got)
	}
	if strings.Count(got, "\n") != 2 {
		t.Errorf("expected one line per log call, got: %q", got)
	}
}

func TestEmptySuffixAddsNothing(t *testing.T) {
	origLevel := GetLogLevel()
	defer SetGlobalLogLevel(origLevel)
	SetGlobalLogLevel(DebugLevel)

	var buf bytes.Buffer
	rl := NewReceptorLogger("")
	rl.SetOutput(&buf)

	rl.Info("plain line")

	got := strings.TrimSuffix(buf.String(), "\n")
	if !strings.HasSuffix(got, "plain line") {
		t.Errorf("expected no suffix, got: %q", got)
	}
}

func TestWithSuffixInheritsOutput(t *testing.T) {
	origLevel := GetLogLevel()
	defer SetGlobalLogLevel(origLevel)
	SetGlobalLogLevel(DebugLevel)

	var buf bytes.Buffer
	parent := NewReceptorLogger(`{"node_id":"control-a"}`)
	parent.SetOutput(&buf)

	child := parent.WithSuffix(`{"node_id":"control-a","remote_id":"execution-a"}`)
	child.Info("child line")

	got := buf.String()
	if !strings.Contains(got, `"remote_id":"execution-a"`) {
		t.Errorf("expected child suffix in parent's output, got: %q", got)
	}
	if !strings.Contains(got, "child line") {
		t.Errorf("expected message body in output, got: %q", got)
	}
}

// TestConcurrentSuffixIsolation validates that
// per-connection remote_id must never leak into another goroutine's lines.
func TestConcurrentSuffixIsolation(t *testing.T) {
	origLevel := GetLogLevel()
	defer SetGlobalLogLevel(origLevel)
	SetGlobalLogLevel(DebugLevel)

	out := &syncBuffer{}
	parent := NewReceptorLogger(`{"node_id":"control-a"}`)
	parent.SetOutput(out)

	const goroutines = 16

	const lines = 50

	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			remoteID := fmt.Sprintf("execution-%d", i)
			rl := parent.WithSuffix(fmt.Sprintf(`{"node_id":"control-a","remote_id":"%s"}`, remoteID))
			for j := 0; j < lines; j++ {
				rl.Info("work on %s is %d", remoteID, j)
			}
		}(i)
	}
	wg.Wait()

	for _, line := range strings.Split(out.String(), "\n") {
		if line == "" {
			continue
		}
		idx := strings.Index(line, "work on ")
		if idx == -1 {
			continue
		}
		var loggedRemote string
		if _, err := fmt.Sscanf(line[idx:], "work on %s", &loggedRemote); err != nil {
			t.Fatalf("could not parse log line %q: %v", line, err)
		}
		if !strings.Contains(line, `"remote_id":"`+loggedRemote+`"`) {
			t.Errorf("remote_id does not match the emitting goroutine: %s", line)
		}
	}
}

// TestConcurrentLevelLabelsStayConsistent validates that the
// level label and the message must be emitted atomically, so a line logged at
// one level can never carry another goroutine's level label.
func TestConcurrentLevelLabelsStayConsistent(t *testing.T) {
	origLevel := GetLogLevel()
	defer SetGlobalLogLevel(origLevel)
	SetGlobalLogLevel(TraceLevel)

	out := &syncBuffer{}
	rl := NewReceptorLogger(`{"node_id":"control-a"}`)
	rl.SetOutput(out)

	levels := []struct {
		name string
		log  func(string, ...interface{})
	}{
		{"INFO", rl.Info},
		{"DEBUG", rl.Debug},
		{"TRACE", rl.Trace},
		{"WARNING", rl.Warning},
		{"ERROR", rl.Error},
	}

	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		l := levels[i%len(levels)]
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				l.log("%s level marker", l.name)
			}
		}()
	}
	wg.Wait()

	for _, line := range strings.Split(out.String(), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			t.Fatalf("empty log line")
		}
		level := fields[0]
		if !strings.Contains(line, level+" level marker") {
			t.Errorf("level label does not match the message: %q", line)
		}
	}
}
