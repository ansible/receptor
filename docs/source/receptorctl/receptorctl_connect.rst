-------
connect
-------

.. contents::
   :local:

``receptorctl connect`` establishes a connection between local client and a Receptor node.

Command syntax: ``receptorctl --socket=<socket_path> connect <remote_node> <remote_control_service>``

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

``remote_node`` is the identifier of a Receptor node.

``remote_control_service`` is the service name of a Receptor node.

.. seealso::

    :ref:`connect_to_csv`
        Connect to any Receptor control service running on the mesh.
