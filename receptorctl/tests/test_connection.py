import pytest


@pytest.mark.usefixtures("receptor_mesh_mesh1")
class TestReceptorCtlConnection:
    def test_connect_to_service(self, default_receptor_controller_unix):
        node1_controller = default_receptor_controller_unix
        node1_controller.connect_to_service("node2", "control", "")
        node1_controller.handshake()
        status = node1_controller.simple_command("status")
        node1_controller.close()
        assert status["NodeID"] == "node2"

    def test_simple_command(self, default_receptor_controller_unix):
        node1_controller = default_receptor_controller_unix
        status = node1_controller.simple_command("status")
        node1_controller.close()
        assert not (
            set(
                [
                    "Advertisements",
                    "Connections",
                    "KnownConnectionCosts",
                    "NodeID",
                    "RoutingTable",
                ]
            )
            - status.keys()
        )

    def test_simple_command_fail(self, default_receptor_controller_unix):
        node1_controller = default_receptor_controller_unix
        with pytest.raises(RuntimeError):
            node1_controller.simple_command("doesnotexist")
        node1_controller.close()

    def test_tcp_control_service(self, default_receptor_controller_tcp):
        node1_controller = default_receptor_controller_tcp
        status = node1_controller.simple_command("status")
        node1_controller.close()
        assert not (
            set(
                [
                    "Advertisements",
                    "Connections",
                    "KnownConnectionCosts",
                    "NodeID",
                    "RoutingTable",
                ]
            )
            - status.keys()
        )

    def test_tcp_control_service_tls(self, default_receptor_controller_tcp_tls):
        node1_controller = default_receptor_controller_tcp_tls
        status = node1_controller.simple_command("status")
        node1_controller.close()
        assert not (
            set(
                [
                    "Advertisements",
                    "Connections",
                    "KnownConnectionCosts",
                    "NodeID",
                    "RoutingTable",
                ]
            )
            - status.keys()
        )
