------------
work release
------------

.. contents::
   :local:

``receptorctl work release`` Deletes one or more units of work.

Command syntax: ``receptorctl --socket=<socket_path> work release [<<Options>>] <<Unit ID>> [...]``

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

``Unit ID`` is a unique identifier for a work unit (job).  When running the ``work release`` command, you should specify the ``Unit ID`` for the Receptor instance to which you are connected.

Options:
``--all`` will delete all work units.
``--force`` will delete locally (even if the remote node is unreachable).
