

.. _interacting_with_nodes:

Interacting with nodes
======================

.. contents::
   :local:


The ``control-service`` allows the user to issue commands like "status" or "work submit" to a receptor node.

foo.yml

.. code-block:: yaml

    ---
    - node:
        id: foo

    - log-level:
        level: Debug

    - tcp-listener:
        port: 2222

    - control-service:
        service: control
        filename: /tmp/foo.sock

bar.yml

.. code-block:: yaml

    ---
    - node:
        id: bar

    - log-level:
        level: Debug

    - tcp-peer:
        address: localhost:2222

    - control-service:
        service: control

If ``filename`` is set, receptor will create a unix domain socket. Use receptorctl to interact with the running receptor node via this domain socket (using "--socket"). The control service on `bar` does not have a ``filename`` set, but can be connected to using the "connect" command, as shown in the :ref:`connect_to_csv` section.

The "status" command will display helpful information about mesh, including known connections, routing tables, control services, and work types.

.. code-block:: console

    $ receptorctl --socket /tmp/foo.sock status
    Node ID: foo
    Version: 0.9.8.dev57-0.git20210722.4d0310f
    System CPU Count: 8
    System Memory MiB: 15876

    Connection   Cost
    bar          1

    Known Node   Known Connections
    bar          {'foo': 1}
    foo          {'bar': 1}

    Route        Via
    bar          bar

    Node         Service   Type       Last Seen           Tags            Work Types
    foo          control   Stream     2021-07-22 23:29:34 -               -
    bar          control   Stream     2021-07-22 23:32:35 -               -

ReceptorControl
----------------

For a more programmatic way to interact with receptor nodes, use the ReceptorControl python class.

 .. code-block:: python

    from receptorctl import ReceptorControl

    r = ReceptorControl("/tmp/foo.sock")
    r.simple_command("work list")

.. _connect_to_csv:

Connect to control service
---------------------------

Use the "connect" command to connect to any receptor control service running on the mesh. From here, issue a series of commands and examine the output, without disconnecting.

.. code-block:: console

    $ receptorctl --socket /tmp/foo.sock connect bar control
    Receptor Control, node bar

This will result in a bridged connection between the local domain socket on `foo`, and the control service listener from `bar`.

One can also connect to the locally running control service in a similar manner

.. code-block:: console

    $ receptorctl --socket /tmp/foo.sock connect localhost control
    Receptor Control, node foo

"localhost" is a special keyword that tells receptor to connect to its own control-service. "localhost" can be used in all other control service commands that expect a node ID.

Once connected to a control service, one can issue commands like "status" or "work list" and get JSON-formatted responses back.

Keep in mind that a "work submit" command will require a payload. Type out the payload contents and press ctrl-D to send the EOF signal. The socket will then close and work will begin. See :ref:`user_guide/workceptor:workceptor` for more on submitting work via receptor.

.. _control_service_commands:

Control service commands
--------------------------

A ``control-service`` can accept commands in two formats; a space-delimited string or JSON. In some cases, JSON accepts arguments that are not supported in the string format and are marked with `json-only` in the table below.

String example:

.. code-block:: console

    work submit bar echoint

JSON example:

.. code-block:: json

    {
      "command":"work",
      "subcommand":"submit",
      "node":"bar",
      "worktype":"echoint"
    }

For 2-word commands like ``work submit`` the first word is the "command", and the second word is the "subcommand"

The order of the parameters (from left to right) in the following table matter, as they are the order expected when issuing commands in string format.

.. list-table::
    :widths: 15 25 50
    :header-rows: 1

    * - command
      - required parameters
      - optional parameters
    * - status
      -
      -
    * - reload
      -
      -
    * - ping
      - target
      -
    * - traceroute
      - target
      -
    * - work list
      -
      - unitid
    * - work submit
      - node, worktype
      - tlsclient (`json-only`), ttl (`json-only`)
    * - work cancel
      - unitid
      -
    * - work release
      - unitid
      -
    * - work force-release
      - unitid
      -
    * - work results
      - unitid, startpos
      -

The above table does not apply the receptorctl command-line tool. For the exact usage of the various receptorctl commands, type ``receptorctl --help``, or to see the help for a specific command, ``receptorctl work submit --help``.

Reload
-------

In general, changes to a receptor configuration file do not take effect until the receptor process is restarted.

However, the action items pertaining to receptor backend connections can be reloaded, without a receptor restart. These include the following,

.. code::

    tcp-peer
    tcp-listener
    ws-peer
    ws-listener
    udp-peer
    udp-listener
    local-only

Changes can include modifying, adding, or removing these items from the configuration file.

After saving the configuration file to disk, connect to a control service and issue a ``reload`` command for the new changes to take effect.

.. code-block:: console

    receptorctl --socket /tmp/foo.sock reload

This command will cancel all running backend connections and sessions, re-parse the configuration file, and start the backends once more.

This allows users to add or remove backend connections without disrupting ongoing receptor operations. For example, sending payloads or getting work results will only momentarily pause after a reload and will resume once the connections are reestablished.
