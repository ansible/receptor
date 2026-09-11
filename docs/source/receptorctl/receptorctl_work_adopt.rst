----------
work adopt
----------

.. contents::
   :local:

``receptorctl work adopt`` attaches to an already-running unit of work on a remote Receptor node, allowing a node to monitor and retrieve results from work that was submitted by another node.

Command syntax: ``receptorctl --socket=<socket_path> work adopt [<<Options>>] --node <<Node ID>> <<Unit ID>>``

``socket_path`` is the control socket address for the Receptor connection.
   The default is ``unix:`` for a Unix socket.
   Use ``tcp://`` for a TCP socket.
   The corresponding environment variable is ``RECEPTORCTL_SOCKET``.

.. code-block:: text

  ss --listening --processes --unix 'src = unix:<socket_path>'
  Netid         State          Recv-Q         Send-Q                   Local Address:Port                     Peer Address:Port        Process
  u_str         LISTEN         0              4096                   /tmp/local.sock 38130170                            * 0            users:(("receptor",pid=3226769,fd=7))

``ps -fp $(pidof receptor)``
``lsof -p <pid>``

``Node ID`` is the identifier of the Receptor node where the work unit is currently running.

``Unit ID`` is the unique identifier for the work unit on the remote node. This is the unit ID that was returned when the work was originally submitted.

^^^^^^^^^^^^^^^^^^
Work adopt options
^^^^^^^^^^^^^^^^^^

You can use the following options with the ``work adopt`` command:

.. list-table::
    :header-rows: 1
    :widths: auto

    * - Option
      - Description
    * - ``--node <<Node ID>>``
      - Specifies the Receptor node where the work unit is running (required).
    * - ``-f``, ``--follow``
      - Remains attached to the job and streams the job results to standard output (stdout).
    * - ``--rm``
      - Releases the local work unit after the remote work completes.
    * - ``--signwork``
      - Digitally signs the adopt request when connecting to the remote node.
    * - ``--tls-client <<TEXT>>``
      - Specifies the TLS client configuration to use when connecting to the remote node.

^^^^^^^^^^^^^
Usage example
^^^^^^^^^^^^^

The ``work adopt`` command is useful when multiple nodes need to monitor the same work unit, or when you want to monitor work that was submitted from a different node.

.. code-block:: shell

   # Node A submits work to Node C
   receptorctl --socket /tmp/nodeA.sock work submit --node nodeC echojob
   Unit ID: abc-123

   # Node B can adopt the same work unit to monitor it
   receptorctl --socket /tmp/nodeB.sock work adopt --node nodeC --follow abc-123

The ``work adopt`` command is idempotent. Running it multiple times with the same unit ID will not create duplicate monitoring sessions.

.. note::
   The work unit must already exist and be running on the remote node. The ``work adopt`` command does not submit new work; it only attaches to existing work units.
