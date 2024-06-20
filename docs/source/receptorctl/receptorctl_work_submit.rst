-----------
work submit
-----------

.. contents::
   :local:

``receptorctl work submit`` requests a Receptor node to run a unit of work.

Command syntax: ``receptorctl --socket=<socket_path> work submit [<<Options>>] <<WorkType>> [<<Runtime Parameters>>]``

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

``WorkType`` specifies an execution request type for the work unit.  Use the ``receptorctl status`` command to find available work types for Receptor nodes.

``Runtime Parameters`` are parameters passed by Receptor to the work command.

^^^^^^^^^^^^^^^^^^^
Work submit options
^^^^^^^^^^^^^^^^^^^

You can use the following options with the ``work submit`` command:

``-a``, ``--param <<KEY>>=<<VALUE>>`` adds a Receptor parameter in key=value format.
``-f``, ``--follow`` keeps Receptorctl to remain attached to the job and displays the job results.
``-l``, ``--payload-literal <<TEXT>>`` uses the value of ``<<TEXT>>`` as the literal unit of work data.
``-n``, ``--no-payload`` sends an empty payload.
``--node <<Node ID>>`` is the Receptor node on which the work runs. The default is the local node.
``-p``, ``--payload <<TEXT>>`` specifies the file that contains data for the unit of work. Specify ``-`` for standard input (stdin).
``--rm`` releases the work unit after completion.
``--signwork`` digitally signs remote work submissions to standard output (stdout).
``--tls-client <<TEXT>>`` specifies the TLS client that submits work to a remote node.
``--ttl <<TEXT>>`` specifies the time to live (TTL) for remote work requests in ``##h##m##s`` format; for example ``1h20m30s`` or ``30m10s``. Use the ``receptorctl work list`` command to display units of work on Receptor nodes and determine appropriate TTL values.
