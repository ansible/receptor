import sys
import os
import re
import threading
import signal
import queue
from pathlib import Path
import json
import pkg_resources
from .plugin_utils import BUFFER_PAYLOAD, BYTES_PAYLOAD, FILE_PAYLOAD

# Allow existing worker plugins to "import receptor" and get our version of plugin_utils
sys.modules['receptor'] = sys.modules[__package__+'.plugin_utils']

# Allowlist pattern for plugin directives: "namespace:function" where both parts are
# restricted to alphanumerics and underscores. This blocks shell metacharacters,
# path separators, and other injection vectors before the string reaches pkg_resources.
_PLUGIN_DIRECTIVE_RE = re.compile(r'^[A-Za-z0-9_]+:[A-Za-z0-9_]+$')


def validate_plugin_directive(directive):
    """Validate that a plugin directive matches 'namespace:function' allowlist pattern.

    Raises ValueError if the directive contains any character outside [A-Za-z0-9_:]
    or does not follow the expected two-part colon-delimited format.
    """
    if not _PLUGIN_DIRECTIVE_RE.match(directive):
        raise ValueError(
            "Plugin directive must be 'namespace:function' using only alphanumerics and underscores"
        )


def validate_unitdir(unitdir):
    """Resolve and validate a unit directory path against path-traversal attacks.

    Strategy:
      1. Require an absolute path so relative paths are rejected immediately.
      2. Reject any '..' components in the path parts before touching the filesystem.
      3. Call Path.resolve() (follows symlinks) and compare to the lexically-normalised
         form.  A mismatch means a symlink redirects the path to a different location —
         reject it so callers cannot use a planted symlink to escape the intended tree.
      4. Require the resolved path to be an existing directory.

    Returns the resolved Path on success; raises ValueError on any violation.
    """
    p = Path(unitdir)
    if not p.is_absolute():
        raise ValueError(f"unitdir must be an absolute path, got: {unitdir!r}")

    # Catch '..' traversal in the path string itself before any I/O.
    if ".." in p.parts:
        raise ValueError(f"unitdir must not contain '..' components: {unitdir!r}")

    resolved = p.resolve()
    normalised = Path(os.path.normpath(str(p)))
    # If resolve() changed the path, a symlink redirects to a different location.
    if resolved != normalised:
        raise ValueError(
            f"unitdir resolves outside its stated location (symlink or traversal): {unitdir!r}"
        )

    if not resolved.is_dir():
        raise ValueError(f"unitdir does not exist or is not a directory: {unitdir!r}")

    return resolved

# WorkState constants
WorkStatePending = 0
WorkStateRunning = 1
WorkStateSucceeded = 2
WorkStateFailed = 3


class WorkPluginRunner:
    def __init__(self, plugin_directive, unitdir, config):
        # Validate both CLI arguments before touching the filesystem.
        validate_plugin_directive(plugin_directive)
        self.plugin_namespace, self.plugin_action = plugin_directive.split(":", 1)

        safe_unitdir = validate_unitdir(unitdir)
        self.config = config
        self.unitdir = str(safe_unitdir)
        self.status_filename = os.path.join(self.unitdir, "status")
        if not os.path.exists(self.status_filename):
            raise ValueError("Status file does not exist in unitdir")
        self.stdin_filename = os.path.join(self.unitdir, "stdin")
        if not os.path.exists(self.stdin_filename):
            raise ValueError("Stdin file does not exist in unitdir")
        self.stdout_filename = os.path.join(self.unitdir, "stdout")
        Path(self.stdout_filename).touch(mode=0o0600, exist_ok=True)
        with open(self.status_filename) as file:
            self.status = json.load(file)
        self.plugin_worker = None
        self.plugin_action_method = None
        self.payload_input_type = None
        self.stdout_size = 0
        self.response_queue = queue.Queue()
        self.monitor_thread = None
        self.shutting_down = False

    def load_plugin(self):
        entry_points = [
            x
            for x in filter(
                lambda x: x.name == self.plugin_namespace, pkg_resources.iter_entry_points("receptor.worker")
            )
        ]
        if not entry_points:
            raise ValueError(f"Plugin {self.plugin_namespace} not found")
        self.plugin_worker = entry_points[0].load()
        self.plugin_action_method = getattr(self.plugin_worker, self.plugin_action, False)
        if not self.plugin_action_method:
            raise ValueError(f"Function {self.plugin_action} does not exist in {self.plugin_namespace}")
        if not getattr(self.plugin_action_method, "receptor_export", False):
            raise ValueError(f"Not allowed to call non-exported {self.plugin_action} in {self.plugin_namespace}")
        self.payload_input_type = getattr(self.plugin_action_method, "payload_type", BYTES_PAYLOAD)

    def save_status(self, state, detail):
        self.status['State'] = state
        self.status['Detail'] = detail
        self.status['StdoutSize'] = self.stdout_size
        with open(self.status_filename, 'w') as file:
            json.dump(self.status, file)

    def write_stdout(self, data):
        with open(self.stdout_filename, 'ab') as file:
            file.write(data)
            self.stdout_size = file.tell()

    def queue_monitor(self):
        self.save_status(WorkStateRunning, "Running")
        while not self.shutting_down:
            item = self.response_queue.get()
            if not self.shutting_down:
                self.write_stdout(bytes(json.dumps(item)+"\n", 'UTF-8'))
                self.save_status(WorkStateRunning, "Running")
            self.response_queue.task_done()

    def run(self):
        self.save_status(WorkStatePending, "Starting Python worker plugin")
        if self.payload_input_type == FILE_PAYLOAD:
            payload = self.stdin_filename
        elif self.payload_input_type == BUFFER_PAYLOAD:
            payload = open(self.stdin_filename, 'rb')
        elif self.payload_input_type == BYTES_PAYLOAD:
            with open(self.stdin_filename, 'rb') as file:
                payload = file.read()
        else:
            raise ValueError("Unknown plugin action method")
        self.monitor_thread = threading.Thread(target=self.queue_monitor, daemon=True)
        self.monitor_thread.start()
        self.plugin_action_method(payload, self.config, self.response_queue)
        self.response_queue.join()
        self.save_status(WorkStateSucceeded, "Complete")


def run():
    if len(sys.argv) != 4:
        print("Invalid command line usage")
        sys.exit(1)

    try:
        wpr = WorkPluginRunner(sys.argv[1], sys.argv[2], json.loads(sys.argv[3]))
    except Exception as e:
        print(f"Error initializing worker object: {repr(e)}")
        sys.exit(1)

    def signal_handler(signum, frame):
        try:
            wpr.shutting_down = True
            wpr.save_status(WorkStateFailed, f"Killed by signal {signum}")
        except Exception as e:
            print(f"Error saving status: {repr(e)}")
            sys.exit(1)
        sys.exit(0)

    signal.signal(signal.SIGINT, signal_handler)
    signal.signal(signal.SIGTERM, signal_handler)

    try:
        message = "Error loading worker"
        wpr.load_plugin()
        message = "Error running worker"
        wpr.run()
    except Exception as e:
        try:
            wpr.shutting_down = True
            wpr.save_status(WorkStateFailed, f"{message}: {repr(e)}")
        except Exception as e:
            print(f"Error saving status: {repr(e)}")
            sys.exit(1)
        sys.exit(0)