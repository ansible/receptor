------
status
------

.. contents::
   :local:

``receptorctl status`` displays the status of the Receptor network.

The command syntax is: ``receptorctl --socket=<socket_path> status [--json]``

``socket_path`` is the control socket address to connect to Receptor.
   It defaults to Unix socket.
   Use tcp:// for TCP socket.
   The corresponding environment variable is ``RECEPTORCTL_SOCKET``

``ss --listening --processes --unix 'src = unix:<socket_path>'``
Netid         State          Recv-Q         Send-Q                   Local Address:Port                     Peer Address:Port        Process
u_str         LISTEN         0              4096                   /tmp/local.sock 38130170                            * 0            users:(("receptor",pid=3226769,fd=7))

``ps -fp $(pidof receptor)``
``lsof -p <<pid>``

``--json`` will format the output as JSON.
   The default is human-readable.
   Using this argument allows output to be machine consumable.  For example, piping into ``jq``

The output is divided into sections listed below.  Field values may be listed in their own section.
Columns are the actual JSON node values.

^^^^^^^^^^^
Node output
^^^^^^^^^^^

.. list-table:: Node output
      :header-rows: 1
      :widths: auto

      * - Column
        - Description
      * - ``."NodeID"``
        - Node Identifier.
      * - ``."SystemCPUCount"``
        - Number of logical CPU cores on the node.
      * - ``.SystemMemoryMiB"``
        - Available Mmemory (MiB) of the node.
      * - ``."Version"``
        - Receptor version.

^^^^^^^^^^^^^^^^^^
Connections output
^^^^^^^^^^^^^^^^^^

.. list-table:: Connections output
    :header-rows: 1
    :widths: auto

    * - Column
      - Description
    * - ``."Connections"``
      - Connections
    * - ``."Connections"."Cost"``
      - Metric (route preference) to reach NodeID
    * - ``."Connections"."NodeID"``
      - Node ID

^^^^^^^^^^^^^^^^^^^^^^^^
Known Connections output
^^^^^^^^^^^^^^^^^^^^^^^^

.. list-table:: Known Connections output
    :header-rows: 1
    :widths: auto

    * - Column
      - Description
    * - ``"KnownConnectionCosts"``
      - Known Connections
    * - ``"KnownConnectionCosts"."<NodeID 1>"``
      - Remote node ID
    * - ``"KnownConnectionCosts"."<NodeID 1>"."<NodeID 2>"``
      - Cost to get to NodeID 1 through NodeID 2

^^^^^^^^^^^^^^^^^^^^
Routing Table Output
^^^^^^^^^^^^^^^^^^^^

.. list-table:: Routing Table output
    :header-rows: 1
    :widths: auto

    * - Column
      - Description
    * - ``."RoutingTable"``
      - Routing Table
    * - ``."RoutingTable"."<NodeID>"``
      - List of NodeID(s) used to get to desired NodeID

^^^^^^^^^^^^^^^^^^^^^^^^^^^^
Service Advertisement Output
^^^^^^^^^^^^^^^^^^^^^^^^^^^^

.. list-table:: Service Advertisement Output
    :header-rows: 1
    :widths: auto

    * - Column
      - Description
    * - ``."Advertisements"``
      - Advertisements
    * - ``."Advertisements"."ConnType"``
      - Connection type (see below for values)
    * - ``."Advertisements"."NodeID"``
      - Node identifier issuing advertisement
    * - ``."Advertisements"."Service"``
      - Receptor services on node
    * - ``."Advertisements"."Tags"``
      - Tags associated with node
    * - ``."Advertisements"."Time"``
      - Timestamp when advertisement sent

=====================
Execution Node Output
=====================

.. list-table:: Execution Node output
    :header-rows: 1
    :widths: auto

    * - Column
      - Description
    * - ``."Advertisements"."WorkCommands"``
      - Execution Node work commands
    * - ``."Advertisements"."WorkCommands"."Secure"``
      - Boolean indicating whether the work commands are signed
    * - ``."Advertisements"."WorkCommands"."WorkType"``
      - Work command(s) supported

===============
Connection Type
===============

.. list-table:: Connection Types
    :header-rows: 1
    :widths: auto

    * - ConnType Value
      - Description
    * - 0
      - Datagram
    * - 1
      - Stream
    * - 2
      - StreamTLS

====
Tags
====

.. list-table:: Tags
    :header-rows: 1
    :widths: auto

    * - Tags
      - Description
    * - network
      - Network name
    * - route_*
      - Route information for specified node
    * - type
      - Node type
