.. _pull_connectivity:

Pull connectivity
=================

.. contents::
   :local:

Receptor supports two connection models that differ in **which side initiates the TCP/WebSocket connection**.
Understanding this distinction matters when execution nodes sit behind firewalls or in private networks.

Push vs pull
------------

**Push (default)**
  The control plane initiates connections to execution nodes.
  Execution nodes run ``*-listeners``; the control plane uses ``*-peers`` to reach them.
  This requires execution nodes to have inbound firewall ports open (typically 27199 for TCP).

**Pull**
  Execution nodes initiate outbound connections to the control plane or a hop node.
  The control plane runs ``*-listeners``; execution nodes use ``*-peers`` to connect out.
  Only outbound traffic from the execution node is required — no inbound firewall rules are needed on the execution node side.

Pull mode is the right choice when execution nodes are in private networks with egress-only internet access,
or when you cannot open inbound ports on execution nodes.

.. note::

   The direction of the TCP connection does **not** change how work is routed or dispatched across the mesh.
   Once the connection is established, the mesh is symmetric — either side can send work to the other.
   Push and pull refer solely to which node calls ``connect()``.

mTLS requirement
----------------

Production pull deployments require mutual TLS (mTLS). Basic pull connections can run without mTLS, but they do not meet this production security requirement.
Because the control plane never initiates a connection to the execution node, it cannot rely on connecting
to a known address to verify identity.
Instead, both sides present certificates signed by a shared CA, so each node can verify the other's identity
during the TLS handshake.

See :doc:`tls` for how to generate certificates and configure ``tls-servers`` / ``tls-clients``.

Configuring pull connectivity
------------------------------

The following examples show a two-node pull setup: one control node and one execution node connecting to it.
The same pattern applies when the execution node connects to a hop node instead.

Control node
^^^^^^^^^^^^

The control node runs a listener with mTLS enabled.
It does **not** need a ``*-peer`` entry for the execution node.

.. tab-set::

   .. tab-item:: Version 2

      .. code-block:: yaml

         ---
         version: 2

         node:
           id: control

         log-level:
           level: info

         control-services:
           - service: control
             filename: /tmp/control.sock

         tls-servers:
           - name: control-server
             cert: /etc/receptor/tls/control.crt
             key: /etc/receptor/tls/control.key
             requireclientcert: true
             clientcas: /etc/receptor/tls/ca.crt

         tcp-listeners:
           - port: 27199
             tls: control-server

   .. tab-item:: Version 1

      .. code-block:: yaml

         ---
         - node:
            id: control

         - log-level: info

         - control-service:
            service: control
            filename: /tmp/control.sock

         - tls-server:
            name: control-server
            cert: /etc/receptor/tls/control.crt
            key: /etc/receptor/tls/control.key
            requireclientcert: true
            clientcas: /etc/receptor/tls/ca.crt

         - tcp-listener:
            port: 27199
            tls: control-server

Execution node — TCP pull
^^^^^^^^^^^^^^^^^^^^^^^^^

The execution node connects outbound to the control node using ``tcp-peer``.
``redial: true`` ensures the execution node reconnects automatically if the connection drops —
this is important in pull mode because the control plane cannot re-initiate the connection.

.. tab-set::

   .. tab-item:: Version 2

      .. code-block:: yaml

         ---
         version: 2

         node:
           id: execution-node-1

         log-level:
           level: info

         control-services:
           - service: control
             filename: /tmp/execution-node-1.sock

         tls-clients:
           - name: execution-client
             cert: /etc/receptor/tls/execution-node-1.crt
             key: /etc/receptor/tls/execution-node-1.key
             rootcas: /etc/receptor/tls/ca.crt
             insecureskipverify: false

         tcp-peers:
           - address: control.example.com:27199
             tls: execution-client
             redial: true

         work-commands:
           - worktype: bash
             command: bash

   .. tab-item:: Version 1

      .. code-block:: yaml

         ---
         - node:
            id: execution-node-1

         - log-level: info

         - control-service:
            service: control
            filename: /tmp/execution-node-1.sock

         - tls-client:
            name: execution-client
            cert: /etc/receptor/tls/execution-node-1.crt
            key: /etc/receptor/tls/execution-node-1.key
            rootcas: /etc/receptor/tls/ca.crt
            insecureskipverify: false

         - tcp-peer:
            address: control.example.com:27199
            tls: execution-client
            redial: true

         - work-command:
            worktype: bash
            command: bash

Execution node — WebSocket pull
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

When only HTTPS traffic is allowed out (for example through a corporate proxy),
use ``ws-peer`` instead of ``tcp-peer``.
The control node must also expose a ``ws-listener``.

Add to the control node config:

.. tab-set::

   .. tab-item:: Version 2

      .. code-block:: yaml

         ws-listeners:
           - port: 8443
             tls: control-server

   .. tab-item:: Version 1

      .. code-block:: yaml

         - ws-listener:
            port: 8443
            tls: control-server

Execution node using WebSocket:

.. tab-set::

   .. tab-item:: Version 2

      .. code-block:: yaml

         ---
         version: 2

         node:
           id: execution-node-2

         log-level:
           level: info

         control-services:
           - service: control
             filename: /tmp/execution-node-2.sock

         tls-clients:
           - name: execution-client
             cert: /etc/receptor/tls/execution-node-2.crt
             key: /etc/receptor/tls/execution-node-2.key
             rootcas: /etc/receptor/tls/ca.crt
             insecureskipverify: false

         ws-peers:
           - address: wss://control.example.com:8443/
             tls: execution-client
             redial: true

         work-commands:
           - worktype: bash
             command: bash

   .. tab-item:: Version 1

      .. code-block:: yaml

         ---
         - node:
            id: execution-node-2

         - log-level: info

         - control-service:
            service: control
            filename: /tmp/execution-node-2.sock

         - tls-client:
            name: execution-client
            cert: /etc/receptor/tls/execution-node-2.crt
            key: /etc/receptor/tls/execution-node-2.key
            rootcas: /etc/receptor/tls/ca.crt
            insecureskipverify: false

         - ws-peer:
            address: wss://control.example.com:8443/
            tls: execution-client
            redial: true

         - work-command:
            worktype: bash
            command: bash

Key configuration options
--------------------------

``redial``
  TCP and WebSocket ``*-peer`` entries enable ``redial`` by default.
  You can set it explicitly to ``true`` for clarity.
  If the connection is lost, the execution node will automatically reconnect to the control node.
  This is critical in pull mode because the control plane cannot re-initiate the connection.

``requireclientcert``
  Set to ``true`` on the control node's ``tls-server`` definition.
  Forces the execution node to present a valid certificate during the TLS handshake,
  preventing unauthorized nodes from joining the mesh.

``requireclientcert`` and the TLS configuration provide the mTLS protection required for a
production pull configuration. ``redial`` is already enabled by default.
