package mesh

import (
	"context"
	"testing"
	"time"

	"github.com/ansible/receptor/tests/functional/lib/mesh"
	"github.com/ansible/receptor/tests/functional/lib/receptorcontrol"
	_ "github.com/fortytw2/leaktest"
)

func TestFirewall(t *testing.T) {
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
			// Setup our mesh yaml data
			data := mesh.YamlData{}
			data.Nodes = make(map[string]*mesh.YamlNode)

			// Generate a mesh of node1->node2->node3 with firewall rules so node2 can talk to node1 and node3,
			// but node1 and node3 can't talk to each other because node2 blocks the traffic
			data.Nodes["node1"] = &mesh.YamlNode{
				Connections: map[string]mesh.YamlConnection{
					"node2": {
						Index: 0,
					},
				},
				Nodedef: []interface{}{
					map[interface{}]interface{}{
						listener: map[interface{}]interface{}{},
					},
				},
			}
			data.Nodes["node2"] = &mesh.YamlNode{
				Nodedef: []interface{}{
					map[interface{}]interface{}{
						listener: map[interface{}]interface{}{},
					},
					map[interface{}]interface{}{
						"node": map[interface{}]interface{}{
							"id": "node2",
							"firewallrules": []map[string]string{
								{"Action": "accept", "FromNode": "node2"},
								{"Action": "reject", "ToNode": "node3"},
								{"Action": "reject", "ToNode": "node1"},
							},
						},
					},
				},
			}
			data.Nodes["node3"] = &mesh.YamlNode{
				Connections: map[string]mesh.YamlConnection{
					"node2": {
						Index: 0,
					},
				},
				Nodedef: []interface{}{
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

			// Test that node1 and node3 can ping node2
			for _, nodeSender := range []mesh.Node{m.Nodes()["node1"], m.Nodes()["node3"]} {
				controller := receptorcontrol.New()
				err = controller.Connect(nodeSender.ControlSocket())
				if err != nil {
					t.Fatal(err)
				}
				response, err := controller.Ping("node2")
				if err != nil {
					t.Error(err)
				}
				t.Logf("%v", response)
				controller.Close()
			}

			// Test that node1 and node3 cannot ping each other
			for strSender, strReceiver := range map[string]string{"node1": "node3", "node3": "node1"} {
				controller := receptorcontrol.New()
				err = controller.Connect(m.Nodes()[strSender].ControlSocket())
				if err != nil {
					t.Fatal(err)
				}
				_, err := controller.Ping(strReceiver)
				if err == nil {
					t.Error("firewall failed to block ping")
				} else if err.Error() != "blocked by firewall" {
					t.Errorf("got wrong error: %s", err)
				}

				t.Logf("%v", err)
				controller.Close()
			}
		})
	}
}
