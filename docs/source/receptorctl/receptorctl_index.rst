========================
Receptor client commands
========================

The Receptor client, ``receptorctl``, provides a command line interface for interacting with and managing Receptor nodes.

.. toctree::
   :maxdepth: 1
   :glob:
   :caption: Receptorctl commands

   receptorctl_connect
   receptorctl_ping
   receptorctl_reload
   receptorctl_status
   receptorctl_traceroute
   receptorctl_version
   receptorctl_work_cancel
   receptorctl_work_list
   receptorctl_work_release
   receptorctl_work_results
   receptorctl_work_submit

.. attention:
   Receptor has commands that are intended to provide internal functionality.  These commands are not supported by ``receptorctl``:
   - ``work force-release``.
   - ``work status``.
