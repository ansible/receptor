package mesh

import (
	"context"
	"testing"
	"time"

	"github.com/ansible/receptor/tests/functional/lib/mesh"
	"github.com/ansible/receptor/tests/functional/lib/receptorcontrol"
	"github.com/ansible/receptor/tests/functional/lib/utils"
	_ "github.com/fortytw2/leaktest"
)

func TestTCPSSLConnections(t *testing.T) {
	t.Parallel()
	testTable := []struct {
		listener string
	}{
		{"tcp-listener"},
		{"ws-listener"},
	}
	for _, data := range testTable {
		listener := data.listener
		t.Run(listener, func(t *testing.T) {
			t.Parallel()
			caKey, caCrt, err := utils.GenerateCA("ca", "localhost")
			if err != nil {
				t.Fatal(err)
			}
			key1, crt1, err := utils.GenerateCertWithCA("node1", caKey, caCrt, "localhost", []string{"localhost"}, nil)
			if err != nil {
				t.Fatal(err)
			}
			key2, crt2, err := utils.GenerateCertWithCA("node2", caKey, caCrt, "localhost", []string{"localhost"}, nil)
			if err != nil {
				t.Fatal(err)
			}

			// Setup our mesh yaml data
			data := mesh.YamlData{}
			data.Nodes = make(map[string]*mesh.YamlNode)

			// Generate a mesh where each node n is connected to only n+1 and n-1
			// if they exist
			data.Nodes["node1"] = &mesh.YamlNode{
				Connections: map[string]mesh.YamlConnection{},
				NodedefBase: []interface{}{
					map[interface{}]interface{}{
						"tls-server": map[interface{}]interface{}{
							"name":              "cert1",
							"key":               key1,
							"cert":              crt1,
							"requireclientcert": true,
							"clientcas":         caCrt,
						},
					},
					map[interface{}]interface{}{
						listener: map[interface{}]interface{}{
							"tls": "cert1",
						},
					},
				},
			}
			data.Nodes["node2"] = &mesh.YamlNode{
				Connections: map[string]mesh.YamlConnection{
					"node1": {
						Index: 1,
						TLS:   "client-cert2",
					},
				},
				NodedefBase: []interface{}{
					map[interface{}]interface{}{
						"tls-server": map[interface{}]interface{}{
							"name": "server-cert2",
							"key":  key2,
							"cert": crt2,
						},
					},
					map[interface{}]interface{}{
						"tls-client": map[interface{}]interface{}{
							"name":    "client-cert2",
							"key":     key2,
							"cert":    crt2,
							"rootcas": caCrt,
						},
					},
					map[interface{}]interface{}{
						listener: map[interface{}]interface{}{
							"tls": "server-cert2",
						},
					},
				},
			}
			data.Nodes["node3"] = &mesh.YamlNode{
				Connections: map[string]mesh.YamlConnection{
					"node2": {
						Index: 2,
						TLS:   "client-secure",
					},
				},
				NodedefBase: []interface{}{
					map[interface{}]interface{}{
						"tls-client": map[interface{}]interface{}{
							"name":    "client-secure",
							"key":     "",
							"cert":    "",
							"rootcas": caCrt,
						},
					},
					map[interface{}]interface{}{
						listener: map[interface{}]interface{}{},
					},
				},
			}
			m, err := mesh.NewCLIMeshFromYaml(data, t.Name())
			if err != nil {
				t.Fatal(err)
			}
			defer m.WaitForShutdown()
			defer m.Destroy()

			ctx, _ := context.WithTimeout(context.Background(), 20*time.Second)
			err = m.WaitForReady(ctx)
			if err != nil {
				t.Fatal(err)
			}
			// Test that each Node can ping each Node
			for _, nodeSender := range m.Nodes() {
				controller := receptorcontrol.New()
				err = controller.Connect(nodeSender.ControlSocket())
				if err != nil {
					t.Fatal(err)
				}
				for nodeIDResponder := range m.Nodes() {
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
	testTable := []struct {
		listener string
	}{
		{"tcp-listener"},
		{"ws-listener"},
	}
	for _, data := range testTable {
		listener := data.listener
		t.Run(listener, func(t *testing.T) {
			t.Parallel()

			_, caCrt, err := utils.GenerateCA("ca", "localhost")
			if err != nil {
				t.Fatal(err)
			}
			key1, crt1, err := utils.GenerateCert("node1", "localhost", []string{"localhost"}, nil)
			if err != nil {
				t.Fatal(err)
			}

			// Setup our mesh yaml data
			data := mesh.YamlData{}
			data.Nodes = make(map[string]*mesh.YamlNode)

			// Generate a mesh where each node n is connected to only n+1 and n-1
			// if they exist
			data.Nodes["node1"] = &mesh.YamlNode{
				Connections: map[string]mesh.YamlConnection{},
				NodedefBase: []interface{}{
					map[interface{}]interface{}{
						"tls-server": map[interface{}]interface{}{
							"name":              "cert1",
							"key":               key1,
							"cert":              crt1,
							"requireclientcert": true,
							"clientcas":         caCrt,
						},
					},
					map[interface{}]interface{}{
						listener: map[interface{}]interface{}{
							"tls": "cert1",
						},
					},
				},
			}
			data.Nodes["node2"] = &mesh.YamlNode{
				Connections: map[string]mesh.YamlConnection{
					"node1": {
						Index: 1,
						TLS:   "client-insecure",
					},
				},
				NodedefBase: []interface{}{
					map[interface{}]interface{}{
						"tls-client": map[interface{}]interface{}{
							"name":               "client-insecure",
							"key":                "",
							"cert":               "",
							"insecureskipverify": true,
						},
					},
					map[interface{}]interface{}{
						listener: map[interface{}]interface{}{},
					},
				},
			}
			m, err := mesh.NewCLIMeshFromYaml(data, t.Name())
			if err != nil {
				t.Fatal(err)
			}
			defer m.WaitForShutdown()
			defer m.Destroy()

			ctx, _ := context.WithTimeout(context.Background(), 20*time.Second)
			err = m.WaitForReady(ctx)
			if err == nil {
				t.Fatal("Receptor client auth was expected to fail but it succeeded")
			}
		})
	}
}

func TestTCPSSLClientAuthFailBadKey(t *testing.T) {
	t.Parallel()
	testTable := []struct {
		listener string
	}{
		{"tcp-listener"},
		{"ws-listener"},
	}
	for _, data := range testTable {
		listener := data.listener
		t.Run(listener, func(t *testing.T) {
			t.Parallel()

			_, caCrt, err := utils.GenerateCA("ca", "localhost")
			if err != nil {
				t.Fatal(err)
			}
			key1, crt1, err := utils.GenerateCert("node1", "localhost", []string{"localhost"}, nil)
			if err != nil {
				t.Fatal(err)
			}

			key2, crt2, err := utils.GenerateCert("node2", "localhost", []string{"localhost"}, nil)
			if err != nil {
				t.Fatal(err)
			}

			// Setup our mesh yaml data
			data := mesh.YamlData{}
			data.Nodes = make(map[string]*mesh.YamlNode)

			// Generate a mesh where each node n is connected to only n+1 and n-1
			// if they exist
			data.Nodes["node1"] = &mesh.YamlNode{
				Connections: map[string]mesh.YamlConnection{},
				NodedefBase: []interface{}{
					map[interface{}]interface{}{
						"tls-server": map[interface{}]interface{}{
							"name":              "cert1",
							"key":               key1,
							"cert":              crt1,
							"requireclientcert": true,
							"clientcas":         caCrt,
						},
					},
					map[interface{}]interface{}{
						listener: map[interface{}]interface{}{
							"tls": "cert1",
						},
					},
				},
			}
			data.Nodes["node2"] = &mesh.YamlNode{
				Connections: map[string]mesh.YamlConnection{
					"node1": {
						Index: 1,
						TLS:   "client-insecure",
					},
				},
				NodedefBase: []interface{}{
					map[interface{}]interface{}{
						"tls-client": map[interface{}]interface{}{
							"name":               "client-insecure",
							"key":                key2,
							"cert":               crt2,
							"insecureskipverify": true,
						},
					},
					map[interface{}]interface{}{
						listener: map[interface{}]interface{}{},
					},
				},
			}
			m, err := mesh.NewCLIMeshFromYaml(data, t.Name())
			if err != nil {
				t.Fatal(err)
			}
			defer m.WaitForShutdown()
			defer m.Destroy()

			ctx, _ := context.WithTimeout(context.Background(), 20*time.Second)
			err = m.WaitForReady(ctx)
			if err == nil {
				t.Fatal("Receptor client auth was expected to fail but it succeeded")
			}
		})
	}
}

func TestTCPSSLServerAuthFailNoKey(t *testing.T) {
	t.Parallel()
	testTable := []struct {
		listener string
	}{
		{"tcp-listener"},
		{"ws-listener"},
	}
	for _, data := range testTable {
		listener := data.listener
		t.Run(listener, func(t *testing.T) {
			t.Parallel()

			_, caCrt, err := utils.GenerateCA("ca", "localhost")
			if err != nil {
				t.Fatal(err)
			}
			key1, crt1, err := utils.GenerateCert("node1", "localhost", []string{"localhost"}, nil)
			if err != nil {
				t.Fatal(err)
			}

			// Setup our mesh yaml data
			data := mesh.YamlData{}
			data.Nodes = make(map[string]*mesh.YamlNode)

			// Generate a mesh where each node n is connected to only n+1 and n-1
			// if they exist
			data.Nodes["node1"] = &mesh.YamlNode{
				Connections: map[string]mesh.YamlConnection{},
				NodedefBase: []interface{}{
					map[interface{}]interface{}{
						listener: map[interface{}]interface{}{},
					},
				},
			}
			data.Nodes["node2"] = &mesh.YamlNode{
				Connections: map[string]mesh.YamlConnection{
					"node1": {
						Index: 1,
						TLS:   "client-secure",
					},
				},
				NodedefBase: []interface{}{
					map[interface{}]interface{}{
						"tls-client": map[interface{}]interface{}{
							"name":    "client-secure",
							"key":     key1,
							"cert":    crt1,
							"rootcas": caCrt,
						},
					},
					map[interface{}]interface{}{
						listener: map[interface{}]interface{}{},
					},
				},
			}
			m, err := mesh.NewCLIMeshFromYaml(data, t.Name())
			if err != nil {
				t.Fatal(err)
			}
			defer m.WaitForShutdown()
			defer m.Destroy()

			ctx, _ := context.WithTimeout(context.Background(), 20*time.Second)
			err = m.WaitForReady(ctx)
			if err == nil {
				t.Fatal("Receptor server auth was expected to fail but it succeeded")
			}
		})
	}
}

func TestTCPSSLServerAuthFailBadKey(t *testing.T) {
	t.Parallel()
	testTable := []struct {
		listener string
	}{
		{"tcp-listener"},
		{"ws-listener"},
	}
	for _, data := range testTable {
		listener := data.listener
		t.Run(listener, func(t *testing.T) {
			t.Parallel()

			_, caCrt, err := utils.GenerateCA("ca", "localhost")
			if err != nil {
				t.Fatal(err)
			}
			key1, crt1, err := utils.GenerateCert("node1", "localhost", []string{"localhost"}, nil)
			if err != nil {
				t.Fatal(err)
			}

			key2, crt2, err := utils.GenerateCert("node2", "localhost", []string{"localhost"}, nil)
			if err != nil {
				t.Fatal(err)
			}

			// Setup our mesh yaml data
			data := mesh.YamlData{}
			data.Nodes = make(map[string]*mesh.YamlNode)

			// Generate a mesh where each node n is connected to only n+1 and n-1
			// if they exist
			data.Nodes["node1"] = &mesh.YamlNode{
				Connections: map[string]mesh.YamlConnection{},
				NodedefBase: []interface{}{
					map[interface{}]interface{}{
						"tls-server": map[interface{}]interface{}{
							"name": "cert1",
							"key":  key1,
							"cert": crt1,
						},
					},
					map[interface{}]interface{}{
						listener: map[interface{}]interface{}{
							"tls": "cert1",
						},
					},
				},
			}
			data.Nodes["node2"] = &mesh.YamlNode{
				Connections: map[string]mesh.YamlConnection{
					"node1": {
						Index: 1,
						TLS:   "client-secure",
					},
				},
				NodedefBase: []interface{}{
					map[interface{}]interface{}{
						"tls-client": map[interface{}]interface{}{
							"name":    "client-secure",
							"key":     key2,
							"cert":    crt2,
							"rootcas": caCrt,
						},
					},
					map[interface{}]interface{}{
						listener: map[interface{}]interface{}{},
					},
				},
			}
			m, err := mesh.NewCLIMeshFromYaml(data, t.Name())
			if err != nil {
				t.Fatal(err)
			}
			defer m.WaitForShutdown()
			defer m.Destroy()

			ctx, _ := context.WithTimeout(context.Background(), 20*time.Second)
			err = m.WaitForReady(ctx)
			if err == nil {
				t.Fatal("Receptor server auth was expected to fail but it succeeded")
			}
		})
	}
}
