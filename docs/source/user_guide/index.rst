.. _user_guide:

*******************
User guide
*******************

This guide describes how to use receptor in multiple environments and uses the following terms:

.. glossary::

   receptor
      The receptor application taken as a whole, which typically runs as a daemon.
      
   receptorctl
      A user-facing command line used to interact with receptor, typically over a Unix domain socket.

   node
      A single running instance of receptor.

   node ID
      An arbitrary string identifying a single node, analogous to an IP address.

   backend
      A type of connection that receptor nodes can pass traffic over. Current backends include TCP, UDP and websockets.

   control service
      A built-in service that usually runs under the name `control`.  Used to report status and to launch and monitor work.

   netceptor
      The component of receptor that handles all networking functionality.

   workceptor
      The component of receptor that handles work units.

.. toctree::
   :maxdepth: 2

   basic_usage
   connecting_nodes
   interacting_with_nodes
   workceptor
   k8s
   tls
   firewall
   edge_networks


