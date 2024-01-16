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

// FileSystemer represents a filesystem.
type FileSystemer interface {
	OpenFile(name string, flag int, perm os.FileMode) (*os.File, error)
	Stat(name string) (os.FileInfo, error)
	Open(name string) (*os.File, error)
	RemoveAll(path string) error
}

// FileSystem represents the real filesystem.
type FileSystem struct{}

// OpenFile opens a file on the filesystem.
func (FileSystem) OpenFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	return os.OpenFile(name, flag, perm)
}

// Stat retrieves the FileInfo for a given file name.
func (FileSystem) Stat(name string) (os.FileInfo, error) {
	return os.Stat(name)
}

// Open opens a file.
func (FileSystem) Open(name string) (*os.File, error) {
	return os.Open(name)
}

// RemoveAll removes path and any children it contains.
func (FileSystem) RemoveAll(path string) error {
	return os.RemoveAll(path)
}

// FileWriteCloser wraps io.WriteCloser.
type FileWriteCloser interface {
	io.WriteCloser
}

// FileReadCloser wraps io.ReadCloser.
type FileReadCloser interface {
	io.ReadCloser
}

// saveStdoutSize only the stdout size in the status metadata file in the unitdir.
func saveStdoutSize(unitdir string, stdoutSize int64) error {
	statusFilename := path.Join(unitdir, "status")
	si := &StatusFileData{}

	return si.UpdateFullStatus(statusFilename, func(status *StatusFileData) {
		status.StdoutSize = stdoutSize
	})
}

// STDoutWriter writes to a stdout file while also updating the status file.
type STDoutWriter struct {
	unitdir      string
	writer       FileWriteCloser
	bytesWritten int64
}

// NewStdoutWriter allocates a new stdoutWriter, which writes to both the stdout and status files.
func NewStdoutWriter(fs FileSystemer, unitdir string) (*STDoutWriter, error) {
	writer, err := fs.OpenFile(path.Join(unitdir, "stdout"), os.O_CREATE+os.O_WRONLY+os.O_SYNC, 0o600)
	if err != nil {
		return nil, err
	}

	return &STDoutWriter{
		unitdir:      unitdir,
		writer:       writer,
		bytesWritten: 0,
	}, nil
}

// Write writes data to the stdout file and status file, implementing io.Writer.
func (sw *STDoutWriter) Write(p []byte) (n int, err error) {
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
func (sw *STDoutWriter) Size() int64 {
	return sw.bytesWritten
}

// SetWriter sets the writer var.
func (sw *STDoutWriter) SetWriter(writer FileWriteCloser) {
	sw.writer = writer
}

// STDinReader reads from a stdin file and provides a Done function.
type STDinReader struct {
	reader   FileReadCloser
	lasterr  error
	doneChan chan struct{}
	doneOnce sync.Once
}

var errFileSizeZero = errors.New("file is empty")

// NewStdinReader allocates a new stdinReader, which reads from a stdin file and provides a Done function.
func NewStdinReader(fs FileSystemer, unitdir string) (*STDinReader, error) {
	stdinpath := path.Join(unitdir, "stdin")
	stat, err := fs.Stat(stdinpath)
	if err != nil {
		return nil, err
	}
	if stat.Size() == 0 {
		return nil, errFileSizeZero
	}
	reader, err := fs.Open(stdinpath)
	if err != nil {
		return nil, err
	}

	return &STDinReader{
		reader:   reader,
		lasterr:  nil,
		doneChan: make(chan struct{}),
		doneOnce: sync.Once{},
	}, nil
}

// Read reads data from the stdout file, implementing io.Reader.
func (sr *STDinReader) Read(p []byte) (n int, err error) {
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
func (sr *STDinReader) Done() <-chan struct{} {
	return sr.doneChan
}

// Error returns the most recent error encountered in the reader.
func (sr *STDinReader) Error() error {
	return sr.lasterr
}

// SetReader sets the reader var.
func (sr *STDinReader) SetReader(reader FileReadCloser) {
	sr.reader = reader
}
