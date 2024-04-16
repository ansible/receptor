==============================
Receptor Configuration Options
==============================

---------------------
Receptor command line
---------------------

Command line arguments use the following syntax: ``receptor [--<action> [<param>=<value> ...] ...]``

The possible options for ``<action>`` are listed below.  Parameters for actions are listed in their own section.

----------------
Persistent Flags
----------------

.. list-table:: Persistent Flags
    :header-rows: 1
    :widths: auto

    * - Action
      - Description
    * - ``--bash-completion``
      - Generates a completion script for the Bash shell.
    * - ``--config <filename>``
      - Loads additional configuration options from a YAML file.
    * - ``--control-service``
      - Runs a control service.
    * - ``--help``
      - Show this help
    * - ``--local-only``
      - Runs a self-contained node with no backend.
    * - ``--log-level``
      - Specifies the verbosity level for command output.
    * - ``--node``
      - Specifies the node configuration of this instance.  This parameter is required.
    * - ``--trace``
      - Enables packet tracing output.
    * - ``--version``
      - Display the Receptor version.

^^^^^^^^^^^^^^^
Bash completion
^^^^^^^^^^^^^^^

To add Receptor auto-completion to the bash session: ``. <(receptor --bash-completion)``

^^^^^^^^^^^^^^^
Control Service
^^^^^^^^^^^^^^^

.. list-table:: Control Service
    :header-rows: 1
    :widths: auto

    * - Parameter
      -  Description
      -  Default value
    * - ``filename=<string>``
      - Specifies the filename of a local Unix socket to bind to the service.
      - No default value.
    * - ``permissions=<int>``
      - Socket file permissions
      - 0600
    * - ``service=<string>``
      - Receptor service name to listen on
      - control
    * - ``tls=<string>``
      - Name of TLS server config for the Receptor listener
      - No default value.
    * - ``tcplisten=<string>``
      - Local TCP port or host:port to bind to the control service
      - No default value.
    * - ``tcptls=<string>``
      - Name of TLS server config for the TCP listener
      - No default value.

^^^^^^^^^
Log level
^^^^^^^^^

.. list-table:: Log Level
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default value
    * - ``level=<string>``
      - Log level: Error, Warning, Info or Debug
      - Error

^^^^
Node
^^^^

.. list-table:: Node
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default value
    * - ``id=<string>``
      - Node ID
      - local hostname
    * - ``datadir=<string>``
      - Directory in which to store node data
      - /tmp/receptor
    * - ``firewallrules=<JSON list of JSON dict of JSON data to JSON data>``
      -  Firewall Rules. See :ref:`firewall_rules` for syntax
      - No default value.
    * - ``maxidleconnectiontimeout=<string>``
      - Max duration with no traffic before a backend connection is timed out and refreshed
      - No default value.

------------------------------------------
Configure resources used by other commands
------------------------------------------

.. list-table:: Configure resources used by other commands
    :header-rows: 1
    :widths: auto

    * - Action
      - Description
    * - ``--tls-client``
      - Define a TLS client configuration
    * - ``--tls-server``
      - Define a TLS server configuration

^^^^^^^^^^
TLS Client
^^^^^^^^^^

.. list-table:: TLS Client
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default value
    * - ``cert=<string>``
      - Client certificate filename (required)
      - No default value.
    * - ``insecureskipverify=<bool>``
      - Accept any server cert
      - false
    * - ``key=<string>``
      - Client private key filename (required)
      - No default value.
    * - ``mintls13=<bool>``
      - Set minimum TLS version to 1.3. Otherwise the minimum is 1.2
      - false
    * - ``name=<string>``
      - Name of this TLS client configuration (required)
      - No default value.
    * - ``pinnedservercert=<[]string (may be repeated)>``
      - Pinned fingerprint of required server certificate
      - No default value.
    * - ``rootcas=<string>``
      - Root CA bundle to use instead of system trust
      - No default value.
    * - ``skipreceptornamescheck=<bool>``
      - if true, skip verifying ReceptorNames OIDs in certificate at startup
      - No default value.

^^^^^^^^^^
TLS Server
^^^^^^^^^^

.. list-table:: TLS Server
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default value
    * - ``cert=<string>``
      - Server certificate filename (required)
      - No default value.
    * - ``clientcas=<string>``
      - Filename of CA bundle to verify client certs with
      - No default value.
    * - ``key=<string>``
      - Server private key filename (required)
      - No default value.
    * - ``mintls13=<bool>``
      - Set minimum TLS version to 1.3. Otherwise the minimum is 1.2
      - false
    * - ``name=<string>``
      - Name of this TLS server configuration (required)
      - No default value.
    * - ``pinnedclientcert=<[]string (may be repeated)>``
      - Pinned fingerprint of required client certificate
      - No default value.
    * - ``requireclientcert=<bool>``
      - Require client certificates
      - false
    * - ``skipreceptornamescheck=<bool>``
      - Skip verifying ReceptorNames OIDs in certificate at startup
      - false

----------------------------------------------------------------------
Commands to configure back-ends, which connect Receptor nodes together
----------------------------------------------------------------------

.. list-table:: Control Service
    :header-rows: 1
    :widths: auto

    * - Action
      - Description
    * - ``--tcp-listener``
      - Run a backend listener on a TCP port
    * - ``--tcp-peer``
      - Make an outbound backend connection to a TCP peer
    * - ``--udp-listener``
      - Run a backend listener on a UDP port
    * - ``--udp-peer``
      - Make an outbound backend connection to a UDP peer
    * - ``--ws-listener``
      - Run an http server that accepts websocket connections
    * - ``--ws-peer``
      - Connect outbound to a websocket peer

^^^^^^^^^^^^
TCP listener
^^^^^^^^^^^^

.. list-table:: TCP Listener
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default value
    * - ``allowedpeers=<[]string (may be repeated)>``
      - Peer node IDs to allow via this connection
      - No default value.
    * - ``bindaddr=<string>``
      - Local address to bind to
      - 0.0.0.0
    * - ``cost=<float64>``
      - Connection cost (weight)
      - 1.0
    * - ``nodecost=<JSON dict of string to float64>``
      - Per-node costs
      - No default value.
    * - ``port=<int>``
      - Local TCP port to listen on (required)
      - No default value.
    * - ``tls=<string>``
      - Name of TLS server config
      - No default value.

^^^^^^^^
TCP Peer
^^^^^^^^

.. list-table:: TCP Peer
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default value
    * - ``address=<string>``
      - Remote address (Host:Port) to connect to (required)
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
    * - ``tls=<string>``
      - Name of TLS client configuration
      - No default value.

^^^^^^^^^^^^
UDP Listener
^^^^^^^^^^^^

.. list-table:: UDP Listener
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default value
    * - ``allowedpeers=<[]string (may be repeated)>``
      - Peer node IDs to allow via this connection
      - No default value.
    * - ``bindaddr=<string>``
      - Local address to bind to
      - 0.0.0.0
    * - ``cost=<float64>``
      - Connection cost (weight)
      - 1.0
    * - ``nodecost=<JSON dict of string to float64>``
      - Per-node costs
      - No default value.
    * - ``port=<int>``
      - Local UDP port to listen on (required)
      - No default value.

^^^^^^^^
UDP Peer
^^^^^^^^

.. list-table:: UDP Peer
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

^^^^^^^^^^^^^^^^^^
Websocket Listener
^^^^^^^^^^^^^^^^^^

.. list-table:: Websocket Listener
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default value
    * - ``allowedpeers=<[]string (may be repeated)>``
      - Peer node IDs to allow via this connection
      - No default value.
    * - ``bindaddr=<string>``
      - Local address to bind to
      - 0.0.0.0
    * - ``cost=<float64>``
      - Connection cost (weight)
      - 1.0
    * - ``nodecost=<JSON dict of string to float64>``
      - Per-node costs
      - No default value.
    * - ``path=<string>``
      - URI path to the websocket server
      - \/
    * - ``port=<int>``
      - Local TCP port to run http server on (required)
      - No default value.
    * - ``tls=<string>``
      - Name of TLS server configuration
      - No default value.

^^^^^^^^^^^^^^
Websocket Peer
^^^^^^^^^^^^^^

.. list-table:: Websocket Peer
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default value
    * - ``address=<string>``
      - URL to connect to (required)
      - No default value.
    * - ``allowedpeers=<[]string (may be repeated)>``
      - Peer node IDs to allow via this connection
      - No default value.
    * - ``cost=<float64>``
      - Connection cost (weight)
      - 1.0
    * - ``extraheader=<string>``
      - Sends extra HTTP header on initial connection
      - No default value.
    * - ``redial=<bool>``
      - Keep redialing on lost connection
      - true
    * - ``tls=<string>``
      - Name of TLS client config
      - No default value.

-------------------------------------------------------
Configure services that run on top of the Receptor mesh
-------------------------------------------------------

.. list-table:: Configure services that run on top of the Receptor mesh
    :header-rows: 1
    :widths: auto

    * - Action
      - Description
    * - ``--command-service``
      - Run an interactive command via a Receptor service
    * - ``--ip-router``
      - Run an IP router using a tun interface
    * - ``--tcp-client``
      - Listen on a Receptor service and forward via TCP
    * - ``--tcp-server``
      - Listen for TCP and forward via Receptor
    * - ``--udp-client``
      - Listen on a Receptor service and forward via UDP
    * - ``--udp-server``
      - Listen for UDP and forward via Receptor
    * - ``--unix-socket-client``
      - Listen via Receptor and forward to a Unix socket
    * - ``--unix-socket-server``
      - Listen on a Unix socket and forward via Receptor

^^^^^^^^^^^^^^^
Command Service
^^^^^^^^^^^^^^^

.. list-table:: Command Service
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default value
    * - ``command=<string>``
      - Command to execute on a connection (required)
      - No default value.
    * - ``service=<string>``
      - Receptor service name to bind to (required)
      - No default value.
    * - ``tls=<string>``
      - Name of TLS server config
      - No default value.

^^^^^^^^^
IP Router
^^^^^^^^^

.. list-table:: IP Router
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default value
    * - ``interface=<string>``
      - Name of the local tun interface
      - No default value.
    * - ``localnet=<string>``
      - Local /30 CIDR address (required)
      - No default value.
    * - ``networkname=<string>``
      - Name of this network and service. (required)
      - No default value.
    * - ``routes=<string>``
      - Comma separated list of CIDR subnets to advertise
      - No default value.

^^^^^^^^^^
TCP Client
^^^^^^^^^^

.. list-table:: TCP Client
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default value
    * - ``address=<string>``
      - Address for outbound TCP connection (required)
      - No default value.
    * - ``service=<string>``
      - Receptor service name to bind to (required)
      - No default value.
    * - ``tlsserver=<string>``
      - Name of TLS server config for the Receptor service
      - No default value.
    * - ``tlsclient=<string>``
      - Name of TLS client config for the TCP connection
      - No default value.

^^^^^^^^^^
TCP Server
^^^^^^^^^^

.. list-table:: TCP Server
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default value
    * - ``bindaddr=<string>``
      - Address to bind TCP listener to
      - 0.0.0.0
    * - ``port=<int>``
      - Local TCP port to bind to (required)
      - No default value.
    * - ``remotenode=<string>``
      - Receptor node to connect to (required)
      - No default value.
    * - ``remoteservice=<string>``
      - Receptor service name to connect to (required)
      - No default value.
    * - ``tlsserver=<string>``
      - Name of TLS server config for the TCP listener
      - No default value.
    * - ``tlsclient=<string>``
      - Name of TLS client config for the Receptor connection
      - No default value.

^^^^^^^^^^
UDP Client
^^^^^^^^^^

.. list-table:: UDP Client
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default value
    * - ``address=<string>``
      - Address for outbound UDP connection (required)
      - No default value.
    * - ``service=<string>``
      - Receptor service name to bind to (required)
      - No default value.

^^^^^^^^^^
UDP Server
^^^^^^^^^^

.. list-table:: UDP Server
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default value
    * - ``bindaddr=<string>``
      - Address to bind UDP listener to
      - 0.0.0.0
    * - ``port=<int>``
      - Local UDP port to bind to (required)
      - No default value.
    * - ``remotenode=<string>``
      - Receptor node to connect to (required)
      - No default value.
    * - ``remoteservice=<string>``
      - Receptor service name to connect to (required)
      - No default value.

^^^^^^^^^^^^^^^^^^
Unix Socket Client
^^^^^^^^^^^^^^^^^^

.. list-table:: Unix Socket Client
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default value
    * - ``filename=<string>``
      - Socket filename, which must already exist (required)
      - No default value.
    * - ``service=<string>``
      - Receptor service name to bind to (required)
      - No default value.
    * - ``tls=<string>``
      - Name of TLS server config for the Receptor connection
      - No default value.

^^^^^^^^^^^^^^^^^^
Unix Socket Server
^^^^^^^^^^^^^^^^^^

.. list-table:: Unix Socket Server
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default value
    * - ``filename=<string>``
      - Socket filename, which will be overwritten (required)
      - No default value.
    * - ``permissions=<int>``
      - Socket file permissions
      - 0600
    * - ``remotenode=<string>``
      - Receptor node to connect to (required)
      - No default value.
    * - ``remoteservice=<string>``
      - Receptor service name to connect to (required)
      - No default value.
    * - ``tls=<string>``
      - Name of TLS client config for the Receptor connection
      - No default value.

--------------------------------------------
Configure workers that process units of work
--------------------------------------------

.. list-table:: Configure workers that process units of work
    :header-rows: 1
    :widths: auto

    * - Action
      - Description
    * - ``--work-command``
      - Run a worker using an external command
    * - ``--work-kubernetes``
      - Run a worker using Kubernetes
    * - ``--work-python``
      - Run a worker using a Python plugin
    * - ``--work-signing``
      - Private key to sign work submissions
    * - ``--work-verification``
      - Public key to verify work submissions

^^^^^^^^^^^^
Work Command
^^^^^^^^^^^^

.. list-table:: Work Command
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default value
    * - ``allowruntimeparams=<bool>``
      - Allow users to add more parameters
      - false
    * - ``command=<string>``
      - Command to run to process units of work (required)
      - No default value.
    * - ``params=<string>``
      - Command-line parameters
      - No default value.
    * - ``verifysignature=<bool>``
      - Verify a signed work submission
      - false
    * - ``worktype=<string>``
      - Name for this worker type (required)
      - No default value.

^^^^^^^^^^^^^^^
Work Kubernetes
^^^^^^^^^^^^^^^

.. list-table:: Work Kubernetes
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default value
    * - ``allowruntimeauth=<bool>``
      - Allow passing API parameters at runtime
      - false
    * - ``allowruntimecommand=<bool>``
      - Allow specifying image & command at runtime
      - false
    * - ``allowruntimeparams=<bool>``
      - Allow adding command parameters at runtime
      - false
    * - ``allowruntimepod=<bool>``
      - Allow passing Pod at runtime
      - false
    * - ``authmethod=<string>``
      - One of: kubeconfig, incluster
      - incluster
    * - ``command=<string>``
      - Command to run in the container (overrides entrypoint)
      - No default value.
    * - ``deletepodonrestart=<bool>``
      - On restart, delete the pod if in pending state
      - true
    * - ``image=<string>``
      - Container image to use for the worker pod
      - No default value.
    * - ``kubeconfig=<string>``
      - Kubeconfig filename (for authmethod=kubeconfig)
      - No default value.
    * - ``namespace=<string>``
      - Kubernetes namespace to create pods in
      - No default value.
    * - ``params=<string>``
      - Command-line parameters to pass to the entrypoint
      - No default value.
    * - ``pod=<string>``
      - Pod definition filename, in json or yaml format
      - No default value.
    * - ``streammethod=<string>``
      - Method for connecting to worker pods: logger or tcp
      - logger
    * - ``verifysignature=<bool>``
      - Verify a signed work submission
      - false
    * - ``worktype=<string>``
      - Name for this worker type (required)
      - No default value.

^^^^^^^^^^^
Work Python
^^^^^^^^^^^

.. list-table:: Work Python
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default value
    * - ``config=<JSON dict with string keys>``
      - Plugin-specific configuration
      - No default value.
    * - ``function=<string>``
      - Receptor-exported function to call (required)
      - No default value.
    * - ``plugin=<string>``
      - Python module name of the worker plugin (required)
      - No default value.
    * - ``worktype=<string>``
      - Name for this worker type (required)
      - No default value.

^^^^^^^^^^^^
Work Signing
^^^^^^^^^^^^

.. list-table:: Work Signing
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default value
    * - ``privatekey=<string>``
      - Private key to sign work submissions
      - No default value.
    * - ``tokenexpiration=<string>``
      - Expiration of the signed json web token, e.g. 3h or 3h30m
      - No default value.

^^^^^^^^^^^^^^^^^
Work Verification
^^^^^^^^^^^^^^^^^

.. list-table:: Work Verification
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default value
    * - ``publickey=<string>``
      - Public key to verify signed work submissions
      - No default value.

-----------------------------------------------------
Generate certificates and run a certificate authority
-----------------------------------------------------

.. list-table:: Generate certificates and run a certificate authority
    :header-rows: 1
    :widths: auto

    * - Action
      - Description
    * - ``--cert-init``
      - Initialize PKI CA
    * - ``--cert-makereq``
      - Create certificate request
    * - ``--cert-signreq``
      - Sign request and produce certificate

^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
Certificate Authority Initialization
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

.. list-table:: Certificate Authority Initialization
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default value
    * - ``bits=<int>``
      - Bit length of the encryption keys of the certificate (required)
      - No default value.
    * - ``commonname=<string>``
      - Common name to assign to the certificate (required)
      - No default value.
    * - ``notafter=<string>``
      - Expiration (NotAfter) date/time, in RFC3339 format
      - No default value.
    * - ``notbefore=<string>``
      - Effective (NotBefore) date/time, in RFC3339 format
      - No default value.
    * - ``outcert=<string>``
      - File to save the CA certificate to (required)
      - No default value.
    * - ``outkey=<string>``
      - File to save the CA private key to (required)
      - No default value.

^^^^^^^^^^^^^^^^^^^^^^^^^^
Create Certificate Request
^^^^^^^^^^^^^^^^^^^^^^^^^^

.. list-table:: Create Certificate Request
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default value
    * - ``bits=<int>``
      - Bit length of the encryption keys of the certificate
      - No default value.
    * - ``commonname=<string>``
      - Common name to assign to the certificate (required)
      - No default value.
    * - ``dnsname=<[]string (may be repeated)>``
      - DNS names to add to the certificate
      - No default value.
    * - ``inkey=<string>``
      - Private key to use for the request
      - No default value.
    * - ``ipaddress=<[]string (may be repeated)>``
      - IP addresses to add to the certificate
      - No default value.
    * - ``nodeid=<[]string (may be repeated)>``
      - Receptor node IDs to add to the certificate
      - No default value.
    * - ``outreq=<string>``
      - File to save the certificate request to (required)
      - No default value.
    * - ``outkey=<string>``
      - File to save the private key to (new key will be generated)
      - No default value.

^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
Sign Request and Produce Certificate
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

.. list-table:: Sign Request and Produce Certificate
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default value
    * - ``cacert=<string>``
      - CA certificate PEM filename (required)
      - No default value.
    * - ``cakey=<string>``
      - CA private key PEM filename (required)
      - No default value.
    * - ``notafter=<string>``
      - Expiration (NotAfter) date/time, in RFC3339 format
      - No default value.
    * - ``notbefore=<string>``
      - Effective (NotBefore) date/time, in RFC3339 format
      - No default value.
    * - ``outcert=<string>``
      - File to save the signed certificate to (required)
      - No default value.
    * - ``req=<string>``
      - Certificate Request PEM filename (required)
      - No default value.
    * - ``verify=<bool>``
      - If true, do not prompt the user for verification
      - False
