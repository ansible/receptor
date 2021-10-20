import sys

sys.path.append("../receptorctl")

from receptorctl import cli as commands
import receptorctl

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

    def test_cmd_work_invalid(self, invoke):
        result = invoke(commands.work, ["cancel", "foobar"])
        assert result.exit_code != 0, "The 'work cancel' command should fail, but did not return non-zero exit code"
