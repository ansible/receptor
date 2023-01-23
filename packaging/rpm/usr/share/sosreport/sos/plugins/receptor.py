# Copyright (c) 2021 Ansible, Inc.
# All Rights Reserved.

import glob

try:
    from sos.plugins import Plugin, RedHatPlugin
except ImportError:
    from sos.report.plugins import Plugin, RedHatPlugin

SOSREPORT_COMMANDS = [
    "ls -ll /etc/receptor",
    "ls -ll /var/run/receptor",
    "ls -ll /var/run/awx-receptor"
]

SOSREPORT_DIRS = [
    "/etc/receptor",
    "/var/lib/receptor"
]

SOSREPORT_FORBIDDEN_PATHS = [
    "/etc/receptor/tls"
]


class Receptor(Plugin, RedHatPlugin):
    '''Collect Receptor information'''

    short_desc = "Receptor information"
    plugin_name = "receptor"
    packages = ('receptor', 'receptorctl',)
    services = ('receptor',)

    def setup(self):

        for s in glob.glob('/var/run/*receptor/*.sock'):
            SOSREPORT_COMMANDS.append(f"receptorctl --socket {s} status")
        self.add_cmd_output(SOSREPORT_COMMANDS)

        if self.get_option("all_logs"):
            SOSREPORT_DIRS.append("/var/log/receptor")
        else:
            SOSREPORT_DIRS.append("/var/log/receptor/*.log")
        self.add_copy_spec(SOSREPORT_DIRS)

        self.add_forbidden_path(SOSREPORT_FORBIDDEN_PATHS)
