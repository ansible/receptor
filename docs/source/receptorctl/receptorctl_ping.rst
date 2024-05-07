----
ping
----

.. contents::
   :local:

``receptorctl ping`` Pings a receptornode.

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
``lsof -p <<pid>``

``count`` (positive integer) is the number of pings to send (default is 4).

``delay`` (positive float) is the time (in seconds) to wait between pings (default = 1.0)
