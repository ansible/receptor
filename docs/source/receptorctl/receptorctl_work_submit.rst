-----------
work submit
-----------

.. contents::
   :local:

``receptorctl work submit`` submits a new unit of work.

Command syntax: ``receptorctl --socket=<socket_path> work submit [<<Options>>] <<WorkType>> [<<Command Parameters>>]``

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

``WorkType`` is the execution request type for the work unit.

``Command Parameters`` are parameters passed to the work command.

``-a``, ``--param TEXT`` adds additional Receptor parameter in key=value format.
``-f``, ``--follow`` causes Receptorctl to remain attached to the job and print its results.
``-l``, ``--payload-literal TEXT`` uses the ``TEXT`` as the literal unit of work data.
``-n``, ``--no-payload`` sends an empty payload.
``--node <<Node ID>>`` is the Receptor node to run the work on. It defaults to the local node.
``-p``, ``--payload TEXT`` specifies the file containing unit of work data. Use - for stdin.
``--rm`` releases the work unit after completion.
``--signwork`` digitally signs remote work submissions to stdout.
``--tls-client TEXT`` specifies the TLS client used when submitting work to a remote node.
``--ttl TEXT`` specifies the time to live until remote work must start, e.g. 1h20m30s or 30m10s.
