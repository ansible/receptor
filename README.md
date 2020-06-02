Receptor
========

Receptor is an overlay network intended to ease the distribution of 
work across a large and dispersed collection of workers.  Receptor nodes
establish peer-to-peer connections with each other via existing networks.
Once connected, the Receptor mesh provides datagram (UDP-like) and stream
(TCP-like) capabilities to applications, as well as robust unit-of-work
handling with resiliency against transient network failures.

Terminology and Concepts

* _Node_: A single running instance of Receptor.
* _Node ID_: The network address of a Receptor node.  Currently a simple string.
* _Service_: An up-to-8-character string identifying an endpoint on a Receptor
  node that can receive messages.  Analogous to a port number in TCP or UDP.
* _Backend_: A type of connection between nodes that the Receptor network
  protocol can run over.  Current backends include TCP, UDP and websockets.
* _Unit of work_: A single task that is intended to be run on a node.  For
  example, a command invocation or an Ansible playbook.
* _Response_: A single item from a stream of responses arising from the
  execution of a unit of work.  For Ansible playbooks, these are job events.
* _Controller_: An application-level process that creates units of work and
  sends them to nodes to be executed.
* _Worker_: A process that binds to a Receptor service and registers itself as
  capable of executing jobs of a particular type.

Use as a Go library

This code can be imported and used from Go programs.  The main libraries are
`netceptor`, which implements the network protocol, and `workceptor` which
implements the unit-of-work handling.  See the `example/` directory for
examples of using these libraries from Go.

Use as a command-line tool

The `receptor` command runs a Receptor node with access to all included
backends and services.  See `receptor --help` for details.

Proxy services

The command-line tool includes services that implement network proxies
via TCP, UDP or (on Linux) tun interfaces.  For example, the following
command will start a TCP proxy that listens for TCP connections on port
1234 on localhost, and then forwards each connection to port 4321.

```sh
receptor --node-id standalone --local-only \
    --tcp-inbound-proxy port=1234 remotenode=standalone remoteservice=proxy \
    --tcp-outbound-proxy address=localhost:4321 service=proxy
```

Unit-of-work handling

The `workceptor` library will handle unit-of-work management.  This is still
in progress.
