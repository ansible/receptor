from receptorctl import cli as commands

# The goal is to write tests following the click documentation:
# https://click.palletsprojects.com/en/8.0.x/testing/

import pytest, json


@pytest.mark.usefixtures("receptor_mesh_mesh1")
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

    def test_cmd_ping(self, invoke):
        result = invoke(commands.ping, ["node2"])
        assert result.exit_code == 0
        assert "Reply from node2 in" in result.output

    @pytest.mark.skip(
        reason="skip code is 0 bug related here https://github.com/ansible/receptor/issues/431"
    )
    def test_cmd_work_missing_subcommand(self, invoke):
        result = invoke(commands.work, [])
        assert result.exit_code != 0
        assert "Usage: cli work [OPTIONS] COMMAND [ARGS]..." in result.output

    @pytest.mark.skip(
        reason="skip code is 0 bug related here https://github.com/ansible/receptor/issues/431"
    )
    @pytest.mark.parametrize(
        "command, error_message",
        [
            ("cancel", "No unit IDs supplied: Not doing anything"),
            ("release", "No unit IDs supplied: Not doing anything"),
            ("results", "Usage: cli work results [OPTIONS] UNIT_ID"),
            ("submit", "Usage: cli work submit [OPTIONS] WORKTYPE [CMDPARAMS]"),
        ],
    )
    def test_cmd_work_missing_param(self, invoke, command, error_message):
        result = invoke(commands.work, [command])
        assert result.exit_code != 0
        assert error_message in result.stdout

    def test_cmd_work_cancel_successfully(self, invoke):
        # Require fixture with a node running work
        pass

    def test_cmd_work_list_empty_work_unit(self, invoke):
        result = invoke(commands.work, ["list"])
        assert result.exit_code == 0
        assert json.loads(result.output) == {}

    def test_cmd_work_list_successfully(self, invoke):
        # Require fixture with a node running work
        pass
