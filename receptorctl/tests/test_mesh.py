from receptorctl import cli as commands

# The goal is to write tests following the click documentation:
# https://click.palletsprojects.com/en/8.0.x/testing/

import pytest
import time


@pytest.mark.usefixtures("receptor_mesh_access_control")
class TestMeshFirewall:
    def test_work_unsigned(self, invoke, receptor_nodes):
        """Run a unsigned work-command

        Steps:
        1. Create node1 with a unsigned work-command
        2. Create node2
        3. Run from node2 a unsigned work-command to node1
        4. Expect to be accepted
        """

        # Run an unsigned command
        result = invoke(
            commands.work,
            "submit unsigned-echo --node node1 --no-payload".split(),
        )
        work_unit_id = result.stdout.split("Unit ID: ")[-1].replace("\n", "")

        time.sleep(5)
        assert result.exit_code == 0

        # Release unsigned work
        result = invoke(commands.work, f"release {work_unit_id}".split())
        time.sleep(5)

        assert result.exit_code == 0

    # DISABLE UNTIL THE FIX BEING IMPLEMENTED
    #
    # def test_work_signed_expect_block(self, invoke, receptor_nodes):
    #     """Run a signed work-command without the right key
    #     and expect to be blocked.

    #     Steps:
    #     1. Create node1 with a signed work-command
    #     2. Create node2
    #     3. Run from node2 a signed work-command to node1
    #     4. Expect to be blocked
    #     """
    #     # Run an unsigned command
    #     result = invoke(
    #         commands.work, "submit signed-echo --node node1 --no-payload".split()
    #     )
    #     work_unit_id = result.stdout.split("Unit ID: ")[-1].replace("\n", "")

    #     time.sleep(5)
    #     assert work_unit_id, "Work unit ID should not be empty"
    #     assert result.exit_code != 0, "Work signed run should fail, but it worked"

    #     # Release unsigned work
    #     result = invoke(commands.work, f"release {work_unit_id}".split())
    #     assert result.exit_code == 0, "Work release failed"
