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

	"github.com/project-receptor/receptor/tests/functional/lib/utils"
)

func ConfirmListening(pid int) (bool, error) {
	pidString := "pid=" + strconv.Itoa(pid)
	var out bytes.Buffer
	out = bytes.Buffer{}
	ssCmd := exec.Command("ss", "-tulnp")
	ssCmd.Stdout = &out
	err := ssCmd.Run()
	if err != nil {
		return false, err
	}
	if strings.Contains(out.String(), pidString) {
		return true, nil
	}
	return false, nil
}

func TestHelp(t *testing.T) {
	t.Parallel()
	cmd := exec.Command("receptor", "--help")
	err := cmd.Run()
	if err != nil {
		t.Fatal(err)
	}
}

func TestListeners(t *testing.T) {
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
			cmd := exec.Command("receptor", "--node", "id=test", listener, "port=0")
			cmd.Stdout = &receptorStdOut
			err := cmd.Start()
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := cmd.Process.Kill(); err != nil {
					panic(err)
				}
				if _, err := cmd.Process.Wait(); err != nil {
					panic(err)
				}
			}()

			ctx, _ := context.WithTimeout(context.Background(), 2*time.Second)
			success, err := utils.CheckUntilTimeoutWithErr(ctx, 10*time.Millisecond, func() (bool, error) {
				return ConfirmListening(cmd.Process.Pid)
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

			key, crt, err := utils.GenerateCert("test", "localhost", []string{"localhost"}, nil)
			if err != nil {
				t.Fatal(err)
			}

			receptorStdOut := bytes.Buffer{}
			port := utils.ReserveTCPPort()
			defer utils.FreeTCPPort(port)
			cmd := exec.Command("receptor", "--node", "id=test", "--tls-server", "name=server-tls", fmt.Sprintf("cert=%s", crt), fmt.Sprintf("key=%s", key), listener, fmt.Sprintf("port=%d", port), "tls=server-tls")
			cmd.Stdout = &receptorStdOut
			err = cmd.Start()
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := cmd.Process.Kill(); err != nil {
					panic(err)
				}
				if _, err := cmd.Process.Wait(); err != nil {
					panic(err)
				}
			}()

			checkFunc := func() bool {
				opensslStdOut := bytes.Buffer{}
				opensslStdIn := bytes.Buffer{}
				opensslCmd := exec.Command("openssl", "s_client", "-connect", "localhost:"+strconv.Itoa(port))
				opensslCmd.Stdin = &opensslStdIn
				opensslCmd.Stdout = &opensslStdOut
				err = opensslCmd.Run()
				if err == nil {
					return true
				}
				return false
			}

			ctx, _ := context.WithTimeout(context.Background(), 2*time.Second)
			success := utils.CheckUntilTimeout(ctx, 10*time.Millisecond, checkFunc)
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
			time.Sleep(100 * time.Millisecond)

			if err := cmd.Process.Kill(); err != nil {
				t.Fatal(err)
			}
			if _, err := cmd.Process.Wait(); err != nil {
				t.Fatal(err)
			}
			if receptorStdOut.String() != "Error: connection cost must be positive\n" {
				t.Fatalf("Expected stdout: Error: connection cost must be positive, actual stdout: %s", receptorStdOut.String())
			}
		})
	}
}

func TestCostMap(t *testing.T) {
	t.Parallel()
	testTable := []struct {
		listener string
		costMaps []string
	}{
		{"--tcp-listener", []string{"{}", "{\"a\": 1}", "{\"a\": 1.1}", "{\"a\": 1.3, \"b\": 5.6, \"c\": 0.2}"}},
		{"--ws-listener", []string{"{}", "{\"a\": 1}", "{\"a\": 1.1}", "{\"a\": 1.3, \"b\": 5.6, \"c\": 0.2}"}},
		{"--udp-listener", []string{"{}", "{\"a\": 1}", "{\"a\": 1.1}", "{\"a\": 1.3, \"b\": 5.6, \"c\": 0.2}"}},
	}
	for _, data := range testTable {
		listener := data.listener
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
					defer func() {
						if err := cmd.Process.Kill(); err != nil {
							panic(err)
						}
						if _, err := cmd.Process.Wait(); err != nil {
							panic(err)
						}
					}()

					ctx, _ := context.WithTimeout(context.Background(), 2*time.Second)
					success, err := utils.CheckUntilTimeoutWithErr(ctx, 10*time.Millisecond, func() (bool, error) {
						return ConfirmListening(cmd.Process.Pid)
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
		listener string
		costs    []string
	}{
		{"--tcp-listener", []string{"1", "1.5", "1.0", "0.2", "52", "23"}},
		{"--ws-listener", []string{"1", "1.5", "1.0", "0.2", "52", "23"}},
		{"--udp-listener", []string{"1", "1.5", "1.0", "0.2", "52", "23"}},
	}
	for _, data := range testTable {
		listener := data.listener
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
					defer func() {
						if err := cmd.Process.Kill(); err != nil {
							panic(err)
						}
						if _, err := cmd.Process.Wait(); err != nil {
							panic(err)
						}
					}()

					ctx, _ := context.WithTimeout(context.Background(), 2*time.Second)
					success, err := utils.CheckUntilTimeoutWithErr(ctx, 10*time.Millisecond, func() (bool, error) {
						return ConfirmListening(cmd.Process.Pid)
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
