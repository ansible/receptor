----
ping
----

.. contents::
   :local:

``receptorctl ping`` tests the network reachability of Receptor nodes.

Command syntax: ``receptorctl --socket=<socket_path> [--count <count>] [--delay <delay>] ping <remote_node>``

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

``count`` specifies the number of pings to send.  The value must be a positive integer. The default is ``4``.

``delay`` specifies the time, in seconds, to wait between pings.  The value must be a positive float. The default is ``1.0``.
