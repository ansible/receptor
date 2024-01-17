+++++++++++++++++++++++++++++++
Creating a basic 3-node network
+++++++++++++++++++++++++++++++

In this section, we will create a three-node network.
The three nodes are: foo, bar, and mal

`foo -> bar <- mal`

foo and mal are directly connected to bar with TCP connections.

foo can reach mal by sending network packets through bar.

#######################
Receptor configurations
#######################

To create this, we will create three configuration files, one for each node.
1. foo.yml

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

1. bar.yml

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

1. mal.yml

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

1. Run the services in separate terminals

.. code-block: bash

    ./receptor --config foo.yml

.. code-block: bash

    ./receptor --config bar.yml

.. code-block: bash

    ./receptor --config mal.yml

.. seealso::

    :ref: `Configuring Receptor with a config file <https://ansible.readthedocs.io/projects/receptor/en/latest/user_guide/basic_usage.html#configuring-receptor-with-a-config-file>`_
    :ref: `Connecting Nodes <https://ansible.readthedocs.io/projects/receptor/en/latest/user_guide/connecting_nodes.html>`_
