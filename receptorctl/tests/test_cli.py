from receptorctl import cli as commands

# The goal is to write tests following the click documentation:
# https://click.palletsprojects.com/en/8.0.x/testing/

import pytest


@pytest.mark.usefixtures("receptor_mesh")
class TestCommands:
    def test_cmd_status(self, invoke_as_json):
        result, json_output = invoke_as_json(commands.status, [])
        assert result.exit_code == 0
        assert set(
            [
                "Advertisements",
                "Connections",
                "KnownConnectionCosts",
                "NodeID",
                "RoutingTable",
                "SystemCPUCount",
                "SystemMemoryMiB",
                "Version",
            ]
        ) == set(
            json_output.keys()
        ), "The command returned unexpected keys from json output"
