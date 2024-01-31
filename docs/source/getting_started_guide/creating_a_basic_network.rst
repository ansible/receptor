###############################
Creating a basic 3-node network
###############################

In this section, we will create a three-node network.
The three nodes are: foo, bar, and mal.

`foo -> bar <- mal`

foo and mal are directly connected to bar with TCP connections.

foo can reach mal by sending network packets through bar.

***********************
Receptor configurations
***********************

1. Create three configuration files, one for each node.

 **foo.yml**

.. code-block:: yaml

    ---
    - node:
      id: foo

    - control-service:
      service: control
      filename: /tmp/foo.sock

    - tcp-peer:
      address: localhost:2222
      redial: true

    - log-level: debug
    ...

**bar.yml**

.. code-block:: yaml

    ---
    - node:
      id: bar

    - control-service:
      service: control
      filename: /tmp/bar.sock

    - tcp-listener:
      port: 2222

    - log-level: debug
    ...

 **mal.yml**

.. code-block:: yaml

    ---
    - node:
      id: mal

    - control-service:
      service: control
      filename: /tmp/mal.sock

    - tcp-peer:
      address: localhost:2222
      redial: true

    - log-level: debug

    - work-command:
      workType: echo
      command: bash
      params: "-c \"while read -r line; do echo $line; sleep 1; done\""
      allowruntimeparams: true
    ...

1. Run the services in separate terminals.

.. code-block:: bash

    ./receptor --config foo.yml

.. code-block:: bash

    ./receptor --config bar.yml

.. code-block:: bash

    ./receptor --config mal.yml

.. seealso::

    :ref: `configuring_receptor_with_a_config_file`
    :ref: `connecting_nodes`
