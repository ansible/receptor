//go:build !no_workceptor
// +build !no_workceptor

package workceptor

import (
	"errors"
	"io"
	"os"
	"path"
	"sync"
)

// saveStdoutSize only the stdout size in the status metadata file in the unitdir.
func saveStdoutSize(unitdir string, stdoutSize int64) error {
	statusFilename := path.Join(unitdir, "status")
	si := &StatusFileData{}

	return si.UpdateFullStatus(statusFilename, func(status *StatusFileData) {
		status.StdoutSize = stdoutSize
	})
}

// stdoutWriter writes to a stdout file while also updating the status file.
type stdoutWriter struct {
	unitdir      string
	writer       io.Writer
	bytesWritten int64
}

// newStdoutWriter allocates a new stdoutWriter, which writes to both the stdout and status files.
func newStdoutWriter(unitdir string) (*stdoutWriter, error) {
	writer, err := os.OpenFile(path.Join(unitdir, "stdout"), os.O_CREATE+os.O_WRONLY+os.O_SYNC, 0o600)
	if err != nil {
		return nil, err
	}

	return &stdoutWriter{
		unitdir:      unitdir,
		writer:       writer,
		bytesWritten: 0,
	}, nil
}

// Write writes data to the stdout file and status file, implementing io.Writer.
func (sw *stdoutWriter) Write(p []byte) (n int, err error) {
	wn, werr := sw.writer.Write(p)
	var serr error
	if wn > 0 {
		sw.bytesWritten += int64(wn)
		serr = saveStdoutSize(sw.unitdir, sw.bytesWritten)
	}
	if werr != nil {
		return wn, werr
	}

	return wn, serr
}

// Size returns the current size of the stdout file.
func (sw *stdoutWriter) Size() int64 {
	return sw.bytesWritten
}

// stdinReader reads from a stdin file and provides a Done function.
type stdinReader struct {
	reader   io.Reader
	lasterr  error
	doneChan chan struct{}
	doneOnce sync.Once
}

var errFileSizeZero = errors.New("file is empty")

// newStdinReader allocates a new stdinReader, which reads from a stdin file and provides a Done function.
func newStdinReader(unitdir string) (*stdinReader, error) {
	stdinpath := path.Join(unitdir, "stdin")
	stat, err := os.Stat(stdinpath)
	if err != nil {
		return nil, err
	}
	if stat.Size() == 0 {
		return nil, errFileSizeZero
	}
	reader, err := os.Open(stdinpath)
	if err != nil {
		return nil, err
	}

	return &stdinReader{
		reader:   reader,
		lasterr:  nil,
		doneChan: make(chan struct{}),
		doneOnce: sync.Once{},
	}, nil
}

// Read reads data from the stdout file, implementing io.Reader.
func (sr *stdinReader) Read(p []byte) (n int, err error) {
	n, err = sr.reader.Read(p)
	if err != nil {
		sr.lasterr = err
		sr.doneOnce.Do(func() {
			close(sr.doneChan)
		})
	}

	return
}

// Done returns a channel that will be closed on error (including EOF) in the reader.
func (sr *stdinReader) Done() <-chan struct{} {
	return sr.doneChan
}

// Error returns the most recent error encountered in the reader.
func (sr *stdinReader) Error() error {
	return sr.lasterr
}
