.. receptor documentation master file, created by
   sphinx-quickstart on Sat May  1 19:39:15 2021.
   You can adapt this file completely to your liking, but it should at least
   contain the root `toctree` directive.

Introduction
============

Receptor is an overlay network intended to ease the distribution of work across a large and dispersed collection of workers. Receptor nodes establish peer-to-peer connections with each other via existing networks. Once connected, the receptor mesh provides datagram (UDP-like) and stream (TCP-like) capabilities to applications, as well as robust unit-of-work handling with resiliency against transient network failures.

Terminology and Concepts
^^^^^^^^^^^^^^^^^^^^^^^^

- `receptor`: The receptor application taken as a whole, which typically runs as a daemon.
- `receptorctl`: A user-facing command line used to interact with receptor, typically over a Unix domain socket.
- `node`: A single running instance of receptor.
- `node ID`: An arbitrary string identifying a single node, analogous to an IP address.
- `backend`: A type of connection that receptor nodes can pass traffic over. Current backends include TCP, UDP and websockets.
- `control service`: A built-in service that usually runs under the name `control`.  Used to report status and to launch and monitor work.
- `netceptor`: The component of receptor that handles all networking functionality.
- `workceptor`: The component of receptor that handles work units.

Installation
^^^^^^^^^^^^

Compile from source code (Golang 1.15.x required)

.. code::

    make receptor

Test the installation with

.. code::

    receptor --help
    receptor --version

The preferred way to interact with receptor nodes is to use the receptorctl command line tool

.. code::

    pip install receptorctl

receptorctl will be used in various places throughout this documentation.

Basic usage
^^^^^^^^^^^

Run the following command in a terminal to start a node called `foo`,

.. code::

    receptor --node id=foo --local-only -- log-level Debug

The log shows the receptor node started successfully

``INFO 2021/07/22 22:40:36 Initialization complete``

Supported log levels, in increasing verbosity, are Error, Warning, Info and Debug.

Note: stop the receptor process with ``ctrl-c``

Config file
^^^^^^^^^^^

Receptor can be configured on the command-line, exemplified above, or via a yaml config file. All actions and parameters shown in ``receptor --help`` can be written to a config file.

.. code-block:: yaml

    ---
    - node:
        id: foo

    - local-only

    - log-level:
        level: Debug

Start receptor using the config file

.. code::

    receptor --config foo.yml

Changing the configuration file does take effect until the receptor process is restarted.

Container image
^^^^^^^^^^^^^^^

.. code::

    podman pull quay.io/project-receptor/receptor

Start a container, which automatically runs receptor with the default config located at ``/etc/receptor/receptor.conf``

.. code::

    podman run -it --rm --name receptor quay.io/project-receptor/receptor

In another terminal, issue a basic "status" command to the running receptor process

.. code::

    $ podman exec receptor receptorctl status
    Node ID: d9b5a8e3c156
    Version: 1.0.0
    System CPU Count: 8
    System Memory MiB: 15865

    Node         Service   Type       Last Seen           Tags            Work Types
    d9b5a8e3c156 control   Stream     2021-08-04 19:26:14 -               -

Note: the config file does not specify a node ID, so the hostname (on the container) is chosen as the node ID.

.. toctree::
   :maxdepth: 3
   :caption: Contents:

   self
   connecting_nodes
   interacting_with_nodes
   workceptor
   k8s
   tls
   firewall
   developer_guide
