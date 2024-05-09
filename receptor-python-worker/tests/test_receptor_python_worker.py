import pytest
import receptor_python_worker


class TestRun:
    def test_run(self):
        with pytest.raises(SystemExit) as e:
            receptor_python_worker.work.run()
        assert e.type == SystemExit, f"Unknown exception: '{e.type}'"
