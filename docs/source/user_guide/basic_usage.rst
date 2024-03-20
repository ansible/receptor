Using Receptor
===============

. contents::

   :local:

Configuring Receptor with the CLI
-----------------------------------

Run the following command in a terminal to start a node called `foo`,

.. code::

    receptor --node id=foo --local-only --log-level Debug

The log shows the receptor node started successfully

``INFO 2021/07/22 22:40:36 Initialization complete``

Supported log levels, in increasing verbosity, are Error, Warning, Info (default) and Debug.

Note: stop the receptor process with ``ctrl-c``

.. _configuring_receptor_with_a_config_file:

Configuring Receptor with a config file
----------------------------------------

Receptor can be configured on the command-line, exemplified above, or via a yaml config file. All actions and parameters shown in ``receptor --help`` can be written to a config file.

.. code-block:: yaml

    ---
    - node:
        id: foo

    - local-only

    - log-level:
        level: Debug

Start receptor using the config file

.. code::

    receptor --config foo.yml

Changing the configuration file does take effect until the receptor process is restarted.

.. _using_receptor_containers:

Use Receptor through a container image
---------------------------------------

.. code::

    podman pull quay.io/ansible/receptor

Start a container, which automatically runs receptor with the default config located at ``/etc/receptor/receptor.conf``

.. code::

    podman run -it --rm --name receptor quay.io/ansible/receptor

In another terminal, issue a basic "status" command to the running receptor process

.. code::

    $ podman exec receptor receptorctl status
    Node ID: d9b5a8e3c156
    Version: 1.0.0
    System CPU Count: 8
    System Memory MiB: 15865

    Node         Service   Type       Last Seen           Tags            Work Types
    d9b5a8e3c156 control   Stream     2021-08-04 19:26:14 -               -

Note: the config file does not specify a node ID, so the hostname (on the container) is chosen as the node ID.
