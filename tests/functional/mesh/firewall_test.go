package mesh

import (
	"context"
	"testing"
	"time"

	"github.com/ansible/receptor/pkg/netceptor"
	_ "github.com/fortytw2/leaktest"
)

func TestFirewall(t *testing.T) {
	t.Parallel()

	for _, proto := range []string{"tcp", "ws", "udp"} {
		proto := proto

		t.Run(proto, func(t *testing.T) {
			t.Parallel()

			m := NewLibMesh()

			defer func() {
				t.Log(m.LogWriter.String())
			}()

			node1 := m.NewLibNode("node1")
			node2 := m.NewLibNode("node2")

			node1.Connections = []Connection{
				{RemoteNode: node2, Protocol: proto},
			}
			m.GetNodes()[node1.GetID()] = node1

			node2.ListenerCfgs = map[listenerName]ListenerCfg{
				listenerName(proto): newListenerCfg(proto, "", 1, nil),
			}
			m.GetNodes()[node2.GetID()] = node2
			node2.Config.FirewallRules = []netceptor.FirewallRuleData{
				{"Action": "accept", "FromNode": "node2"},
				{"Action": "reject", "ToNode": "node3"},
				{"Action": "reject", "ToNode": "node1"},
			}

			node3 := m.NewLibNode("node3")
			node3.Connections = []Connection{
				{RemoteNode: node2, Protocol: proto},
			}
			m.GetNodes()[node3.GetID()] = node3

			defer m.WaitForShutdown()
			defer m.Destroy()
			err := m.Start(t.Name())
			if err != nil {
				t.Fatal(err)
			}

			ctx1, cancel1 := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel1()

			err = m.WaitForReady(ctx1)
			if err != nil {
				t.Fatal(err)
			}

			// Test that node1 and node3 can ping node2
			for _, nodeSender := range []*LibNode{m.GetNodes()["node1"], m.GetNodes()["node3"]} {
				controller := NewReceptorControl()
				err = controller.Connect(nodeSender.GetControlSocket())
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
				controller := NewReceptorControl()
				err = controller.Connect(m.GetNodes()[strSender].GetControlSocket())
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
