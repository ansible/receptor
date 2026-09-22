"""Tests for plugin_utils — covers the decorator body (lines 51-56)."""
import subprocess
import sys

from receptor_python_worker.plugin_utils import (
    BUFFER_PAYLOAD,
    BYTES_PAYLOAD,
    FILE_PAYLOAD,
    plugin_export,
)


class TestPluginExport:
    def test_decorator_sets_receptor_export(self):
        @plugin_export(payload_type=BYTES_PAYLOAD)
        def my_action(msg, cfg, q):
            pass

        assert my_action.receptor_export is True

    def test_decorator_sets_payload_type_bytes(self):
        @plugin_export(payload_type=BYTES_PAYLOAD)
        def my_action(msg, cfg, q):
            pass

        assert my_action.payload_type == BYTES_PAYLOAD

    def test_decorator_sets_payload_type_buffer(self):
        @plugin_export(payload_type=BUFFER_PAYLOAD)
        def my_action(msg, cfg, q):
            pass

        assert my_action.payload_type == BUFFER_PAYLOAD

    def test_decorator_sets_payload_type_file(self):
        @plugin_export(payload_type=FILE_PAYLOAD)
        def my_action(msg, cfg, q):
            pass

        assert my_action.payload_type == FILE_PAYLOAD

    def test_decorator_returns_original_function(self):
        def my_action(msg, cfg, q):
            return "result"

        decorated = plugin_export(payload_type=BYTES_PAYLOAD)(my_action)
        assert decorated("m", "c", "q") == "result"


class TestMainModule:
    def test_main_module_calls_run(self):
        result = subprocess.run(
            [sys.executable, "-m", "receptor_python_worker"],
            capture_output=True,
            text=True,
        )
        assert result.returncode == 1
        assert "Invalid command line usage" in result.stdout
