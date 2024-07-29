Using Receptor
===============

. contents::

   :local:

----------------------
Using the Receptor CLI
----------------------

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
      - Display this help

Configuring Receptor with a config file
----------------------------------------

Receptor can be configured on the command-line, exemplified above, or via a yaml config file. All actions and parameters shown in ``receptor --help`` can be written to a config file.

.. code-block:: yaml

    ---
    version: 2
    node:
      id: foo

    local-only:
      local: true

    log-level:
      level: Debug

Start receptor using the config file

.. code-block:: bash

    receptor --config foo.yml

Changing the configuration file does take effect until the receptor process is restarted.

.. _using_receptor_containers:

Use Receptor through a container image
---------------------------------------

.. code-block:: bash

    podman pull quay.io/ansible/receptor

Start a container, which automatically runs receptor with the default config located at ``/etc/receptor/receptor.conf``

.. code-block:: bash

    podman run -it --rm --name receptor quay.io/ansible/receptor

In another terminal, issue a basic "status" command to the running receptor process

.. code-block:: bash

    $ podman exec receptor receptorctl status
    Node ID: d9b5a8e3c156
    Version: 1.0.0
    System CPU Count: 8
    System Memory MiB: 15865

    Node         Service   Type       Last Seen           Tags            Work Types
    d9b5a8e3c156 control   Stream     2021-08-04 19:26:14 -               -

Note: the config file does not specify a node ID, so the hostname (on the container) is chosen as the node ID.
