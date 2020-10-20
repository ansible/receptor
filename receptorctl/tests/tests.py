import pytest
import subprocess
from receptorctl import ReceptorControl
import time

@pytest.fixture(scope="class")
def receptor_mesh(request):
    node1 = subprocess.Popen(["receptor", "-c", "tests/mesh-definitions/mesh1/node1.yaml"])
    node2 = subprocess.Popen(["receptor", "-c", "tests/mesh-definitions/mesh1/node2.yaml"])
    time.sleep(0.5)
    node1_controller = ReceptorControl()
    node1_controller.connect("unix:///tmp/node1.sock")

    while True:
        status = node1_controller.simple_command("status")
        if status["RoutingTable"] == {"node2":"node2"}:
            break

    node1_controller.close()
    yield

    node1.kill()
    node2.kill()
    node1.wait()
    node2.wait()

@pytest.mark.usefixtures('receptor_mesh')
class TestReceptorCTL:
    def test_simple_command(self):
        node1_controller = ReceptorControl()
        node1_controller.connect("unix:///tmp/node1.sock")
        status = node1_controller.simple_command("status")
        node1_controller.close()
        assert not (set(["Advertisements", "Connections", "KnownConnectionCosts", "NodeID", "RoutingTable"]) - status.keys())

    def test_simple_command_fail(self):
        node1_controller = ReceptorControl()
        node1_controller.connect("unix:///tmp/node1.sock")
        with pytest.raises(RuntimeError):
            node1_controller.simple_command("doesnotexist")
        node1_controller.close()

    def test_tcp_control_service(self):
        node1_controller = ReceptorControl()
        node1_controller.connect("tcp://localhost:11112")
        status = node1_controller.simple_command("status")
        node1_controller.close()
        assert not (set(["Advertisements", "Connections", "KnownConnectionCosts", "NodeID", "RoutingTable"]) - status.keys())

    def test_connect_to_service(self):
        node1_controller = ReceptorControl()
        node1_controller.connect("unix:///tmp/node1.sock")
        node1_controller.connect_to_service("node2", "control", "")
        node1_controller.handshake()
        status = node1_controller.simple_command("status")
        node1_controller.close()
        assert status["NodeID"] == "node2"
