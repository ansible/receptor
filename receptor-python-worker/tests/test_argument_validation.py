"""
Unit tests for CLI argument validation in work.py.

Covers:
  - validate_plugin_directive: allowlist pattern enforcement
  - validate_unitdir: path-traversal prevention and basic sanity checks
"""
import os
import tempfile
import pytest

from receptor_python_worker.work import validate_plugin_directive, validate_unitdir


# ---------------------------------------------------------------------------
# validate_plugin_directive
# ---------------------------------------------------------------------------

class TestValidatePluginDirective:
    def test_valid_simple(self):
        validate_plugin_directive("myplugin:myaction")

    def test_valid_underscores(self):
        validate_plugin_directive("my_plugin:my_action")

    def test_valid_mixed_case(self):
        validate_plugin_directive("MyPlugin:MyAction")

    def test_valid_digits(self):
        validate_plugin_directive("plugin1:action2")

    # --- invalid inputs ---

    def test_missing_colon(self):
        with pytest.raises(ValueError, match="namespace:function"):
            validate_plugin_directive("pluginaction")

    def test_empty_string(self):
        with pytest.raises(ValueError):
            validate_plugin_directive("")

    def test_shell_metachar_semicolon(self):
        with pytest.raises(ValueError):
            validate_plugin_directive("plugin;rm -rf /:action")

    def test_shell_metachar_ampersand(self):
        with pytest.raises(ValueError):
            validate_plugin_directive("plugin&action:func")

    def test_path_separator_in_namespace(self):
        with pytest.raises(ValueError):
            validate_plugin_directive("../evil:action")

    def test_path_separator_in_action(self):
        with pytest.raises(ValueError):
            validate_plugin_directive("plugin:../../etc/passwd")

    def test_newline_injection(self):
        with pytest.raises(ValueError):
            validate_plugin_directive("plugin:ac\ntion")

    def test_null_byte(self):
        with pytest.raises(ValueError):
            validate_plugin_directive("plugin:action\x00")

    def test_double_colon(self):
        # Two colons: second part contains a colon — not matched by strict pattern
        with pytest.raises(ValueError):
            validate_plugin_directive("ns:action:extra")

    def test_space_in_directive(self):
        with pytest.raises(ValueError):
            validate_plugin_directive("my plugin:action")


# ---------------------------------------------------------------------------
# validate_unitdir
# ---------------------------------------------------------------------------

class TestValidateUnitdir:
    def test_valid_real_directory(self, tmp_path):
        result = validate_unitdir(str(tmp_path))
        assert result == tmp_path.resolve()

    def test_rejects_relative_path(self):
        with pytest.raises(ValueError, match="absolute"):
            validate_unitdir("relative/path")

    def test_rejects_dot_relative(self):
        with pytest.raises(ValueError, match="absolute"):
            validate_unitdir("./some/dir")

    def test_rejects_dotdot_traversal(self, tmp_path):
        # Construct a path that normalises back inside tmp_path but uses '..'
        subdir = tmp_path / "sub"
        subdir.mkdir()
        traversal = str(subdir) + "/../../etc"
        with pytest.raises(ValueError):
            validate_unitdir(traversal)

    def test_rejects_nonexistent_directory(self):
        with pytest.raises(ValueError, match="does not exist"):
            validate_unitdir("/nonexistent_receptor_unitdir_xyz_12345")

    def test_rejects_file_path(self, tmp_path):
        f = tmp_path / "notadir"
        f.write_text("data")
        with pytest.raises(ValueError, match="does not exist or is not a directory"):
            validate_unitdir(str(f))

    def test_symlink_outside_base_is_rejected(self, tmp_path):
        # Create a symlink that points outside tmp_path
        target = tmp_path / "real_sub"
        target.mkdir()
        outside = tempfile.mkdtemp()
        try:
            link = tmp_path / "escape_link"
            link.symlink_to(outside)
            with pytest.raises(ValueError, match="traversal"):
                validate_unitdir(str(link))
        finally:
            os.rmdir(outside)

    def test_returns_resolved_path_type(self, tmp_path):
        from pathlib import Path
        result = validate_unitdir(str(tmp_path))
        assert isinstance(result, Path)
