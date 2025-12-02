import json
import re
import time

import pytest

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
        assert "Reply from node2 in" in result.output

    def test_cmd_traceroute(self, invoke):
        """Test traceroute command to a valid node"""
        result = invoke(commands.traceroute, ["node2"])
        assert result.exit_code == 0

        # Verify output format: "hop_number: NodeName in TimeStr"
        # Example: "0: node1 in 200.323µs", "1: node2 in 490.723µs"
        lines = result.output.strip().split("\n")
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
        lines = result.output.strip().split("\n")
        assert len(lines) == 1, "Traceroute should produce a line for each node"
        assert result.exit_code == 0
        assert "ERROR: 1: Error no route to node from node1 in " in str(result.stderr_bytes)

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

    def test_cmd_work_results_invalid_unit_id(self, invoke):
        """Test results command with an invalid work unit ID"""
        result = invoke(commands.work, ["results", "invalid-unit-id"])
        assert result.exit_code != 0
        assert result.exception is not None
        assert "invalid-unit-id" in str(result.exception)

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
        assert payload.upper() in result.output

        node1_controller.close()

    def test_cmd_work_invalid(self, invoke):
        result = invoke(commands.work, ["cancel", "foobar"])
        assert result.exit_code != 0, (
            "The 'work cancel' command should fail, but did not return non-zero exit code"
        )
