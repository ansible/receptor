
.. _creating_a_basic_network:

###############################
Creating a basic 3-node network
###############################

In this section, we will create a three-node network.
The three nodes are: foo, bar, and baz.

`foo -> bar <- baz`

foo and baz are directly connected to bar with TCP connections.

foo can reach baz by sending network packets through bar.

***********************
Receptor configurations
***********************

1. Create three configuration files, one for each node.

``foo.yml``

.. code-block:: yaml

  - node:
      id: foo

  - control-service:
      service: control
      filename: /tmp/foo.sock

  - tcp-peer:
      address: localhost:2222

  - log-level:
      level: debug

  ...

``bar.yml``

.. code-block:: yaml

  ---
  - node:
      id: bar

  - control-service:
      service: control
      filename: /tmp/bar.sock

  - tcp-listener:
      port: 2222

  - log-level:
      level: debug

  ...

``baz.yml``

.. code-block:: yaml

  ---
  - node:
      id: baz

  - control-service:
      service: control
      filename: /tmp/baz.sock

  - tcp-peer:
      address: localhost:2222

  - log-level:
      level: debug

  - work-command:
      workType: echo
      command: bash
      params: "-c \"while read -r line; do echo $line; sleep 1; done\""
      allowruntimeparams: true

  ...

2. Run the services in separate terminals.

.. code-block:: bash

    ./receptor --config foo.yml

.. code-block:: bash

    ./receptor --config bar.yml

.. code-block:: bash

    ./receptor --config baz.yml

.. seealso::

    :ref:`configuring_receptor_with_a_config_file`
        Configuring Receptor with a configuration file
    :ref:`connecting_nodes`
        Detail on connecting receptor nodes
