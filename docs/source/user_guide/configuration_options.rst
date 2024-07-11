==============================
Receptor Configuration Options
==============================

.. ---------------------
.. Receptor yaml config
.. ---------------------

.. Command line arguments use the following syntax: ``receptor [--<action> [<param>=<value> ...] ...]``

.. The possible options for ``<action>`` are listed below.  Parameters for actions are listed in their own section.

----------------
Persistent Flags
----------------

.. list-table:: Persistent Flags
    :header-rows: 1
    :widths: auto

    * - Action
      - Description
    * - ``--config <filename>``
      - Loads configuration options from a YAML file.
    * - ``--version``
      - Display the Receptor version.
    * - ``--help``
      - Show this help

^^^^^^^^^^^^^^^^
Control Services
^^^^^^^^^^^^^^^^

.. list-table:: Control Service (List item)
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default value
      - Type
    * - ``filename``
      - Specifies the filename of a local Unix socket to bind to the service.
      - No default value.
      - string
    * - ``permissions``
      - Socket file permissions
      - 0600
      - int
    * - ``service``
      - Receptor service name to listen on
      - control
      - string
    * - ``tls``
      - Name of TLS server config for the Receptor listener
      - No default value.
      - string
    * - ``tcplisten``
      - Local TCP port or host:port to bind to the control service
      - No default value.
      - string
    * - ``tcptls``
      - Name of TLS server config for the TCP listener
      - No default value.
      - string

.. code-block:: yaml

    control-services:
      - service: foo
        filename: /tmp/foo.sock

^^^^^^^^^
Log level
^^^^^^^^^

.. list-table:: Log Level
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default value
      - Type
    * - ``level``
      - Log level: Error, Warning, Info or Debug
      - Error
      - string

.. code-block:: yaml

  log-level:
    level: debug

^^^^
Node
^^^^

.. list-table:: Node
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default value
      - Type
    * - ``id``
      - Node ID
      - local hostname
      - string
    * - ``datadir``
      - Directory in which to store node data
      - /tmp/receptor
      - string
    * - ``firewallrules``
      -  Firewall Rules. See :ref:`firewall_rules` for syntax
      - No default value.
      - JSON
    * - ``maxidleconnectiontimeout``
      - Max duration with no traffic before a backend connection is timed out and refreshed
      - No default value.
      - string


.. code-block:: yaml

    node:
      id: foo

------------------------------------------
Configure resources used by other commands
------------------------------------------

.. .. list-table:: Configure resources used by other commands
..     :header-rows: 1
..     :widths: auto

..     * - Action
..       - Description
..     * - ``--tls-client``
..       - Define a TLS client configuration
..     * - ``--tls-server``
..       - Define a TLS server configuration

^^^^^^^^^^^
TLS Clients
^^^^^^^^^^^

.. list-table:: TLS Client (List item)
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default value
      - Type
    * - ``cert``
      - Client certificate filename (required)
      - No default value.
      - string
    * - ``insecureskipverify``
      - Accept any server cert
      - false
      - bool
    * - ``key``
      - Client private key filename (required)
      - No default value.
      - string
    * - ``mintls13``
      - Set minimum TLS version to 1.3. Otherwise the minimum is 1.2
      - false
      - bool
    * - ``name``
      - Name of this TLS client configuration (required)
      - No default value.
      - string
    * - ``pinnedservercert``
      - Pinned fingerprint of required server certificate
      - No default value.
      - list of string
    * - ``rootcas``
      - Root CA bundle to use instead of system trust
      - No default value.
      - string
    * - ``skipreceptornamescheck``
      - if true, skip verifying ReceptorNames OIDs in certificate at startup
      - No default value.
      - bool

.. code-block:: yaml

    tls-clients:
      - name: tlsclient
        cert: /tmp/certs/foo.crt
        key: /tmp/certs/key.crt

^^^^^^^^^^^
TLS Servers
^^^^^^^^^^^

.. list-table:: TLS Server (List item)
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default value
      - Type
    * - ``cert``
      - Server certificate filename (required)
      - No default value.
      - string
    * - ``clientcas``
      - Filename of CA bundle to verify client certs with
      - No default value.
      - string
    * - ``key``
      - Server private key filename (required)
      - No default value.
      - string
    * - ``mintls13``
      - Set minimum TLS version to 1.3. Otherwise the minimum is 1.2
      - false
      - bool
    * - ``name``
      - Name of this TLS server configuration (required)
      - No default value.
      - string
    * - ``pinnedclientcert``
      - Pinned fingerprint of required client certificate
      - No default value.
      - list of string
    * - ``requireclientcert``
      - Require client certificates
      - false
      - bool
    * - ``skipreceptornamescheck``
      - Skip verifying ReceptorNames OIDs in certificate at startup
      - false
      - bool

.. code-block:: yaml

    tls-servers:
      - name: tlsserver
        cert: /tmp/certs/foo.crt
        key: /tmp/certs/key.crt

----------------------------------------------------------------------
Options to configure back-ends, which connect Receptor nodes together
----------------------------------------------------------------------

.. .. list-table:: Control Service
..     :header-rows: 1
..     :widths: auto

..     * - Action
..       - Description
..     * - ``--tcp-listener``
..       - Run a backend listener on a TCP port
..     * - ``--tcp-peer``
..       - Make an outbound backend connection to a TCP peer
..     * - ``--udp-listener``
..       - Run a backend listener on a UDP port
..     * - ``--udp-peer``
..       - Make an outbound backend connection to a UDP peer
..     * - ``--ws-listener``
..       - Run an http server that accepts websocket connections
..     * - ``--ws-peer``
..       - Connect outbound to a websocket peer

^^^^^^^^^^^^^
TCP listeners
^^^^^^^^^^^^^

.. list-table:: TCP Listener (List item)
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default value
      - Type
    * - ``allowedpeers``
      - Peer node IDs to allow via this connection
      - No default value.
      - list of string
    * - ``bindaddr``
      - Local address to bind to
      - 0.0.0.0
      - string
    * - ``cost``
      - Connection cost (weight)
      - 1.0
      - float64
    * - ``nodecost``
      - Per-node costs
      - No default value.
      - float64
    * - ``port``
      - Local TCP port to listen on (required)
      - No default value.
      - int
    * - ``tls``
      - Name of TLS server config
      - No default value.
      - string

.. code-block:: yaml

    tcp-listeners:
      - port: 2223

^^^^^^^^^
TCP Peers
^^^^^^^^^

.. list-table:: TCP Peer
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default value
      - Type
    * - ``address``
      - Remote address (Host:Port) to connect to (required)
      - No default value.
      - string
    * - ``allowedpeers``
      - Peer node IDs to allow via this connection
      - No default value.
      - list of string
    * - ``cost``
      - Connection cost (weight)
      - 1.0
      - float64
    * - ``redial``
      - Keep redialing on lost connection
      - true
      - bool
    * - ``tls``
      - Name of TLS client configuration
      - No default value.
      - string

.. code-block:: yaml

    tcp-peers:
      - address: localhost:2223


^^^^^^^^^^^^^
UDP Listeners
^^^^^^^^^^^^^

.. list-table:: UDP Listener (List item)
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default value
      - Type
    * - ``allowedpeers``
      - Peer node IDs to allow via this connection
      - No default value.
      - list of string
    * - ``bindaddr``
      - Local address to bind to
      - 0.0.0.0
      - string
    * - ``cost``
      - Connection cost (weight)
      - 1.0
      - float64
    * - ``nodecost``
      - Per-node costs
      - No default value.
      - float64
    * - ``port``
      - Local UDP port to listen on (required)
      - No default value.
      - int

.. code-block:: yaml

    tcp-listeners:
      - port: 2223

^^^^^^^^^
UDP Peers
^^^^^^^^^

.. list-table:: UDP Peer (List item)
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default value
    * - ``address=<string>``
      - Host:Port to connect to (required)
      - No default value.
    * - ``allowedpeers=<[]string (may be repeated)>``
      - Peer node IDs to allow via this connection
      - No default value.
    * - ``cost=<float64>``
      - Connection cost (weight)
      - 1.0
    * - ``redial=<bool>``
      - Keep redialing on lost connection
      - true

.. code-block:: yaml

    udp-peers:
      - address: localhost:2223

^^^^^^^^^^^^^^^^^^^
Websocket Listeners
^^^^^^^^^^^^^^^^^^^

.. list-table:: Websocket Listener
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default value
      - Type
    * - ``allowedpeers``
      - Peer node IDs to allow via this connection
      - No default value.
      - list of string
    * - ``bindaddr``
      - Local address to bind to
      - 0.0.0.0
      - string
    * - ``cost``
      - Connection cost (weight)
      - 1.0
      - float64
    * - ``nodecost``
      - Per-node costs
      - No default value.
      - float64
    * - ``path``
      - URI path to the websocket server
      - \/
      - string
    * - ``port``
      - Local TCP port to run http server on (required)
      - No default value.
      - int
    * - ``tls``
      - Name of TLS server configuration
      - No default value.
      - string

.. code-block:: yaml

    ws-listeners:
      - port: 27198

^^^^^^^^^^^^^^^
Websocket Peers
^^^^^^^^^^^^^^^

.. list-table:: Websocket Peer (List item)
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default value
      - Type
    * - ``address``
      - URL to connect to (required)
      - No default value.
      - string
    * - ``allowedpeers``
      - Peer node IDs to allow via this connection
      - No default value.
      - list of string
    * - ``cost``
      - Connection cost (weight)
      - 1.0
      - float64
    * - ``extraheader``
      - Sends extra HTTP header on initial connection
      - No default value.
      - string
    * - ``redial``
      - Keep redialing on lost connection
      - true
      - bool
    * - ``tls``
      - Name of TLS client config
      - No default value.
      - string

.. code-block:: yaml

    ws-peers:
      - address: ws://localhost:27198

-------------------------------------------------------
Configure services that run on top of the Receptor mesh
-------------------------------------------------------

.. .. list-table:: Configure services that run on top of the Receptor mesh
..     :header-rows: 1
..     :widths: auto

..     * - Action
..       - Description
..     * - ``--command-service``
..       - Run an interactive command via a Receptor service
..     * - ``--ip-router``
..       - Run an IP router using a tun interface
..     * - ``--tcp-client``
..       - Listen on a Receptor service and forward via TCP
..     * - ``--tcp-server``
..       - Listen for TCP and forward via Receptor
..     * - ``--udp-client``
..       - Listen on a Receptor service and forward via UDP
..     * - ``--udp-server``
..       - Listen for UDP and forward via Receptor
..     * - ``--unix-socket-client``
..       - Listen via Receptor and forward to a Unix socket
..     * - ``--unix-socket-server``
..       - Listen on a Unix socket and forward via Receptor

.. ^^^^^^^^^^^^^^^^
.. Command Services
.. ^^^^^^^^^^^^^^^^

.. .. list-table:: Command Service (List item)
..     :header-rows: 1
..     :widths: auto

..     * - Parameter
..       - Description
..       - Default value
..       - Type
..     * - ``command``
..       - Command to execute on a connection (required)
..       - No default value.
..       - string
..     * - ``service``
..       - Receptor service name to bind to (required)
..       - No default value.
..       - string
..     * - ``tls``
..       - Name of TLS server config
..       - No default value.
..       - string
.. .. code-block:: yaml

..     command-servies:
..       - command: cat
..         service: foo

^^^^^^^^^^
IP Routers
^^^^^^^^^^

.. list-table:: IP Router (List item)
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default value
      - Type
    * - ``interface``
      - Name of the local tun interface
      - No default value.
      - string
    * - ``localnet``
      - Local /30 CIDR address (required)
      - No default value.
      - string
    * - ``networkname``
      - Name of this network and service. (required)
      - No default value.
      - string
    * - ``routes``
      - Comma separated list of CIDR subnets to advertise
      - No default value.
      - string

.. code-block:: yaml

    ip-routers:
      - networkname: hello
        localnet: abc

^^^^^^^^^^^
TCP Clients
^^^^^^^^^^^

.. list-table:: TCP Client (List item)
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default value
    * - ``address``
      - Address for outbound TCP connection (required)
      - No default value.
    * - ``service``
      - Receptor service name to bind to (required)
      - No default value.
    * - ``tlsserver``
      - Name of TLS server config for the Receptor service
      - No default value.
    * - ``tlsclient``
      - Name of TLS client config for the TCP connection
      - No default value.

.. code-block:: yaml

    tcp-clients:
      - address: localhost:2223
        service: foo

^^^^^^^^^^^
TCP Servers
^^^^^^^^^^^

.. list-table:: TCP Server (List item)
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default value
      - Type
    * - ``bindaddr``
      - Address to bind TCP listener to
      - 0.0.0.0
      - string
    * - ``port``
      - Local TCP port to bind to (required)
      - No default value.
      - int
    * - ``remotenode``
      - Receptor node to connect to (required)
      - No default value.
      - string
    * - ``remoteservice``
      - Receptor service name to connect to (required)
      - No default value.
      - string
    * - ``tlsserver``
      - Name of TLS server config for the TCP listener
      - No default value.
      - string
    * - ``tlsclient``
      - Name of TLS client config for the Receptor connection
      - No default value.
      - string

.. code-block:: yaml

    tcp-clients:
      - port: 2223
        remotenode: foo
        remoteservice: foo


^^^^^^^^^^^
UDP Clients
^^^^^^^^^^^

.. list-table:: UDP Client (List item)
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default value
      - Type
    * - ``address``
      - Address for outbound UDP connection (required)
      - No default value.
      - string
    * - ``service``
      - Receptor service name to bind to (required)
      - No default value.
      - string

.. code-block:: yaml

    udp-clients:
      - address: localhost:2223
        service: foo


^^^^^^^^^^^
UDP Servers
^^^^^^^^^^^

.. list-table:: UDP Server (List item)
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default value
      - Type
    * - ``bindaddr``
      - Address to bind UDP listener to
      - 0.0.0.0
      - string
    * - ``port``
      - Local UDP port to bind to (required)
      - No default value.
      - int
    * - ``remotenode``
      - Receptor node to connect to (required)
      - No default value.
      - string
    * - ``remoteservice``
      - Receptor service name to connect to (required)
      - No default value.
      - string

.. code-block:: yaml

    udp-servers:
      - address: 2223
        remotenode: foo
        remoteservice: foo


^^^^^^^^^^^^^^^^^^^
Unix Socket Clients
^^^^^^^^^^^^^^^^^^^

.. list-table:: Unix Socket Client (List item)
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default value
      - Type
    * - ``filename``
      - Socket filename, which must already exist (required)
      - No default value.
      - string
    * - ``service``
      - Receptor service name to bind to (required)
      - No default value.
      - string
    * - ``tls``
      - Name of TLS server config for the Receptor connection
      - No default value.
      - string

.. code-block:: yaml

    unix-socket-clients:
      - filename: /tmp/foo.sock
        service: foo


^^^^^^^^^^^^^^^^^^^
Unix Socket Servers
^^^^^^^^^^^^^^^^^^^

.. list-table:: Unix Socket Server (List item)
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default value
      - Type
    * - ``filename``
      - Socket filename, which will be overwritten (required)
      - No default value.
      - string
    * - ``permissions``
      - Socket file permissions
      - 0600
      - int
    * - ``remotenode``
      - Receptor node to connect to (required)
      - No default value.
      - string
    * - ``remoteservice``
      - Receptor service name to connect to (required)
      - No default value.
      - string
    * - ``tls``
      - Name of TLS client config for the Receptor connection
      - No default value.
      - string

.. code-block:: yaml

    unix-socket-servers:
      - filename: /tmp/foo.sock
        remotenode: foo
        remoteservice: foo


--------------------------------------------
Configure workers that process units of work
--------------------------------------------

.. .. list-table:: Configure workers that process units of work
..     :header-rows: 1
..     :widths: auto

..     * - Action
..       - Description
..     * - ``--work-command``
..       - Run a worker using an external command
..     * - ``--work-kubernetes``
..       - Run a worker using Kubernetes
..     * - ``--work-python``
..       - Run a worker using a Python plugin
..     * - ``--work-signing``
..       - Private key to sign work submissions
..     * - ``--work-verification``
..       - Public key to verify work submissions

^^^^^^^^^^^^^
Work Commands
^^^^^^^^^^^^^

.. list-table:: Work Command (List item)
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default value
      - Type
    * - ``allowruntimeparams``
      - Allow users to add more parameters
      - false
      - bool
    * - ``command``
      - Command to run to process units of work (required)
      - No default value.
      - string
    * - ``params``
      - Command-line parameters
      - No default value.
      - string
    * - ``verifysignature``
      - Verify a signed work submission
      - false
      - bool
    * - ``worktype``
      - Name for this worker type (required)
      - No default value.
      - string

.. code-block:: yaml

    work-commands:
      - command: cat
        worktype: cat


^^^^^^^^^^^^^^^
Work Kubernetes
^^^^^^^^^^^^^^^

.. list-table:: Work Kubernetes
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default value
      - Type
    * - ``allowruntimeauth``
      - Allow passing API parameters at runtime
      - false
      - bool
    * - ``allowruntimecommand``
      - Allow specifying image & command at runtime
      - false
      - bool
    * - ``allowruntimeparams``
      - Allow adding command parameters at runtime
      - false
      - bool
    * - ``allowruntimepod``
      - Allow passing Pod at runtime
      - false
      - bool
    * - ``authmethod``
      - One of: kubeconfig, incluster
      - incluster
      - string
    * - ``command``
      - Command to run in the container (overrides entrypoint)
      - No default value.
      - string
    * - ``deletepodonrestart``
      - On restart, delete the pod if in pending state
      - true
      - bool
    * - ``image``
      - Container image to use for the worker pod
      - No default value.
      - string
    * - ``kubeconfig``
      - Kubeconfig filename (for authmethod=kubeconfig)
      - No default value.
      - string
    * - ``namespace``
      - Kubernetes namespace to create pods in
      - No default value.
      - string
    * - ``params``
      - Command-line parameters to pass to the entrypoint
      - No default value.
      - string
    * - ``pod``
      - Pod definition filename, in json or yaml format
      - No default value.
      - string
    * - ``streammethod``
      - Method for connecting to worker pods: logger or tcp
      - logger
      - string
    * - ``verifysignature``
      - Verify a signed work submission
      - false
      - bool
    * - ``worktype``
      - Name for this worker type (required)
      - No default value.
      - string

.. code-block:: yaml

    work-kubernetes:
      - worktype: cat

.. ^^^^^^^^^^^
.. Work Python
.. ^^^^^^^^^^^

.. .. list-table:: Work Python
..     :header-rows: 1
..     :widths: auto

..     * - Parameter
..       - Description
..       - Default value
..       - Type
..     * - ``config``
..       - Plugin-specific configuration
..       - No default value.
..       - JSON
..     * - ``function``
..       - Receptor-exported function to call (required)
..       - No default value.
..       - string
..     * - ``plugin``
..       - Python module name of the worker plugin (required)
..       - No default value.
..       - string
..     * - ``worktype``
..       - Name for this worker type (required)
..       - No default value.
..       - string

.. .. code-block:: yaml

..     tcp-clients:
..       - address: localhost:2223
..         service: foo


^^^^^^^^^^^^
Work Signing
^^^^^^^^^^^^

.. list-table:: Work Signing
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default value
      - Type
    * - ``privatekey``
      - Private key to sign work submissions
      - No default value.
      - string
    * - ``tokenexpiration``
      - Expiration of the signed json web token, e.g. 3h or 3h30m
      - No default value.
      - string

.. code-block:: yaml

    work-signing:
      privatekey: /tmp/signworkprivate.pem
      tokenexpiration: 30m


^^^^^^^^^^^^^^^^^
Work Verification
^^^^^^^^^^^^^^^^^

.. list-table:: Work Verification
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default value
      - Type
    * - ``publickey``
      - Public key to verify signed work submissions
      - No default value.
      - string

.. code-block:: yaml

    work-verification:
      publickey: /tmp/signworkpublic.pem


-----------------------------------------------------
Generate certificates and run a certificate authority
-----------------------------------------------------

.. .. list-table:: Generate certificates and run a certificate authority
..     :header-rows: 1
..     :widths: auto

..     * - Action
..       - Description
..     * - ``--cert-init``
..       - Initialize PKI CA
..     * - ``--cert-makereq``
..       - Create certificate request
..     * - ``--cert-signreq``
..       - Sign request and produce certificate

^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
Certificate Authority Initialization
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

.. list-table:: Certificate Authority Initialization
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default value
      - Type
    * - ``bits``
      - Bit length of the encryption keys of the certificate (required)
      - No default value.
      - int
    * - ``commonname``
      - Common name to assign to the certificate (required)
      - No default value.
      - string
    * - ``notafter``
      - Expiration (NotAfter) date/time, in RFC3339 format
      - No default value.
      - string
    * - ``notbefore``
      - Effective (NotBefore) date/time, in RFC3339 format
      - No default value.
      - string
    * - ``outcert``
      - File to save the CA certificate to (required)
      - No default value.
      - string
    * - ``outkey``
      - File to save the CA private key to (required)
      - No default value.
      - string

.. code-block:: yaml

    cert-init:
      commonname: test CA
      bits: 2048
      outcert: /tmp/certs/ca.crt
      outkey: /tmp/certs/ca.key


^^^^^^^^^^^^^^^^^^^^^^^^^^^
Create Certificate Requests
^^^^^^^^^^^^^^^^^^^^^^^^^^^

.. list-table:: Create Certificate Request (List item)
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default value
      - Type
    * - ``bits``
      - Bit length of the encryption keys of the certificate
      - No default value.
      - int
    * - ``commonname``
      - Common name to assign to the certificate (required)
      - No default value.
      - string
    * - ``dnsname``
      - DNS names to add to the certificate
      - No default value.
      - list of string
    * - ``inkey``
      - Private key to use for the request
      - No default value.
      - string
    * - ``ipaddress``
      - IP addresses to add to the certificate
      - No default value.
      - list of string
    * - ``nodeid``
      - Receptor node IDs to add to the certificate
      - No default value.
      - list of string
    * - ``outreq``
      - File to save the certificate request to (required)
      - No default value.
      - string
    * - ``outkey``
      - File to save the private key to (new key will be generated)
      - No default value.
      - string

.. code-block:: yaml

    cert-makereqs:
      - address: localhost:2223
        service: foo


^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
Sign Request and Produce Certificate
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

.. list-table:: Sign Request and Produce Certificate
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default value
      - Type
    * - ``cacert``
      - CA certificate PEM filename (required)
      - No default value.
      - string
    * - ``cakey``
      - CA private key PEM filename (required)
      - No default value.
      - string
    * - ``notafter``
      - Expiration (NotAfter) date/time, in RFC3339 format
      - No default value.
      - string
    * - ``notbefore``
      - Effective (NotBefore) date/time, in RFC3339 format
      - No default value.
      - string
    * - ``outcert``
      - File to save the signed certificate to (required)
      - No default value.
      - string
    * - ``req``
      - Certificate Request PEM filename (required)
      - No default value.
      - string
    * - ``verify``
      - If true, do not prompt the user for verification
      - False
      - bool

.. code-block:: yaml

    tcp-clients:
      - address: localhost:2223
        service: foo

