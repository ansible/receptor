package mesh

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/ansible/receptor/pkg/netceptor"
	"github.com/ansible/receptor/tests/utils"
)

func TestTCPSSLConnections(t *testing.T) {
	t.Parallel()

	for _, proto := range []string{"tcp", "ws"} {
		proto := proto
		t.Run(proto, func(t *testing.T) {
			t.Parallel()

			m := NewLibMesh()

			caKey, caCrt, err := utils.GenerateCA("ca", "localhost")
			if err != nil {
				t.Fatal(err)
			}
			key1, crt1, err := utils.GenerateCertWithCA("node1", caKey, caCrt, "localhost", []string{"localhost"}, []string{"node1"})
			if err != nil {
				t.Fatal(err)
			}
			key2, crt2, err := utils.GenerateCertWithCA("node2", caKey, caCrt, "localhost", []string{"localhost"}, []string{"node2"})
			if err != nil {
				t.Fatal(err)
			}
			key3, crt3, err := utils.GenerateCertWithCA("node3", caKey, caCrt, "localhost", []string{"localhost"}, []string{"node3"})
			if err != nil {
				t.Fatal(err)
			}

			defer func() {
				t.Log(m.LogWriter.String())
			}()

			node1 := m.NewLibNode("node1")
			node1.TLSServerConfigs = []*netceptor.TLSServerConfig{
				{
					Name:              "server",
					Key:               key1,
					Cert:              crt1,
					RequireClientCert: true,
					ClientCAs:         caCrt,
				},
			}
			node1.ListenerCfgs = map[listenerName]ListenerCfg{
				listenerName(proto): newListenerCfg(proto, "server", 1, nil),
			}
			m.GetNodes()[node1.GetID()] = node1

			node2 := m.NewLibNode("node2")
			node2.Connections = []Connection{
				{RemoteNode: node1, Protocol: proto, TLS: "client"},
			}
			node2.TLSServerConfigs = []*netceptor.TLSServerConfig{
				{
					Name:              "server",
					Key:               key2,
					Cert:              crt2,
					RequireClientCert: true,
					ClientCAs:         caCrt,
				},
			}
			node2.TLSClientConfigs = []*netceptor.TLSClientConfig{
				{
					Name:    "client",
					Key:     key2,
					Cert:    crt2,
					RootCAs: caCrt,
				},
			}
			node2.ListenerCfgs = map[listenerName]ListenerCfg{
				listenerName(proto): newListenerCfg(proto, "server", 1, nil),
			}
			m.GetNodes()[node2.GetID()] = node2

			node3 := m.NewLibNode("node3")
			node3.Connections = []Connection{
				{RemoteNode: node2, Protocol: proto, TLS: "client"},
			}
			node3.TLSClientConfigs = []*netceptor.TLSClientConfig{
				{
					Name:    "client",
					Key:     key3,
					Cert:    crt3,
					RootCAs: caCrt,
				},
			}
			m.GetNodes()[node3.GetID()] = node3

			err = m.Start(t.Name())
			if err != nil {
				t.Fatal(err)
			}
			defer m.WaitForShutdown()
			defer m.Destroy()

			ctx1, cancel1 := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel1()

			err = m.WaitForReady(ctx1)
			if err != nil {
				t.Fatal(err)
			}

			// Test that each Node can ping each Node
			for _, nodeSender := range m.GetNodes() {
				controller := NewReceptorControl()
				err = controller.Connect(nodeSender.GetControlSocket())
				if err != nil {
					t.Fatal(err)
				}
				for nodeIDResponder := range m.GetNodes() {
					response, err := controller.Ping(nodeIDResponder)
					if err != nil {
						t.Error(err)
					}
					t.Logf("%v", response)
				}
				controller.Close()
			}
		})
	}
}

func TestTCPSSLClientAuthFailNoKey(t *testing.T) {
	t.Parallel()
	for _, proto := range []string{"tcp", "ws"} {
		proto := proto
		t.Run(proto, func(t *testing.T) {
			t.Parallel()

			_, caCrt, err := utils.GenerateCA("ca", "localhost")
			if err != nil {
				t.Fatal(err)
			}
			key1, crt1, err := utils.GenerateCert("node1", "localhost", []string{"localhost"}, []string{"node1"})
			if err != nil {
				t.Fatal(err)
			}

			m := NewLibMesh()

			defer func() {
				t.Log(m.LogWriter.String())
			}()

			node1 := m.NewLibNode("node1")
			node1.TLSServerConfigs = []*netceptor.TLSServerConfig{
				{
					Name:              "cert1",
					Key:               key1,
					Cert:              crt1,
					RequireClientCert: true,
					ClientCAs:         caCrt,
				},
			}
			node1.ListenerCfgs = map[listenerName]ListenerCfg{
				listenerName(proto): newListenerCfg(proto, "cert1", 1, nil),
			}
			m.GetNodes()[node1.GetID()] = node1

			node2 := m.NewLibNode("node2")
			node2.Connections = []Connection{
				{RemoteNode: node1, Protocol: proto, TLS: "client-insecure"},
			}
			node2.TLSClientConfigs = []*netceptor.TLSClientConfig{
				{
					Name: "client-insecure",
					Key:  "",
					Cert: "",
				},
			}
			m.GetNodes()[node2.GetID()] = node2

			err = m.Start(t.Name())
			if err != nil {
				t.Fatal(err)
			}

			defer m.WaitForShutdown()
			defer m.Destroy()

			ctx1, cancel1 := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel1()

			sleepInterval := 100 * time.Millisecond
			if !utils.CheckUntilTimeout(ctx1, sleepInterval, func() bool {
				linuxTLSError := strings.Contains(m.LogWriter.String(), "certificate signed by unknown authority")
				macTLSError := strings.Contains(m.LogWriter.String(), "certificate is not trusted")

				return linuxTLSError || macTLSError
			}) {
				t.Fatal("Expected connection to fail but it succeeded")
			}
		})
	}
}

func TestTCPSSLClientAuthFailBadKey(t *testing.T) {
	t.Parallel()

	for _, proto := range []string{"tcp", "ws"} {
		proto := proto

		t.Run(proto, func(t *testing.T) {
			t.Parallel()

			_, caCrt, err := utils.GenerateCA("ca", "localhost")
			if err != nil {
				t.Fatal(err)
			}
			key1, crt1, err := utils.GenerateCert("node1", "localhost", []string{"localhost"}, []string{"node1"})
			if err != nil {
				t.Fatal(err)
			}

			key2, crt2, err := utils.GenerateCert("node2", "localhost", []string{"localhost"}, []string{"node2"})
			if err != nil {
				t.Fatal(err)
			}

			m := NewLibMesh()

			defer func() {
				t.Log(m.LogWriter.String())
			}()

			node1 := m.NewLibNode("node1")
			node1.TLSServerConfigs = []*netceptor.TLSServerConfig{
				{
					Name:              "cert1",
					Key:               key1,
					Cert:              crt1,
					RequireClientCert: true,
					ClientCAs:         caCrt,
				},
			}
			node1.ListenerCfgs = map[listenerName]ListenerCfg{
				listenerName(proto): newListenerCfg(proto, "cert1", 1, nil),
			}
			m.GetNodes()[node1.GetID()] = node1

			node2 := m.NewLibNode("node2")
			node2.Connections = []Connection{
				{RemoteNode: node1, Protocol: proto, TLS: "client-insecure"},
			}
			node2.TLSClientConfigs = []*netceptor.TLSClientConfig{
				{
					Name: "client-insecure",
					Key:  key2,
					Cert: crt2,
				},
			}
			m.GetNodes()[node2.GetID()] = node2

			err = m.Start(t.Name())
			if err != nil {
				t.Fatal(err)
			}
			defer m.WaitForShutdown()
			defer m.Destroy()

			ctx1, cancel1 := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel1()

			sleepInterval := 100 * time.Millisecond
			if !utils.CheckUntilTimeout(ctx1, sleepInterval, func() bool {
				linuxTLSError := strings.Contains(m.LogWriter.String(), "certificate signed by unknown authority")
				macTLSError := strings.Contains(m.LogWriter.String(), "certificate is not trusted")

				return linuxTLSError || macTLSError
			}) {
				t.Fatal("Expected connection to fail but it succeeded")
			}
		})
	}
}

func TestTCPSSLServerAuthFailNoKey(t *testing.T) {
	t.Parallel()
	for _, proto := range []string{"tcp", "ws"} {
		proto := proto
		t.Run(proto, func(t *testing.T) {
			t.Parallel()

			_, caCrt, err := utils.GenerateCA("ca", "localhost")
			if err != nil {
				t.Fatal(err)
			}

			key1, crt1, err := utils.GenerateCert("node2", "localhost", []string{"localhost"}, []string{"node2"})
			if err != nil {
				t.Fatal(err)
			}

			m := NewLibMesh()

			defer func() {
				t.Log(m.LogWriter.String())
			}()

			node1 := m.NewLibNode("node1")
			node1.ListenerCfgs = map[listenerName]ListenerCfg{
				listenerName(proto): newListenerCfg(proto, "", 1, nil),
			}
			m.GetNodes()[node1.GetID()] = node1

			node2 := m.NewLibNode("node2")
			node2.Connections = []Connection{
				{RemoteNode: node1, Protocol: proto, TLS: "client"},
			}
			node2.TLSClientConfigs = []*netceptor.TLSClientConfig{
				{
					Name:    "client",
					Key:     key1,
					Cert:    crt1,
					RootCAs: caCrt,
				},
			}
			m.GetNodes()[node2.GetID()] = node2

			err = m.Start(t.Name())
			if err != nil {
				t.Fatal(err)
			}

			defer m.WaitForShutdown()
			defer m.Destroy()

			ctx1, cancel1 := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel1()

			sleepInterval := 100 * time.Millisecond
			if !utils.CheckUntilTimeout(ctx1, sleepInterval, func() bool {
				return strings.Contains(m.LogWriter.String(), "first record does not look like a TLS handshake")
			}) {
				t.Fatal("Expected connection to fail but it succeeded")
			}
		})
	}
}

func TestTCPSSLServerAuthFailBadKey(t *testing.T) {
	t.Parallel()
	for _, proto := range []string{"tcp", "ws"} {
		proto := proto
		t.Run(proto, func(t *testing.T) {
			t.Parallel()

			_, caCrt, err := utils.GenerateCA("ca", "localhost")
			if err != nil {
				t.Fatal(err)
			}
			key1, crt1, err := utils.GenerateCert("node1", "localhost", []string{"localhost"}, []string{"node1"})
			if err != nil {
				t.Fatal(err)
			}

			key2, crt2, err := utils.GenerateCert("node2", "localhost", []string{"localhost"}, []string{"node2"})
			if err != nil {
				t.Fatal(err)
			}
			m := NewLibMesh()

			defer func() {
				t.Log(m.LogWriter.String())
			}()

			node1 := m.NewLibNode("node1")
			node1.TLSServerConfigs = []*netceptor.TLSServerConfig{
				{
					Name:              "cert1",
					Key:               key1,
					Cert:              crt1,
					RequireClientCert: true,
					ClientCAs:         caCrt,
				},
			}
			node1.ListenerCfgs = map[listenerName]ListenerCfg{
				listenerName(proto): newListenerCfg(proto, "cert1", 1, nil),
			}
			m.GetNodes()[node1.GetID()] = node1

			node2 := m.NewLibNode("node2")
			node2.Connections = []Connection{
				{RemoteNode: node1, Protocol: proto, TLS: "client-insecure"},
			}
			node2.TLSClientConfigs = []*netceptor.TLSClientConfig{
				{
					Name: "client-insecure",
					Key:  key2,
					Cert: crt2,
				},
			}
			m.GetNodes()[node2.GetID()] = node2

			err = m.Start(t.Name())
			if err != nil {
				t.Fatal(err)
			}
			defer m.WaitForShutdown()
			defer m.Destroy()

			ctx1, cancel1 := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel1()

			sleepInterval := 100 * time.Millisecond
			if !utils.CheckUntilTimeout(ctx1, sleepInterval, func() bool {
				linuxTLSError := strings.Contains(m.LogWriter.String(), "certificate signed by unknown authority")
				macTLSError := strings.Contains(m.LogWriter.String(), "certificate is not trusted")

				return linuxTLSError || macTLSError
			}) {
				t.Fatal("Expected connection to fail but it succeeded")
			}
		})
	}
}
