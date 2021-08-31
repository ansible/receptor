# Copyright (c) 2016 Ansible, Inc.
# All Rights Reserved.

try:
    from sos.plugins import Plugin, RedHatPlugin
except ImportError:
    from sos.report.plugins import Plugin, RedHatPlugin

SOSREPORT_COMMANDS = [
    "ls -ll /var/run/receptor",  # check default socket file location
    "ls -ll /etc/receptor",  # list conf files
    "umask -p",  # check current umask
]

SOSREPORT_DIRS = [
    "/etc/receptor/",
]

SOSREPORT_FORBIDDEN_PATHS = [
]


class Receptor(Plugin, RedHatPlugin):
    '''Collect Receptor information'''

    plugin_name = "receptor"
    short_desc = "Receptor information"

    def setup(self):

        for path in SOSREPORT_DIRS:
            self.add_copy_spec(path)

        self.add_forbidden_path(SOSREPORT_FORBIDDEN_PATHS)

        self.add_cmd_output(SOSREPORT_COMMANDS)
