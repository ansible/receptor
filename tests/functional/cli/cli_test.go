package main

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ansible/receptor/tests/utils"
)

func ConfirmListening(pid int, proto string) (bool, error) {
	out := bytes.Buffer{}
	cmd := exec.Command("lsof", "-tap", fmt.Sprint(pid), "-i", proto)
	cmd.Stdout = &out
	cmd.Run()

	if strings.Contains(out.String(), fmt.Sprint(pid)) {
		return true, nil
	}

	return false, nil
}

func TestHelp(t *testing.T) {
	t.Parallel()
	cmd := exec.Command("receptor", "--help")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
}

func TestListeners(t *testing.T) {
	t.Parallel()
	testTable := []struct {
		listener    string
		listenProto string
	}{
		{"--tcp-listener", "TCP"},
		{"--ws-listener", "TCP"},
		{"--udp-listener", "UDP"},
	}
	for _, data := range testTable {
		listener := data.listener
		listenProto := data.listenProto
		t.Run(listener, func(t *testing.T) {
			t.Parallel()
			receptorStdOut := bytes.Buffer{}
			cmd := exec.Command("receptor", "--node", "id=test", listener, "port=0")
			cmd.Stdout = &receptorStdOut
			err := cmd.Start()
			if err != nil {
				t.Fatal(err)
			}
			defer cmd.Process.Wait()
			defer cmd.Process.Kill()

			ctx1, cancel1 := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel1()

			success, err := utils.CheckUntilTimeoutWithErr(ctx1, 10*time.Millisecond, func() (bool, error) {
				return ConfirmListening(cmd.Process.Pid, listenProto)
			})
			if err != nil {
				t.Fatal(err)
			}
			if !success {
				t.Fatalf("Timed out while waiting for backend to start:\n%s", receptorStdOut.String())
			}
		})
	}
}

func TestSSLListeners(t *testing.T) {
	t.Parallel()
	testTable := []struct {
		listener string
	}{
		{"--tcp-listener"},
		{"--ws-listener"},
	}
	for _, data := range testTable {
		listener := data.listener
		t.Run(listener, func(t *testing.T) {
			t.Parallel()

			key, crt, err := utils.GenerateCert("test", "test", []string{"test"}, []string{"test"})
			if err != nil {
				t.Fatal(err)
			}

			receptorStdOut := bytes.Buffer{}
			port, err := utils.GetFreeTCPPort()
			if err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("receptor", "--node", "id=test", "--tls-server", "name=server-tls", fmt.Sprintf("cert=%s", crt), fmt.Sprintf("key=%s", key), listener, fmt.Sprintf("port=%d", port), "tls=server-tls")
			cmd.Stdout = &receptorStdOut
			err = cmd.Start()
			if err != nil {
				t.Fatal(err)
			}
			defer cmd.Process.Wait()
			defer cmd.Process.Kill()

			checkFunc := func() bool {
				opensslStdOut := bytes.Buffer{}
				opensslStdIn := bytes.Buffer{}
				opensslCmd := exec.Command("openssl", "s_client", "-connect", "localhost:"+strconv.Itoa(port))
				opensslCmd.Stdin = &opensslStdIn
				opensslCmd.Stdout = &opensslStdOut
				err = opensslCmd.Run()

				return err == nil
			}

			ctx1, cancel1 := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel1()

			success := utils.CheckUntilTimeout(ctx1, 10*time.Millisecond, checkFunc)
			if !success {
				t.Fatalf("Timed out while waiting for tls backend to start:\n%s", receptorStdOut.String())
			}
		})
	}
}

func TestNegativeCost(t *testing.T) {
	t.Parallel()
	testTable := []struct {
		listener string
	}{
		{"--tcp-listener"},
		{"--ws-listener"},
		{"--udp-listener"},
	}
	for _, data := range testTable {
		listener := data.listener
		t.Run(listener, func(t *testing.T) {
			t.Parallel()
			receptorStdOut := bytes.Buffer{}
			cmd := exec.Command("receptor", "--node", "id=test", listener, "port=0", "cost=-1")
			cmd.Stdout = &receptorStdOut
			err := cmd.Start()
			if err != nil {
				t.Fatal(err)
			}

			// Wait for our process to hopefully run and quit
			time.Sleep(500 * time.Millisecond)

			cmd.Process.Kill()
			cmd.Wait()
			if !strings.Contains(receptorStdOut.String(), "Error: connection cost must be positive") {
				t.Fatalf("Expected stdout: Error: connection cost must be positive, actual stdout: %s", receptorStdOut.String())
			}
		})
	}
}

func TestCostMap(t *testing.T) {
	t.Parallel()
	testTable := []struct {
		listener    string
		listenProto string
		costMaps    []string
	}{
		{"--tcp-listener", "TCP", []string{"{}", "{\"a\": 1}", "{\"a\": 1.1}", "{\"a\": 1.3, \"b\": 5.6, \"c\": 0.2}"}},
		{"--ws-listener", "TCP", []string{"{}", "{\"a\": 1}", "{\"a\": 1.1}", "{\"a\": 1.3, \"b\": 5.6, \"c\": 0.2}"}},
		{"--udp-listener", "UDP", []string{"{}", "{\"a\": 1}", "{\"a\": 1.1}", "{\"a\": 1.3, \"b\": 5.6, \"c\": 0.2}"}},
	}
	for _, data := range testTable {
		listener := data.listener
		listenProto := data.listenProto
		costMaps := make([]string, len(data.costMaps))
		copy(costMaps, data.costMaps)
		t.Run(listener, func(t *testing.T) {
			t.Parallel()
			for _, costMap := range costMaps {
				costMapCopy := costMap
				t.Run(costMapCopy, func(t *testing.T) {
					t.Parallel()
					receptorStdOut := bytes.Buffer{}
					cmd := exec.Command("receptor", "--node", "id=test", listener, "port=0", fmt.Sprintf("nodecost=%s", costMapCopy))
					cmd.Stdout = &receptorStdOut
					err := cmd.Start()
					if err != nil {
						t.Fatal(err)
					}
					defer cmd.Process.Wait()
					defer cmd.Process.Kill()

					ctx1, cancel1 := context.WithTimeout(context.Background(), 2*time.Second)
					defer cancel1()

					success, err := utils.CheckUntilTimeoutWithErr(ctx1, 10*time.Millisecond, func() (bool, error) {
						return ConfirmListening(cmd.Process.Pid, listenProto)
					})
					if err != nil {
						t.Fatal(err)
					}
					if !success {
						t.Fatalf("Timed out while waiting for backend to start:\n%s", receptorStdOut.String())
					}
				})
			}
		})
	}
}

func TestCosts(t *testing.T) {
	t.Parallel()
	testTable := []struct {
		listener    string
		listenProto string
		costs       []string
	}{
		{"--tcp-listener", "TCP", []string{"1", "1.5", "1.0", "0.2", "52", "23"}},
		{"--ws-listener", "TCP", []string{"1", "1.5", "1.0", "0.2", "52", "23"}},
		{"--udp-listener", "UDP", []string{"1", "1.5", "1.0", "0.2", "52", "23"}},
	}
	for _, data := range testTable {
		listener := data.listener
		listenProto := data.listenProto
		costs := make([]string, len(data.costs))
		copy(costs, data.costs)
		t.Run(listener, func(t *testing.T) {
			t.Parallel()
			for _, cost := range costs {
				costCopy := cost
				t.Run(costCopy, func(t *testing.T) {
					t.Parallel()
					receptorStdOut := bytes.Buffer{}
					cmd := exec.Command("receptor", "--node", "id=test", listener, "port=0", fmt.Sprintf("cost=%s", costCopy))
					cmd.Stdout = &receptorStdOut
					err := cmd.Start()
					if err != nil {
						t.Fatal(err)
					}
					defer cmd.Process.Wait()
					defer cmd.Process.Kill()

					ctx1, cancel1 := context.WithTimeout(context.Background(), 2*time.Second)
					defer cancel1()

					success, err := utils.CheckUntilTimeoutWithErr(ctx1, 10*time.Millisecond, func() (bool, error) {
						return ConfirmListening(cmd.Process.Pid, listenProto)
					})
					if err != nil {
						t.Fatal(err)
					}
					if !success {
						t.Fatalf("Timed out while waiting for backend to start:\n%s", receptorStdOut.String())
					}
				})
			}
		})
	}
}
