import json
import re
import time
from io import BytesIO

import pytest
from click.testing import CliRunner

from receptorctl import cli as commands

# The goal is to write tests following the click documentation:
# https://click.palletsprojects.com/en/8.0.x/testing/


@pytest.mark.usefixtures("receptor_mesh_mesh1")
class TestCLI:
    def test_cli_cmd_status(self, invoke_as_json):
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
        ) == set(json_output.keys()), "The command returned unexpected keys from json output"

    def test_cmd_ping(self, invoke):
        result = invoke(commands.ping, ["node2"])
        assert result.exit_code == 0
        assert "Reply from node2 in" in result.stdout

    def test_cmd_traceroute(self, invoke):
        """Test traceroute command to a valid node"""
        result = invoke(commands.traceroute, ["node2"])
        assert result.exit_code == 0

        # Verify output format: "hop_number: NodeName in TimeStr"
        # Example: "0: node1 in 200.323µs", "1: node2 in 490.723µs"
        lines = result.stdout.strip().split("\n")
        assert len(lines) == 2, "Traceroute should produce a line for each node"

        # Regex pattern: hop_number: node_name in time_value(µs|ms|ns|s)
        pattern = r"^\d+: \S+ in [\d.]+(?:µs|ms|ns|s)$"

        for i, line in enumerate(lines):
            assert re.match(pattern, line), f"Line '{line}' does not match expected format"
            # Verify hop number matches line index
            hop_number = int(line.split(":")[0])
            assert hop_number == i, f"Expected hop {i}, got {hop_number}"

        # Verify the destination node appears in the last line
        assert "node2" in lines[-1]

    def test_cmd_traceroute_invalid_node(self, invoke):
        """Test traceroute command to a non-existent node"""
        result = invoke(commands.traceroute, ["nonexistent-node"])
        lines = [
            line for line in result.stderr.strip().split("\n") if not line.startswith("Warning:")
        ]
        assert len(lines) == 1, "Traceroute should produce a line for each node"
        assert result.exit_code == 0
        assert "ERROR: 1: Error no route to node from node1 in " in result.stderr

    @pytest.mark.skip(
        reason="skip code is 0 bug related here https://github.com/ansible/receptor/issues/431"
    )
    def test_cmd_work_missing_subcommand(self, invoke):
        result = invoke(commands.work, [])
        assert result.exit_code != 0
        assert "Usage: cli work [OPTIONS] COMMAND [ARGS]..." in result.stdout

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
        assert json.loads(result.stdout) == {}

    def test_cmd_work_list_successfully(self, invoke):
        # Require fixture with a node running work
        pass

    def test_cmd_work_results_invalid_unit_id(self, invoke):
        """Test results command with an invalid work unit ID"""
        result = invoke(commands.work, ["results", "invalid-unit-id"])
        assert result.exit_code != 0
        assert result.exception is not None
        assert "invalid-unit-id" in str(result.exception)

    @pytest.mark.parametrize(
        "args",
        [
            ["submit", "sleep", "--no-payload", "--rm"],
            ["adopt", "--node", "node3", "unit123", "--rm"],
        ],
    )
    def test_cmd_work_rm_requires_follow_before_control_request(self, invoke, monkeypatch, args):
        get_rc_calls = []
        monkeypatch.setattr(commands, "get_rc", lambda ctx: get_rc_calls.append(ctx))

        result = invoke(commands.work, args)

        assert result.exit_code == 1
        assert "Must use --rm with --follow." in result.stderr
        assert get_rc_calls == []

    def test_cmd_work_results_successful(self, invoke, default_receptor_controller_socket_file):
        node1_controller = default_receptor_controller_socket_file

        # Submit a simple echo work unit
        payload = "test-output-data"
        work = node1_controller.submit_work("echo-uppercase", payload, node="node3")
        unit_id = work.pop("unitid")

        # Wait for work to complete
        max_retries = 10
        work_completed = False
        for _ in range(max_retries):
            status = node1_controller.simple_command(f"work status {unit_id}")
            if status.get("StateName") == "Succeeded" and status.get("Detail") == "exit status 0":
                work_completed = True
                break
            time.sleep(1)

        assert work_completed, "Work unit timed out and never finished"

        # Test the CLI results command
        result = invoke(commands.work, ["results", unit_id])
        assert result.exit_code == 0
        assert payload.upper() in result.stdout

        node1_controller.close()

    def test_cmd_work_invalid(self, invoke):
        result = invoke(commands.work, ["cancel", "foobar"])
        assert result.exit_code != 0, (
            "The 'work cancel' command should fail, but did not return non-zero exit code"
        )


class AdoptController:
    def __init__(self, error=None):
        self.error = error
        self.adopt_calls = []
        self.simple_commands = []

    def adopt_work(self, node, unit_id, tlsclient="", signwork=False):
        self.adopt_calls.append((node, unit_id, tlsclient, signwork))
        if self.error:
            raise self.error
        return {"result": "Adopted", "unitid": "local-unit"}

    def get_work_results(self, unit_id, startpos=0):
        return BytesIO(b"adopted output\n")

    def simple_command(self, command):
        self.simple_commands.append(command)
        if command.startswith("work status"):
            return {"State": 2}
        return {"result": "Released"}


def test_adopt_cli_success_prints_result_and_passes_options():
    controller = AdoptController()
    result = CliRunner().invoke(
        commands.adopt,
        ["--node", "node3", "remote-unit", "--tls-client", "client1", "--signwork"],
        obj={"rc": controller},
    )

    assert result.exit_code == 0
    assert "Result: Adopted" in result.stdout
    assert "Unit ID: local-unit" in result.stdout
    assert controller.adopt_calls == [("node3", "remote-unit", "client1", True)]


def test_adopt_cli_follow_and_remove_releases_completed_work():
    controller = AdoptController()
    result = CliRunner().invoke(
        commands.adopt,
        ["--node", "node3", "remote-unit", "--follow", "--rm"],
        obj={"rc": controller},
    )

    assert result.exit_code == 0
    assert "adopted output" in result.stdout
    assert controller.adopt_calls == [("node3", "remote-unit", "", False)]
    assert controller.simple_commands == ["work status local-unit", "work release local-unit"]


def test_adopt_cli_reports_control_request_error():
    controller = AdoptController(error=RuntimeError("controller unavailable"))
    result = CliRunner().invoke(
        commands.adopt,
        ["--node", "node3", "remote-unit"],
        obj={"rc": controller},
    )

    assert result.exit_code == 101
    assert "controller unavailable" in result.stderr
