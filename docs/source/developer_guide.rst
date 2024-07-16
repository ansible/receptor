.. _dev_guide:

===============
Developer guide
===============

Receptor is an open source project that lives at `ansible/receptor repository <https://github.com/ansible/receptor/>`

.. contents::
   :local:

See the :ref:`contributing:contributing` for more general details.

-------
Linters
-------

All code must pass a suite of Go linters.
There is a pre-commit yaml file in the Receptor repository that points to the linter suite.
It is strongly recommmended to install the pre-commit yaml so that the linters run locally on each commit.

.. code-block:: bash

    cd $HOME
    go get github.com/golangci/golangci-lint/cmd/golangci-lint
    pip install pre-commit
    cd receptor
    pre-commit install

See `Pre commit <https://pre-commit.com/>`_ and `Golangci-lint <https://golangci-lint.run/>`_ for more details on installing and using these tools.

-------
Testing
-------

^^^^^^^^^^^
Development
^^^^^^^^^^^

Write unit tests for new features or functionality.
Add/Update tests for bug fixes.

^^^^^^^
Mocking
^^^^^^^

We are using gomock to generate mocks for our unit tests. The mocks are living inside of a package under the real implementation, prefixed by ``mock_``. An example is the package mock_workceptor under pkg/workceptor.

In order to genenerate a mock for a particular file, run:

.. code-block:: bash

    mockgen -source=pkg/filename.go -destination=pkg/mock_pkg/mock_filename.go

For example, to create/update mocks for Workceptor, we can run:

.. code-block:: bash

    mockgen -source=pkg/workceptor/workceptor.go -destination=pkg/workceptor/mock_workceptor/workceptor.go

^^^^^^^^^^
Kubernetes
^^^^^^^^^^

Some of the tests require access to a Kubernetes cluster; these tests will load in the kubeconfig file located at ``$HOME/.kube/config``. One simple way to make these tests work is to start minikube locally before running ``make test``. See https://minikube.sigs.k8s.io/docs/start/ for more information about minikube.

To skip tests that depend on Kubernetes, set environment variable ``export SKIP_KUBE=1``.

^^^^^^^^^
Execution
^^^^^^^^^

Pull requests must pass a suite of unit and integration tests before being merged into ``devel``.

``make test`` will run the full test suite locally.

-----------
Source code
-----------

The following sections help orient developers to the Receptor code base and provide a starting point for understanding how Receptor works.

^^^^^^^^^^^^^^^^^^^^^
Parsing receptor.conf
^^^^^^^^^^^^^^^^^^^^^

Let's see how items in the config file are mapped to Golang internals.

As an example, in tcp.go

.. code-block:: go

    cmdline.RegisterConfigTypeForApp("receptor-backends",
      "tcp-peer", "Make an outbound backend connection to a TCP peer", TCPDialerCfg{}, cmdline.Section(backendSection))


"tcp-peer" is a top-level key (action item) in receptor.conf

.. code-block:: yaml

    - tcp-peer:
        address: localhost:2222

``RegisterConfigTypeForApp`` tells the cmdline parser that "tcp-peer" is mapped to the ``TCPDialerCfg{}`` structure.

``main()`` in ``receptor.go`` is the entry point for a running Receptor process.

In ``receptor.go`` (modified for clarity):

.. code-block:: go

    cl.ParseAndRun("receptor.conf", []string{"Init", "Prepare", "Run"})

A Receptor config file has many action items, such as ```node``, ``work-command``, and ``tcp-peer``. ``ParseAndRun`` is how each of these items are instantiated when Receptor starts.

Specifically, ParseAndRun will run the Init, Prepare, and Run methods associated with each action item.

Here is the Prepare method for ``TCPDialerCfg``. By the time this code executes, the cfg structure has already been populated with the data provided in the config file.

.. code-block:: go

    // Prepare verifies the parameters are correct.
    func (cfg TCPDialerCfg) Prepare() error {
        if cfg.Cost <= 0.0 {
            return fmt.Errorf("connection cost must be positive")
        }

        return nil
    }

This simply does a check to make sure the provided Cost is valid.

The Run method for the ``TCPDialerCfg`` object:

.. code-block:: go

    // Run runs the action.
    func (cfg TCPDialerCfg) Run() error {
        logger.Debug("Running TCP peer connection %s\n", cfg.Address)
        host, _, err := net.SplitHostPort(cfg.Address)
        if err != nil {
            return err
        }
        tlscfg, err := netceptor.MainInstance.GetClientTLSConfig(cfg.TLS, host, "dns")
        if err != nil {
            return err
        }
        b, err := NewTCPDialer(cfg.Address, cfg.Redial, tlscfg)
        if err != nil {
            logger.Error("Error creating peer %s: %s\n", cfg.Address, err)

            return err
        }
        err = netceptor.MainInstance.AddBackend(b, cfg.Cost, nil)
        if err != nil {
            return err
        }

        return nil
    }

This gets a new TCP dialer object and passes it to the netceptor ``AddBackend`` method, so that it can be processed further.
``AddBackend`` will start proper Go routines that periodically dial the address defined in the TCP dialer structure, which will lead to a proper TCP connection to another Receptor node.

In general, when studying how the start up process works in Receptor, take a look at the ``Init``, ``Prepare``, and ``Run`` methods throughout the code, as these are the entry points to running those specific components of Receptor.

^^^^
Ping
^^^^

Studying how pings work in Receptor will provide a useful glimpse into the internal workings of netceptor -- the main component of Receptor that handles connections and data traffic over the mesh.

``receptorctl --socket /tmp/foo.sock ping bar``

The control-service on `foo` will receive this command and subsequently call the following,

**ping.go::ping**

.. code-block:: go

    func ping(nc *netceptor.Netceptor, target string, hopsToLive byte) (time.Duration, string, error) {
        pc, err := nc.ListenPacket("")

``target`` is the target node, "bar" in this case.

``nc.ListenPacket("")`` starts a new ephemeral service and returns a ``PacketConn`` object. This is a datagram connection that has a WriteTo() and ReadFrom() method for sending and receiving data to other nodes on the mesh.

**packetconn.go::ListenPacket**

.. code-block:: go

    pc := &PacketConn{
        s:            s,
        localService: service,
        recvChan:     make(chan *messageData),
        advertise:    false,
        adTags:       nil,
        connType:     ConnTypeDatagram,
        hopsToLive:   s.maxForwardingHops,
    }

    s.listenerRegistry[service] = pc

    return pc, nil

``s`` is the main netceptor object, and a reference to the PacketConn object is stored in netceptor's ``listenerRegistry`` map.


**ping.go::ping**

.. code-block:: go

    _, err = pc.WriteTo([]byte{}, nc.NewAddr(target, "ping"))

Sends an empty message to the address "bar:ping" on the mesh. Recall that nodes are analogous to DNS names, and services are like port numbers.

``WriteTo`` calls ``sendMessageWithHopsToLive``

**netceptor.go::sendMessageWithHopsToLive**

.. code-block:: go

    md := &messageData{
        FromNode:    s.nodeID,
        FromService: fromService,
        ToNode:      toNode,
        ToService:   toService,
        HopsToLive:  hopsToLive,
        Data:        data,
    }

    return s.handleMessageData(md)

Here the message is constructed with essential information such as the source node and service, and the destination node and service. The Data field contains the actual message, which is empty in this case.

``handleMessageData`` calls ``forwardMessage`` with the ``md`` object.

**netceptor.go::forwardMessage**

.. code-block:: go

    nextHop, ok := s.routingTable[md.ToNode]

The current node might not be directly connected to the target node, and thus netceptor needs to determine what is the next hop to pass the data to. ``s.routingTable`` is a map where the key is a destination ("bar"), and the value is the next hop along the path to that node. In a simple two-node setup with `foo` and `bar`, ``s.routingTable["bar"] == "bar"``.

**netceptor.go::forwardMessage**

.. code-block:: go

    c, ok := s.connections[nextHop]

    c.WriteChan <- message

``c`` here is a ``ConnInfo`` object, which interacts with the various backend connections (UDP, TCP, websockets).

``WriteChan`` is a golang channel. Channels allows communication between separate threads (Go routines) running in the application. When `foo` and `bar` had first started, they established a backend connection. Each node runs the netceptor runProtocol go routine, which in turn starts a protoWriter go routine.

**netceptor.go::protoWriter**

.. code-block:: go

    case message, more := <-ci.WriteChan:
      err := sess.Send(message)

So before the "ping" command was issued, this protoWriter Go routine was already running and waiting to read messages from WriteChan.

``sess`` is a BackendSession object. BackendSession is an abstraction over the various available backends. If `foo` and `bar` are connected via TCP, then ``sess.Send(message)`` will pass along data to the already established TCP session.

**tcp.go::Send**

.. code-block:: go

    func (ns *TCPSession) Send(data []byte) error {
        buf := ns.framer.SendData(data)
        n, err := ns.conn.Write(buf)

``ns.conn`` is net.Conn object, which is part of the Golang standard library.

At this point the message has left the node via a backend connection, where it will be received by `bar`.

Let's review the code from `bar`'s perspective and how it handles the incoming message that is targeting its "ping" service.

On the receiving side, the data will first be read here

**tcp.go::Recv**

.. code-block:: go

    n, err := ns.conn.Read(buf)

    ns.framer.RecvData(buf[:n])


Recv was called in protoReader Go routine, similar to the protoWriter when the message sent from `foo`.

Note that ``ns.conn.Read(buf)`` might not contain the full message, so the data is buffered until the ``messageReady()`` returns true. The size of the message is tagged in the message itself, so when Recv has received N bytes, and the message is N bytes, Recv will return.

**netceptor.go::protoReader**

.. code-block:: go

    buf, err := sess.Recv(1 * time.Second)
    ci.ReadChan <- buf

The data is passed to a ReadChan channel.

**netceptor.go::runProtocol**

.. code-block:: go

    case data := <-ci.ReadChan:

      message, err := s.translateDataToMessage(data)

      err = s.handleMessageData(message)

The data is read from the channel, and deserialized into an actual message format in ``translateDataToMessage``.

**netceptor.go::handleMessageData**

.. code-block:: go

    if md.ToNode == s.nodeID {
      handled, err := s.dispatchReservedService(md)

This checks whether the destination node indicated in the message is the current node. If so, the message can be dispatched to the service.

"ping" is a reserved service in the netceptor instance.

.. code-block:: go

    s.reservedServices = map[string]func(*messageData) error{
      "ping":    s.handlePing,
    }

**netceptor.go::handlePing**

.. code-block:: go

    func (s *Netceptor) handlePing(md *messageData) error {
        return s.sendMessage("ping", md.FromNode, md.FromService, []byte{})
    }

This is the ping reply handler. It sends an empty message to the FromNode (`foo`).

The FromService here is not "ping", but rather the ephemeral service that was created from ``ListenPacket("")`` in ping.go on `foo`.

With ``trace`` enabled in the Receptor configuration, the following log statements show the reply from ``bar``,

.. code-block:: bash

    TRACE --- Received data length 0 from foo:h73opPEh to bar:ping via foo
    TRACE --- Sending data length 0 from bar:ping to foo:h73opPEh

So the ephemeral service on `foo` is called h73opPEh (randomly generated string).


From here, the message from `bar` will passed along in a very similar fashion as the original ping message sent from `foo`.

Back on node `foo`, the message is received receive the message where it is finally handled in ping.go

**ping.go::ping**

.. code-block:: go

    _, addr, err := pc.ReadFrom(buf)

.. code-block:: go

    case replyChan <- fromNode:

.. code-block:: go

    case remote := <-replyChan:
      return time.Since(startTime), remote, nil

The data is read from the PacketConn object, written to a channel, where it is read later by the ping() function, and ping() returns with the roundtrip delay, ``time.Since(startTime)``.
