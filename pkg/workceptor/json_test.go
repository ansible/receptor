// +build !no_workceptor

package workceptor

import (
	"context"
	"github.com/project-receptor/receptor/pkg/netceptor"
	"io/ioutil"
	"os"
	"testing"
)

func newCommandWorker() WorkUnit {
	return &commandUnit{
		command:    "echo",
		baseParams: "foo",
	}
}

func TestWorkceptorJson(t *testing.T) {
	tmpdir, err := ioutil.TempDir(os.TempDir(), "receptor-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)
	nc := netceptor.New(nil, "test", nil)
	w, err := New(context.Background(), nc, tmpdir)
	if err != nil {
		t.Fatal(err)
	}
	err = w.RegisterWorker("command", newCommandWorker)
	if err != nil {
		t.Fatal(err)
	}
	cw, err := w.AllocateUnit("command", "")
	if err != nil {
		t.Fatal(err)
	}
	cw.UpdateFullStatus(func(status *StatusFileData) {
		ed, ok := status.ExtraData.(*commandExtraData)
		if !ok {
			t.Fatal("ExtraData type assertion failed")
		}
		ed.Pid = 12345
	})
	err = cw.Save()
	if err != nil {
		t.Fatal(err)
	}
	cw2 := newCommandWorker()
	cw2.Init(w, cw.ID(), "command", "")
	err = cw2.Load()
	if err != nil {
		t.Fatal(err)
	}
	ed2, ok := cw2.Status().ExtraData.(*commandExtraData)
	if !ok {
		t.Fatal("ExtraData type assertion failed")
	}
	if ed2.Pid != 12345 {
		t.Fatal("PID did not make it through")
	}
}
