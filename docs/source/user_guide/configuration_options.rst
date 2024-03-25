==============================
Receptor Configuration Options
==============================

---------------------
Receptor command line
---------------------

Command line arguments have the form: receptor [--<action> [<param>=<value> ...] ...]

<action> are listed below.  When an action has parameters, they are listed in their own section.

-------------
Miscellaneous
-------------

.. list-table:: Miscellaneous
    :header-rows: 1
    :widths: auto

    * - Action
      - Description
    * - --bash-completion
      - Generate a completion script for the bash shell
    * - --config <filename>
      - Load additional config options from a YAML file
    * - --control-service
      - Run a control service
    * - --help
      - Show this help
    * - --local-only
      - Run a self-contained node with no backends
    * - --log-level
      - Set specific log level output
    * - --node
      - Node configuration of this instance (required)
    * - --trace
      - Enables packet tracing output
    * - --version
      - Show the Receptor version

^^^^^^^^^^^^^^^
Bash completion
^^^^^^^^^^^^^^^

Run ". <(receptor --bash-completion)" to activate now

^^^^^^^^^^^^^^^
Control Service
^^^^^^^^^^^^^^^

.. list-table:: Control Service
    :header-rows: 1
    :widths: auto

    * - Parameter
      -  Description
      -  Default
    * - filename=<string>
      - Filename of local Unix socket to bind to the service
      -
    * - permissions=<int>
      - Socket file permissions
      - 0600
    * - service=<string>
      - Receptor service name to listen on
      - control
    * - tls=<string>
      - Name of TLS server config for the Receptor listener
      -
    * - tcplisten=<string>
      - Local TCP port or host:port to bind to the control service
      -
    * - tcptls=<string>
      - Name of TLS server config for the TCP listener
      -

^^^^^^^^^
Log level
^^^^^^^^^

.. list-table:: Log Level
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default
    * - level=<string>
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
      - Default
    * - id=<string>
      - Node ID
      - local hostname
    * - datadir=<string>
      - Directory in which to store node data
      -
    * - firewallrules=<JSON list of JSON dict of JSON data to JSON data>
      -  Firewall Rules (see documentation for syntax)
      -
    * - maxidleconnectiontimeout=<string>
      - Max duration with no traffic before a backend connection is timed out and refreshed
      -

------------------------------------------
Configure resources used by other commands
------------------------------------------

.. list-table:: Configure resources used by other commands
    :header-rows: 1
    :widths: auto

    * - Action
      - Description
    * - --tls-client
      - Define a TLS client configuration
    * - --tls-server
      - Define a TLS server configuration

^^^^^^^^^^
TLS Client
^^^^^^^^^^

.. list-table:: TLS Client
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default
    * - cert=<string>
      - Client certificate filename
      -
    * - insecureskipverify=<bool>
      - Accept any server cert
      - false
    * - key=<string>
      - Client private key filename
      -
    * - mintls13=<bool>
      - Set minimum TLS version to 1.3. Otherwise the minimum is 1.2
      - false
    * - name=<string>
      - Name of this TLS client configuration (required)
      -
    * - pinnedservercert=<[]string (may be repeated)>
      - Pinned fingerprint of required server certificate
      -
    * - rootcas=<string>
      - Root CA bundle to use instead of system trust
      -
    * - skipreceptornamescheck=<bool>
      - if true, skip verifying ReceptorNames OIDs in certificate at startup
      -

^^^^^^^^^^
TLS Server
^^^^^^^^^^

.. list-table:: TLS Server
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default
    * - cert=<string>
      - Server certificate filename (required)
      -
    * - clientcas=<string>
      - Filename of CA bundle to verify client certs with
      -
    * - key=<string>
      - Server private key filename (required)
      -
    * - mintls13=<bool>
      - Set minimum TLS version to 1.3. Otherwise the minimum is 1.2
      - false
    * - name=<string>
      - Name of this TLS server configuration (required)
      -
    * - pinnedclientcert=<[]string (may be repeated)>
      - Pinned fingerprint of required client certificate
      -
    * - requireclientcert=<bool>
      - Require client certificates
      - false
    * - skipreceptornamescheck=<bool>
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
    * - --tcp-listener
      - Run a backend listener on a TCP port
    * - --tcp-peer
      - Make an outbound backend connection to a TCP peer
    * - --udp-listener
      - Run a backend listener on a UDP port
    * - --udp-peer
      - Make an outbound backend connection to a UDP peer
    * - --ws-listener
      - Run an http server that accepts websocket connections
    * - --ws-peer
      - Connect outbound to a websocket peer

^^^^^^^^^^^^
TCP listener
^^^^^^^^^^^^

.. list-table:: TCP Listener
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default
    * - allowedpeers=<[]string (may be repeated)>
      - Peer node IDs to allow via this connection
      -
    * - bindaddr=<string>
      - Local address to bind to
      - 0.0.0.0
    * - cost=<float64>
      - Connection cost (weight)
      - 1.0
    * - nodecost=<JSON dict of string to float64>
      - Per-node costs
      -
    * - port=<int>
      - Local TCP port to listen on (required)
      -
    * - tls=<string>
      - Name of TLS server config
      -

^^^^^^^^
TCP Peer
^^^^^^^^

.. list-table:: TCP Peer
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default
    * - address=<string>
      - Remote address (Host:Port) to connect to (required)
      -
    * - allowedpeers=<[]string (may be repeated)>
      - Peer node IDs to allow via this connection
      -
    * - cost=<float64>
      - Connection cost (weight)
      - 1.0
    * - redial=<bool>
      - Keep redialing on lost connection
      - true
    * - tls=<string>
      - Name of TLS client configuration
      -

^^^^^^^^^^^^
UDP Listener
^^^^^^^^^^^^

.. list-table:: UDP Listener
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default
    * - allowedpeers=<[]string (may be repeated)>
      - Peer node IDs to allow via this connection
      -
    * - bindaddr=<string>
      - Local address to bind to
      - 0.0.0.0
    * - cost=<float64>
      - Connection cost (weight)
      - 1.0
    * - nodecost=<JSON dict of string to float64>
      - Per-node costs
      -
    * - port=<int>
      - Local UDP port to listen on (required)
      -

^^^^^^^^
UDP Peer
^^^^^^^^

.. list-table:: UDP Peer
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default
    * - address=<string>
      - Host:Port to connect to (required)
      -
    * - allowedpeers=<[]string (may be repeated)>
      - Peer node IDs to allow via this connection
      -
    * - cost=<float64>
      - Connection cost (weight)
      - 1.0
    * - redial=<bool>
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
      - Default
    * - allowedpeers=<[]string (may be repeated)>
      - Peer node IDs to allow via this connection
      -
    * - bindaddr=<string>
      - Local address to bind to
      - 0.0.0.0
    * - cost=<float64>
      - Connection cost (weight)
      - 1.0
    * - nodecost=<JSON dict of string to float64>
      - Per-node costs
      -
    * - path=<string>
      - URI path to the websocket server
      - \/
    * - port=<int>
      - Local TCP port to run http server on (required)
      -
    * - tls=<string>
      - Name of TLS server configuration
      -

^^^^^^^^^^^^^^
Websocket Peer
^^^^^^^^^^^^^^

.. list-table:: Websocket Peer
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default
    * - address=<string>
      - URL to connect to (required)
      -
    * - allowedpeers=<[]string (may be repeated)>
      - Peer node IDs to allow via this connection
      -
    * - cost=<float64>
      - Connection cost (weight)
      - 1.0
    * - extraheader=<string>
      - Sends extra HTTP header on initial connection
      -
    * - redial=<bool>
      - Keep redialing on lost connection
      - true
    * - tls=<string>
      - Name of TLS client config
      -

-------------------------------------------------------
Configure services that run on top of the Receptor mesh
-------------------------------------------------------

.. list-table:: Configure serivces that run on top of the Receptor mesh
    :header-rows: 1
    :widths: auto

    * - Action
      - Description
    * - --command-service
      - Run an interactive command via a Receptor service
    * - --ip-router
      - Run an IP router using a tun interface
    * - --tcp-client
      - Listen on a Receptor service and forward via TCP
    * - --tcp-server
      - Listen for TCP and forward via Receptor
    * - --udp-client
      - Listen on a Receptor service and forward via UDP
    * - --udp-server
      - Listen for UDP and forward via Receptor
    * - --unix-socket-client
      - Listen via Receptor and forward to a Unix socket
    * - --unix-socket-server
      - Listen on a Unix socket and forward via Receptor

^^^^^^^^^^^^^^^
Command Service
^^^^^^^^^^^^^^^

.. list-table:: Command Service
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default
    * - command=<string>
      - Command to execute on a connection (required)
      -
    * - service=<string>
      - Receptor service name to bind to (required)
      -
    * - tls=<string>
      - Name of TLS server config
      -

^^^^^^^^^
IP Router
^^^^^^^^^

.. list-table:: IP Router
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default
    * - interface=<string>
      - Name of the local tun interface
      -
    * - localnet=<string>
      - Local /30 CIDR address (required)
      -
    * - networkname=<string>
      - Name of this network and service. (required)
      -
    * - routes=<string>
      - Comma separated list of CIDR subnets to advertise
      -

^^^^^^^^^^
TCP Client
^^^^^^^^^^

.. list-table:: TCP Client
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default
    * - address=<string>
      - Address for outbound TCP connection (required)
      -
    * - service=<string>
      - Receptor service name to bind to (required)
      -
    * - tlsserver=<string>
      - Name of TLS server config for the Receptor service
      -
    * - tlsclient=<string>
      - Name of TLS client config for the TCP connection
      -

^^^^^^^^^^
TCP Server
^^^^^^^^^^

.. list-table:: TCP Server
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default
    * - bindaddr=<string>
      - Address to bind TCP listener to
      - 0.0.0.0
    * - port=<int>
      - Local TCP port to bind to (required)
      -
    * - remotenode=<string>
      - Receptor node to connect to (required)
      -
    * - remoteservice=<string>
      - Receptor service name to connect to (required)
      -
    * - tlsserver=<string>
      - Name of TLS server config for the TCP listener
      -
    * - tlsclient=<string>
      - Name of TLS client config for the Receptor connection
      -

^^^^^^^^^^
UDP Client
^^^^^^^^^^

.. list-table:: UDP Client
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default
    * - address=<string>
      - Address for outbound UDP connection (required)
      -
    * - service=<string>
      - Receptor service name to bind to (required)
      -

^^^^^^^^^^
UDP Server
^^^^^^^^^^

.. list-table:: UDP Server
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default
    * - bindaddr=<string>
      - Address to bind UDP listener to
      - 0.0.0.0
    * - port=<int>
      - Local UDP port to bind to (required)
      -
    * - remotenode=<string>
      - Receptor node to connect to (required)
      -
    * - remoteservice=<string>
      - Receptor service name to connect to (required)
      -

^^^^^^^^^^^^^^^^^^
Unix Socket Client
^^^^^^^^^^^^^^^^^^

.. list-table:: Unix Socket Client
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default
    * - filename=<string>
      - Socket filename, which must already exist (required)
      -
    * - service=<string>
      - Receptor service name to bind to (required)
      -
    * - tls=<string>
      - Name of TLS server config for the Receptor connection
      -

^^^^^^^^^^^^^^^^^^
Unix Socket Server
^^^^^^^^^^^^^^^^^^

.. list-table:: Unix Socket Server
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default
    * - filename=<string>
      - Socket filename, which will be overwritten (required)
      -
    * - permissions=<int>
      - Socket file permissions
      - 0600
    * - remotenode=<string>
      - Receptor node to connect to (required)
      -
    * - remoteservice=<string>
      - Receptor service name to connect to (required)
      -
    * - tls=<string>
      - Name of TLS client config for the Receptor connection
      -

--------------------------------------------
Configure workers that process units of work
--------------------------------------------

.. list-table:: Configure workers that process units of work
    :header-rows: 1
    :widths: auto

    * - Action
      - Description
    * - --work-command
      - Run a worker using an external command
    * - --work-kubernetes
      - Run a worker using Kubernetes
    * - --work-python
      - Run a worker using a Python plugin
    * - --work-signing
      - Private key to sign work submissions
    * - --work-verification
      - Public key to verify work submissions

^^^^^^^^^^^^
Work Command
^^^^^^^^^^^^

.. list-table:: Work Command
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default
    * - allowruntimeparams=<bool>
      - Allow users to add more parameters
      - false
    * - command=<string>
      - Command to run to process units of work (required)
      -
    * - params=<string>
      - Command-line parameters
      -
    * - verifysignature=<bool>
      - Verify a signed work submission
      - false
    * - worktype=<string>
      - Name for this worker type (required)
      -

^^^^^^^^^^^^^^^
Work Kubernetes
^^^^^^^^^^^^^^^

.. list-table:: Work Kubernetes
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default
    * - allowruntimeauth=<bool>
      - Allow passing API parameters at runtime
      - false
    * - allowruntimecommand=<bool>
      - Allow specifying image & command at runtime
      - false
    * - allowruntimeparams=<bool>
      - Allow adding command parameters at runtime
      - false
    * - allowruntimepod=<bool>
      - Allow passing Pod at runtime
      - false
    * - authmethod=<string>
      - One of: kubeconfig, incluster
      - incluster
    * - command=<string>
      - Command to run in the container (overrides entrypoint)
      -
    * - deletepodonrestart=<bool>
      - On restart, delete the pod if in pending state
      - true
    * - image=<string>
      - Container image to use for the worker pod
      -
    * - kubeconfig=<string>
      - Kubeconfig filename (for authmethod=kubeconfig)
      -
    * - namespace=<string>
      - Kubernetes namespace to create pods in
      -
    * - params=<string>
      - Command-line parameters to pass to the entrypoint
      -
    * - pod=<string>
      - Pod definition filename, in json or yaml format
      -
    * - streammethod=<string>
      - Method for connecting to worker pods: logger or tcp
      - logger
    * - verifysignature=<bool>
      - Verify a signed work submission
      - false
    * - worktype=<string>
      - Name for this worker type (required)
      -

^^^^^^^^^^^
Work Python
^^^^^^^^^^^

.. list-table:: Work Python
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default
    * - config=<JSON dict with string keys>
      - Plugin-specific configuration
      -
    * - function=<string>
      - Receptor-exported function to call (required)
      -
    * - plugin=<string>
      - Python module name of the worker plugin (required)
      -
    * - worktype=<string>
      - Name for this worker type (required)
      -

^^^^^^^^^^^^
Work Signing
^^^^^^^^^^^^

.. list-table:: Work Signing
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default
    * - privatekey=<string>
      - Private key to sign work submissions
      -
    * - tokenexpiration=<string>
      - Expiration of the signed json web token, e.g. 3h or 3h30m
      -

^^^^^^^^^^^^^^^^^
Work Verification
^^^^^^^^^^^^^^^^^

.. list-table:: Work Verification
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default
    * - publickey=<string>
      - Public key to verify signed work submissions
      -

-----------------------------------------------------
Generate certificates and run a certificate authority
-----------------------------------------------------

.. list-table:: Generate certificates and run a certificate authority
    :header-rows: 1
    :widths: auto

    * - Action
      - Description
    * - --cert-init
      - Initialize PKI CA
    * - --cert-makereq
      - Create certificate request
    * - --cert-signreq
      - Sign request and produce certificate

^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
Certificate Authority Initialization
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

.. list-table:: Certificate Authoirity Initialization
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default
    * - bits=<int>
      - Bit length of the encryption keys of the certificate (required)
      -
    * - commonname=<string>
      - Common name to assign to the certificate (required)
      -
    * - notafter=<string>
      - Expiration (NotAfter) date/time, in RFC3339 format
      -
    * - notbefore=<string>
      - Effective (NotBefore) date/time, in RFC3339 format
      -
    * - outcert=<string>
      - File to save the CA certificate to (required)
      -
    * - outkey=<string>
      - File to save the CA private key to (required)
      -

^^^^^^^^^^^^^^^^^^^^^^^^^^
Create Certificate Request
^^^^^^^^^^^^^^^^^^^^^^^^^^

.. list-table:: Create Certificate Request
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default
    * - bits=<int>
      - Bit length of the encryption keys of the certificate
      -
    * - commonname=<string>
      - Common name to assign to the certificate (required)
      -
    * - dnsname=<[]string (may be repeated)>
      - DNS names to add to the certificate
      -
    * - inkey=<string>
      - Private key to use for the request
      -
    * - ipaddress=<[]string (may be repeated)>
      - IP addresses to add to the certificate
      -
    * - nodeid=<[]string (may be repeated)>
      - Receptor node IDs to add to the certificate
      -
    * - outreq=<string>
      - File to save the certificate request to (required)
      -
    * - outkey=<string>
      - File to save the private key to (new key will be generated)
      -

^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
Sign Request and Produce Certificate
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

.. list-table:: Sign Request and Produce Certificate
    :header-rows: 1
    :widths: auto

    * - Parameter
      - Description
      - Default
    * - cacert=<string>
      - CA certificate PEM filename (required)
      -
    * - cakey=<string>
      - CA private key PEM filename (required)
      -
    * - notafter=<string>
      - Expiration (NotAfter) date/time, in RFC3339 format
      -
    * - notbefore=<string>
      - Effective (NotBefore) date/time, in RFC3339 format
      -
    * - outcert=<string>
      - File to save the signed certificate to (required)
      -
    * - req=<string>
      - Certificate Request PEM filename (required)
      -
    * - verify=<bool>
      - If true, do not prompt the user for verification
      - False
