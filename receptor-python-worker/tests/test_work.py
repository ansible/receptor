"""
Unit tests for WorkPluginRunner and the module-level run() entrypoint.
"""
import json
import queue
import signal
import sys
import threading
from pathlib import Path
from unittest.mock import MagicMock, patch

import pytest

from receptor_python_worker import work
from receptor_python_worker.work import (
    WorkPluginRunner,
    WorkStateFailed,
    WorkStatePending,
    WorkStateRunning,
    WorkStateSucceeded,
)
from receptor_python_worker.plugin_utils import BUFFER_PAYLOAD, BYTES_PAYLOAD, FILE_PAYLOAD


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _make_unitdir(tmp_path, status=None):
    """Create a minimal unitdir with status and stdin files."""
    if status is None:
        status = {"State": WorkStatePending, "Detail": "", "StdoutSize": 0}
    (tmp_path / "status").write_text(json.dumps(status))
    (tmp_path / "stdin").write_bytes(b"hello")
    return tmp_path


def _make_wpr(tmp_path, directive="myplugin:myaction", config=None):
    _make_unitdir(tmp_path)
    return WorkPluginRunner(directive, str(tmp_path), config or {})


# ---------------------------------------------------------------------------
# WorkPluginRunner.__init__
# ---------------------------------------------------------------------------

class TestWorkPluginRunnerInit:
    def test_init_success(self, tmp_path):
        wpr = _make_wpr(tmp_path)
        assert wpr.plugin_namespace == "myplugin"
        assert wpr.plugin_action == "myaction"
        assert wpr.unitdir == str(tmp_path)
        assert wpr.stdout_size == 0
        assert not wpr.shutting_down

    def test_init_stores_config(self, tmp_path):
        cfg = {"key": "value"}
        wpr = _make_wpr(tmp_path, config=cfg)
        assert wpr.config == cfg

    def test_init_missing_status_file(self, tmp_path):
        (tmp_path / "stdin").write_bytes(b"")
        with pytest.raises(ValueError, match="Status file does not exist"):
            WorkPluginRunner("ns:fn", str(tmp_path), {})

    def test_init_missing_stdin_file(self, tmp_path):
        (tmp_path / "status").write_text(json.dumps({"State": 0, "Detail": "", "StdoutSize": 0}))
        with pytest.raises(ValueError, match="Stdin file does not exist"):
            WorkPluginRunner("ns:fn", str(tmp_path), {})

    def test_init_invalid_directive_rejected(self, tmp_path):
        _make_unitdir(tmp_path)
        with pytest.raises(ValueError):
            WorkPluginRunner("../evil:action", str(tmp_path), {})

    def test_init_invalid_unitdir_rejected(self, tmp_path):
        with pytest.raises(ValueError):
            WorkPluginRunner("ns:fn", "relative/path", {})

    def test_init_creates_stdout_file(self, tmp_path):
        _make_wpr(tmp_path)
        assert (tmp_path / "stdout").exists()


# ---------------------------------------------------------------------------
# WorkPluginRunner.load_plugin
# ---------------------------------------------------------------------------

class TestLoadPlugin:
    def _make_entry_point(self, name, worker_module, action_exists=True, exported=True, payload_type=BYTES_PAYLOAD):
        ep = MagicMock()
        ep.name = name
        action = MagicMock()
        action.receptor_export = exported
        action.payload_type = payload_type
        if action_exists:
            setattr(worker_module, "myaction", action)
        else:
            del worker_module.myaction
        ep.load.return_value = worker_module
        return ep

    def test_plugin_not_found(self, tmp_path):
        wpr = _make_wpr(tmp_path)
        with patch("receptor_python_worker.work.pkg_resources.iter_entry_points", return_value=[]):
            with pytest.raises(ValueError, match="not found"):
                wpr.load_plugin()

    def test_action_not_found(self, tmp_path):
        wpr = _make_wpr(tmp_path)
        worker = MagicMock(spec=[])  # no attributes
        ep = MagicMock()
        ep.name = "myplugin"
        ep.load.return_value = worker
        with patch("receptor_python_worker.work.pkg_resources.iter_entry_points", return_value=[ep]):
            with pytest.raises(ValueError, match="does not exist"):
                wpr.load_plugin()

    def test_action_not_exported(self, tmp_path):
        wpr = _make_wpr(tmp_path)
        action = MagicMock()
        action.receptor_export = False
        worker = MagicMock()
        worker.myaction = action
        ep = MagicMock()
        ep.name = "myplugin"
        ep.load.return_value = worker
        with patch("receptor_python_worker.work.pkg_resources.iter_entry_points", return_value=[ep]):
            with pytest.raises(ValueError, match="Not allowed"):
                wpr.load_plugin()

    def test_load_plugin_success(self, tmp_path):
        wpr = _make_wpr(tmp_path)
        action = MagicMock()
        action.receptor_export = True
        action.payload_type = BYTES_PAYLOAD
        worker = MagicMock()
        worker.myaction = action
        ep = MagicMock()
        ep.name = "myplugin"
        ep.load.return_value = worker
        with patch("receptor_python_worker.work.pkg_resources.iter_entry_points", return_value=[ep]):
            wpr.load_plugin()
        assert wpr.plugin_action_method is action
        assert wpr.payload_input_type == BYTES_PAYLOAD

    def test_load_plugin_default_payload_type(self, tmp_path):
        wpr = _make_wpr(tmp_path)
        action = MagicMock(spec=["receptor_export"])  # no payload_type attr
        action.receptor_export = True
        worker = MagicMock()
        worker.myaction = action
        ep = MagicMock()
        ep.name = "myplugin"
        ep.load.return_value = worker
        with patch("receptor_python_worker.work.pkg_resources.iter_entry_points", return_value=[ep]):
            wpr.load_plugin()
        assert wpr.payload_input_type == BYTES_PAYLOAD


# ---------------------------------------------------------------------------
# WorkPluginRunner.save_status / write_stdout
# ---------------------------------------------------------------------------

class TestSaveStatusAndWriteStdout:
    def test_save_status_writes_json(self, tmp_path):
        wpr = _make_wpr(tmp_path)
        wpr.save_status(WorkStateRunning, "Running")
        data = json.loads((tmp_path / "status").read_text())
        assert data["State"] == WorkStateRunning
        assert data["Detail"] == "Running"
        assert data["StdoutSize"] == 0

    def test_save_status_includes_stdout_size(self, tmp_path):
        wpr = _make_wpr(tmp_path)
        wpr.stdout_size = 42
        wpr.save_status(WorkStateSucceeded, "Done")
        data = json.loads((tmp_path / "status").read_text())
        assert data["StdoutSize"] == 42

    def test_write_stdout_appends(self, tmp_path):
        wpr = _make_wpr(tmp_path)
        wpr.write_stdout(b"abc")
        wpr.write_stdout(b"def")
        assert (tmp_path / "stdout").read_bytes() == b"abcdef"

    def test_write_stdout_updates_size(self, tmp_path):
        wpr = _make_wpr(tmp_path)
        wpr.write_stdout(b"hello")
        assert wpr.stdout_size == 5


# ---------------------------------------------------------------------------
# WorkPluginRunner.queue_monitor
# ---------------------------------------------------------------------------

class TestQueueMonitor:
    def test_monitor_writes_items_to_stdout(self, tmp_path):
        wpr = _make_wpr(tmp_path)
        wpr.response_queue.put({"result": "ok"})

        # Stop the loop *after* the write completes so the write guard
        # (if not self.shutting_down) is still True during the write.
        original_write = wpr.write_stdout

        def write_then_stop(data):
            original_write(data)
            wpr.shutting_down = True  # loop exits after task_done()

        wpr.write_stdout = write_then_stop
        t = threading.Thread(target=wpr.queue_monitor)
        t.start()
        t.join(timeout=2)
        assert not t.is_alive()
        content = (tmp_path / "stdout").read_bytes()
        assert b'"result": "ok"' in content

    def test_monitor_skips_write_when_shutting_down(self, tmp_path):
        wpr = _make_wpr(tmp_path)
        wpr.shutting_down = True
        wpr.response_queue.put({"drop": "me"})
        t = threading.Thread(target=wpr.queue_monitor)
        t.start()
        t.join(timeout=2)
        assert (tmp_path / "stdout").read_bytes() == b""


# ---------------------------------------------------------------------------
# WorkPluginRunner.run (method)
# ---------------------------------------------------------------------------

class TestWorkPluginRunnerRun:
    def _wpr_with_action(self, tmp_path, payload_type):
        wpr = _make_wpr(tmp_path)
        action = MagicMock()
        action.receptor_export = True
        action.payload_type = payload_type
        wpr.plugin_action_method = action
        wpr.payload_input_type = payload_type
        return wpr

    def test_run_bytes_payload(self, tmp_path):
        wpr = self._wpr_with_action(tmp_path, BYTES_PAYLOAD)
        wpr.run()
        wpr.plugin_action_method.assert_called_once()
        args = wpr.plugin_action_method.call_args[0]
        assert args[0] == b"hello"

    def test_run_file_payload(self, tmp_path):
        wpr = self._wpr_with_action(tmp_path, FILE_PAYLOAD)
        wpr.run()
        args = wpr.plugin_action_method.call_args[0]
        assert args[0] == wpr.stdin_filename

    def test_run_buffer_payload(self, tmp_path):
        wpr = self._wpr_with_action(tmp_path, BUFFER_PAYLOAD)
        wpr.run()
        args = wpr.plugin_action_method.call_args[0]
        assert hasattr(args[0], "read")

    def test_run_unknown_payload_raises(self, tmp_path):
        wpr = _make_wpr(tmp_path)
        wpr.plugin_action_method = MagicMock()
        wpr.payload_input_type = "unknown"
        with pytest.raises(ValueError, match="Unknown plugin action method"):
            wpr.run()

    def test_run_sets_succeeded_status(self, tmp_path):
        wpr = self._wpr_with_action(tmp_path, BYTES_PAYLOAD)
        wpr.run()
        data = json.loads((tmp_path / "status").read_text())
        assert data["State"] == WorkStateSucceeded


# ---------------------------------------------------------------------------
# Module-level run() entrypoint
# ---------------------------------------------------------------------------

class TestModuleRun:
    def test_wrong_arg_count_exits(self):
        with patch.object(sys, "argv", ["prog"]):
            with pytest.raises(SystemExit) as exc:
                work.run()
            assert exc.value.code == 1

    def test_invalid_unitdir_exits(self, tmp_path):
        with patch.object(sys, "argv", ["prog", "ns:fn", "relative/path", "{}"]):
            with pytest.raises(SystemExit) as exc:
                work.run()
            assert exc.value.code == 1

    def test_invalid_json_config_exits(self, tmp_path):
        _make_unitdir(tmp_path)
        with patch.object(sys, "argv", ["prog", "ns:fn", str(tmp_path), "not-json"]):
            with pytest.raises(SystemExit) as exc:
                work.run()
            assert exc.value.code == 1

    def test_load_plugin_failure_exits(self, tmp_path):
        _make_unitdir(tmp_path)
        with patch.object(sys, "argv", ["prog", "ns:fn", str(tmp_path), "{}"]):
            with patch("receptor_python_worker.work.pkg_resources.iter_entry_points", return_value=[]):
                with pytest.raises(SystemExit) as exc:
                    work.run()
                assert exc.value.code == 0  # saves status and exits 0

    def test_successful_run(self, tmp_path):
        _make_unitdir(tmp_path)
        action = MagicMock()
        action.receptor_export = True
        action.payload_type = BYTES_PAYLOAD
        worker = MagicMock()
        worker.fn = action
        ep = MagicMock()
        ep.name = "ns"
        ep.load.return_value = worker
        with patch.object(sys, "argv", ["prog", "ns:fn", str(tmp_path), "{}"]):
            with patch("receptor_python_worker.work.pkg_resources.iter_entry_points", return_value=[ep]):
                work.run()
        data = json.loads((tmp_path / "status").read_text())
        assert data["State"] == WorkStateSucceeded

    def test_signal_handler_saves_failed_status(self, tmp_path):
        _make_unitdir(tmp_path)
        action = MagicMock()
        action.receptor_export = True
        action.payload_type = BYTES_PAYLOAD

        def slow_action(payload, config, q):
            # Send SIGTERM to self mid-run
            import os
            os.kill(os.getpid(), signal.SIGTERM)

        action.side_effect = slow_action
        worker = MagicMock()
        worker.fn = action
        ep = MagicMock()
        ep.name = "ns"
        ep.load.return_value = worker
        with patch.object(sys, "argv", ["prog", "ns:fn", str(tmp_path), "{}"]):
            with patch("receptor_python_worker.work.pkg_resources.iter_entry_points", return_value=[ep]):
                with pytest.raises(SystemExit) as exc:
                    work.run()
        assert exc.value.code == 0
        data = json.loads((tmp_path / "status").read_text())
        assert data["State"] == WorkStateFailed
