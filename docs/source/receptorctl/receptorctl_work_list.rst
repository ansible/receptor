---------
work list
---------

.. contents::
   :local:

``receptorctl work list`` displays known units of work

Command syntax: ``receptorctl --socket=<socket_path> work list``

``socket_path`` is the control socket address for the Receptor connection.
   The default is ``unix:`` for a Unix socket.
   Use ``tcp://`` for a TCP socket.
   The corresponding environment variable is ``RECEPTORCTL_SOCKET``.

.. code-block:: text

  ss --listening --processes --unix 'src = unix:<socket_path>'
  Netid         State          Recv-Q         Send-Q                   Local Address:Port                     Peer Address:Port        Process
  u_str         LISTEN         0              4096                   /tmp/local.sock 38130170                            * 0            users:(("receptor",pid=3226769,fd=7))

``ps -fp $(pidof receptor)``
``lsof -p <<pid>``

The output is divided into work unit sections listed below.
Field values may be listed in their own section.
Columns are the actual JSON node values.

^^^^^^^^^^^^^^^^^
Work unit section
^^^^^^^^^^^^^^^^^

.. list-table:: Work unit section
      :header-rows: 1
      :widths: auto

      * - Column
        - Description
      * - ``."Work unit string"``
        - Random 8 character work unit (job) string.
      * - ``."Work unit string"."Detail"``
        - Work unit output.
      * - ``."Work unit string"."ExtraData"``
        - Additional information.
      * - ``.""Work unit string"."State"``
        - Current state for work unit (int).
      * - ``.""Work unit string"."StateName"``
        - Human readable current state for work unit.
      * - ``.""Work unit string"."StdoutSize"``
        - Size of work unit output (bytes).
      * - ``.""Work unit string"."WorkType"``
        - Work unit type (queue).

^^^^^^^^^^^^^^^^
Work Unit States
^^^^^^^^^^^^^^^^

.. list-table:: Work unit states
      :header-rows: 1
      :widths: auto

      * - State
        - StateName
        - Description
      * - ``0``
        - ``Pending``
        - Work unit has not started.
      * - ``1``
        - ``Running``
        - Work unit is executing.
      * - ``2``
        - ``Succeeded``
        - Work unit completed without error.
      * - ``3``
        - ``Failed``
        - Work unit completed with error.
      * - ``4``
        - ``Canceled``
        - Work unit was terminated externally.
