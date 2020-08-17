package main

import (
	"bytes"
	"fmt"
	"github.com/project-receptor/receptor/tests/functional/lib/utils"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestHelp(t *testing.T) {
	t.Parallel()
	cmd := exec.Command("receptor", "--help")
	err := cmd.Run()
	if err != nil {
		t.Fatal(err)
	}
}

func TestListenTCP(t *testing.T) {
	t.Parallel()
	tcpPort := utils.ReserveTCPPort()
	defer utils.FreeTCPPort(tcpPort)
	receptorStdOut := bytes.Buffer{}
	cmd := exec.Command("receptor", "--node", "id=test", "--tcp-listener", fmt.Sprintf("port=%d", tcpPort))
	cmd.Stdout = &receptorStdOut
	err := cmd.Start()

	done := false
	pidString := "pid=" + strconv.Itoa(cmd.Process.Pid)
	var out bytes.Buffer
	for timeout := 2 * time.Second; timeout > 0 && !done; {
		out = bytes.Buffer{}
		ssCmd := exec.Command("ss", "-tuanp")
		ssCmd.Stdout = &out
		err = ssCmd.Run()
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(out.String(), "\n") {
			if strings.Contains(line, pidString) && strings.Contains(line, strconv.Itoa(tcpPort)) {
				done = true
				break
			}
		}
		time.Sleep(100 * time.Millisecond)
		timeout -= 100 * time.Millisecond
	}
	cmd.Process.Kill()
	cmd.Process.Wait()
	if done == false {
		t.Fatalf("Timed out while waiting for TCP backend to start:\n%s", receptorStdOut.String())
	}
}

func TestListenWS(t *testing.T) {
	t.Parallel()
	tcpPort := utils.ReserveTCPPort()
	defer utils.FreeTCPPort(tcpPort)
	receptorStdOut := bytes.Buffer{}
	cmd := exec.Command("receptor", "--node", "id=test", "--ws-listener", fmt.Sprintf("port=%d", tcpPort))
	cmd.Stdout = &receptorStdOut
	err := cmd.Start()

	done := false
	pidString := "pid=" + strconv.Itoa(cmd.Process.Pid)
	var out bytes.Buffer
	for timeout := 2 * time.Second; timeout > 0 && !done; {
		out = bytes.Buffer{}
		ssCmd := exec.Command("ss", "-tuanp")
		ssCmd.Stdout = &out
		err = ssCmd.Run()
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(out.String(), "\n") {
			if strings.Contains(line, pidString) && strings.Contains(line, strconv.Itoa(tcpPort)) {
				done = true
				break
			}
		}
		time.Sleep(100 * time.Millisecond)
		timeout -= 100 * time.Millisecond
	}
	cmd.Process.Kill()
	cmd.Process.Wait()
	if done == false {
		t.Fatalf("Timed out while waiting for WS backend to start:\n%s", receptorStdOut.String())
	}
}

func TestListenUDP(t *testing.T) {
	t.Parallel()
	udpPort := utils.ReserveUDPPort()
	defer utils.FreeUDPPort(udpPort)
	receptorStdOut := bytes.Buffer{}
	cmd := exec.Command("receptor", "--node", "id=test", "--udp-listener", fmt.Sprintf("port=%d", udpPort))
	cmd.Stdout = &receptorStdOut
	err := cmd.Start()

	done := false
	pidString := "pid=" + strconv.Itoa(cmd.Process.Pid)
	var out bytes.Buffer
	for timeout := 2 * time.Second; timeout > 0 && !done; {
		out = bytes.Buffer{}
		ssCmd := exec.Command("ss", "-tuanp")
		ssCmd.Stdout = &out
		err = ssCmd.Run()
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(out.String(), "\n") {
			if strings.Contains(line, pidString) && strings.Contains(line, strconv.Itoa(udpPort)) {
				done = true
				break
			}
		}
		time.Sleep(100 * time.Millisecond)
		timeout -= 100 * time.Millisecond
	}
	cmd.Process.Kill()
	cmd.Process.Wait()
	if done == false {
		t.Fatalf("Timed out while waiting for UDP backend to start:\n%s", receptorStdOut.String())
	}
}

func TestNegativeCost(t *testing.T) {
	t.Parallel()
	tcpPort := utils.ReserveTCPPort()
	defer utils.FreeTCPPort(tcpPort)
	receptorStdOut := bytes.Buffer{}
	cmd := exec.Command("receptor", "--node", "id=test", "--tcp-listener", fmt.Sprintf("port=%d", tcpPort), "cost=-1")
	cmd.Stdout = &receptorStdOut
	err := cmd.Start()
	if err != nil {
		t.Fatal(err)
	}

	cmd.Process.Kill()
	cmd.Process.Wait()
	if cmd.ProcessState.ExitCode() == 0 {
		t.Fatalf("receptor exited with exitcode: 0\nreceptor Stdout:\n%s", receptorStdOut.String())
	}
}
