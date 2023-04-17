# The goal is to write tests following the click documentation:
# https://click.palletsprojects.com/en/8.0.x/testing/

import pytest
import time


@pytest.fixture(scope="function")
def wait_for_workunit_state():
    def _wait_for_workunit_state(
        node_controller,
        unitid: str,
        expected_detail: str = None,
        expected_state_name: str = None,
        timeout_seconds: int = 30,
    ) -> bool:
        """Wait for a workunit to finish

        At least 'expected_detail' or 'expected_state_name' must be specified.

        Args:
            node_controller: The node controller used to create the workunit
            unitid: The unitid of the workunit to wait for
            expected_detail: The expected detail of the workunit
            expected_state_name: The expected state name of the workunit
            timeout_seconds: The number of seconds to wait before timing out

        Returns:
            True if the workunit finished, False if it timed out
        """
        if expected_detail is None and expected_state_name is None:
            raise ValueError(
                "At least 'expected_detail' or 'expected_state_name' must be specified"
            )

        remaining_time = timeout_seconds

        if expected_detail is not None:
            for _ in range(remaining_time):
                status = node_controller.simple_command("work status {}".format(unitid))
                if status["Detail"] == expected_detail:
                    return True
                else:
                    time.sleep(1)
                    remaining_time -= 1

        if remaining_time <= 0:
            return False

        if expected_state_name is not None:
            for _ in range(remaining_time):
                status = node_controller.simple_command("work status {}".format(unitid))
                if status["StateName"] == expected_state_name:
                    return True
                else:
                    time.sleep(1)
                    remaining_time -= 1
        return False

    return _wait_for_workunit_state


@pytest.fixture(scope="function")
def wait_for_work_finished(wait_for_workunit_state):
    def _wait_for_work_finished(
        node_controller, unitid: str, timeout_seconds: int = 30
    ) -> bool:
        """Wait for a workunit to finish

        Args:
            node_controller: The node controller used to create the workunit
            unitid: The unitid of the workunit to wait for
            timeout_seconds: The number of seconds to wait before timing out

        Returns:
            True if the workunit finished, False if it timed out
        """

        return wait_for_workunit_state(
            node_controller,
            unitid,
            expected_detail="exit status 0",
            expected_state_name="Succeeded",
            timeout_seconds=timeout_seconds,
        )

    return _wait_for_work_finished


@pytest.mark.usefixtures("receptor_mesh_mesh1")
class TestWorkUnit:
    def test_workunit_simple(
        self,
        invoke_as_json,
        default_receptor_controller_socket_file,
        wait_for_work_finished,
    ):
        # Spawn a long running command
        node1_controller = default_receptor_controller_socket_file

        wait_for = 5  # in seconds

        payload = "That's a long string example! And there's emoji too! 👾"
        work = node1_controller.submit_work("echo-uppercase", payload, node="node3")
        state_result = work.pop("result")
        state_unitid = work.pop("unitid")

        assert state_result == "Job Started"
        assert wait_for_work_finished(
            node1_controller, state_unitid, wait_for
        ), "Workunit timed out and never finished"

        work_result = (
            node1_controller.get_work_results(state_unitid)
            .read()
            .decode("utf-8")
            .strip()
        )

        assert payload.upper() == work_result, (
            f"Workunit did not report the expected result:\n - payload: {payload}"
            f"\n - work_result: {work_result}"
        )

        node1_controller.close()

    def test_workunit_cmd_cancel(
        self,
        invoke_as_json,
        default_receptor_controller_socket_file,
        wait_for_workunit_state,
    ):
        # Spawn a long running command
        node1_controller = default_receptor_controller_socket_file

        sleep_for = 9999  # in seconds
        wait_for = 15  # in seconds

        work = node1_controller.submit_work("sleep", str(sleep_for), node="node3")
        state_result = work.pop("result")
        state_unitid = work.pop("unitid")
        assert state_result == "Job Started"

        # HACK: Wait for the workunit to start
        # receptor should be able to cancel the workunit with this
        time.sleep(5)

        # Run and check cancel command
        cancel_output = node1_controller.simple_command(f"work cancel {state_unitid}")
        assert cancel_output["cancelled"] == state_unitid

        # Wait workunit detail == 'Cancelled'
        assert wait_for_workunit_state(
            node1_controller,
            state_unitid,
            expected_detail="Canceled",
            timeout_seconds=wait_for,
        ), "Workunit timed out and never finished"

        # Get work list and check for the workunit detail state
        work_list = node1_controller.simple_command("work list")
        assert work_list[state_unitid]["Detail"] == "Canceled"

        node1_controller.close()
