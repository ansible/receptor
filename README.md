Receptor
========

Receptor is an overlay network intended to ease the distribution of work across a large and dispersed collection of workers.  Receptor nodes establish peer-to-peer connections with each other via existing networks.  Once connected, the Receptor mesh provides datagram (UDP-like) and stream (TCP-like) capabilities to applications, as well as robust unit-of-work handling with resiliency against transient network failures.

See the readthedocs page for Receptor at:

https://receptor.readthedocs.io/en/latest

## Terminology and Concepts

* _Receptor_: The Receptor application taken as a whole, that typically runs as a daemon.
* _Receptorctl_: A user-facing command line used to interact with Receptor, typically over a Unix domain socket.
* _Netceptor_: The networking part of Receptor.  Usable as a Go library.
* _Workceptor_: The unit-of-work handling of Receptor, which makes use of Netceptor.  Also usable as a Go library.
* _Node_: A single running instance of Receptor.
* _Node ID_: An arbitrary string identifying a single node, analogous to an IP address.
* _Service_: An up-to-8-character string identifying an endpoint on a Receptor node that can receive messages.  Analogous to a port number in TCP or UDP.
* _Backend_: A type of connection that Receptor nodes can pass traffic over. Current backends include TCP, UDP and websockets.
* _Control Service_: A built-in service that usually runs under the name `control`.  Used to report status and to launch and monitor work.

## How to Get It

The easiest way to check out Receptor is to run it as a container.  Images are kept on the Quay registry.  To use this, run:
```
[docker|podman] pull quay.io/ansible/receptor
[docker|podman] run -d -v /path/to/receptor.conf:/etc/receptor/receptor.conf:Z receptor
```

## Use as a Go library

This code can be imported and used from Go programs.  The main libraries are:

* _Netceptor_: https://pkg.go.dev/github.com/ansible/receptor/pkg/netceptor
* _Workceptor_: https://pkg.go.dev/github.com/ansible/receptor/pkg/workceptor

See the `example/` directory for examples of using these libraries from Go.

## Use as a command-line tool

The `receptor` command runs a Receptor node with access to all included backends and services.  See `receptor --help` for details.

The command line is organized into entities which take parameters, like: `receptor --entity1 param1=value1 param2=value1 --entity2 param1=value2 param2=value2`.  In this case we are configuring two things, `entity1` and `entity2`, each of which takes two parameters.  Distinct entities are marked with a double dash, and bare parameters attach to the immediately preceding entity.

Receptor can also take its configuration from a file in YAML format.  The allowed directives are the same as on the command line, with a top-level list of entities and each entity receiving zero or more parameters as a dict.  The above command in YAML format would look like this:

```
---
- entity1:
    param1: value1
    param2: value1
- entity2:
    param1: value2
    param2: value2
```

## Python Receptor and the 0.6 versions

As of June 25th, this repo is the Go implementation of Receptor. If you are looking for the older Python version of Receptor, including any 0.6.x version, it is now located at https://github.com/ansible/python-receptor.
