import os
import tempfile

import pytest
import yaml

from receptorctl.socket_interface import ReceptorControl


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


class TestReceptorCtlConfig:
    @pytest.mark.parametrize(
        "config_data,expected",
        [
            pytest.param(
                {
                    "name": "happy-path",
                    "rootcas": "/path/to/rootcas.crt",
                    "key": "/path/to/key.pem",
                    "cert": "/path/to/cert.pem",
                    "insecureskipverify": True,
                },
                {
                    "_rootcas": "/path/to/rootcas.crt",
                    "_key": "/path/to/key.pem",
                    "_cert": "/path/to/cert.pem",
                    "_insecureskipverify": True,
                },
                id="happy-path",
            ),
            pytest.param(
                {"name": "only-root-ca", "rootcas": "/path/to/rootcas.crt"},
                {
                    "_rootcas": "/path/to/rootcas.crt",
                    "_key": None,
                    "_cert": None,
                    "_insecureskipverify": False,
                },
                id="only-root-ca",
            ),
            pytest.param(
                {"name": "only-client-cert", "cert": "/path/to/cert.pem"},
                {
                    "_rootcas": None,
                    "_key": None,
                    "_cert": "/path/to/cert.pem",
                    "_insecureskipverify": False,
                },
                id="only-client-cert",
            ),
            pytest.param(
                {},
                {"_rootcas": None, "_key": None, "_cert": None, "_insecureskipverify": False},
                id="empty-config-data",
            ),
        ],
    )
    def test_readconfig(self, config_data, expected):
        """Test readconfig with various configuration scenarios"""
        with tempfile.NamedTemporaryFile(mode="w", suffix=".yaml", delete=False) as f:
            yaml_data = [{"tls-client": config_data}] if config_data else []
            yaml.dump(yaml_data, f)
            config_file = f.name
        controller = ReceptorControl("unix:///tmp/test.sock")
        controller.readconfig(config_file, config_data.get("name", None))

        try:
            for key, value in expected.items():
                attr = getattr(controller, key)
                assert attr == value, f"Expected {key}={value}, got {attr}"
        finally:
            if os.path.exists(config_file):
                os.unlink(config_file)

    def test_readconfig_file_not_found(self):
        """Test readconfig with non-existent file"""
        controller = ReceptorControl("unix:///tmp/test.sock")
        with pytest.raises(FileNotFoundError):
            controller.readconfig("/nonexistent/path/config.yaml", "test-client")

    def test_readconfig_malformed_yaml(self):
        """Test readconfig with malformed YAML"""
        with tempfile.NamedTemporaryFile(mode="w", suffix=".yaml", delete=False) as f:
            f.write("this is not: valid: yaml: content:\n  - broken")
            config_file = f.name

        try:
            controller = ReceptorControl("unix:///tmp/test.sock")
            with pytest.raises(yaml.YAMLError):
                controller.readconfig(config_file, "test-client")
        finally:
            os.unlink(config_file)
