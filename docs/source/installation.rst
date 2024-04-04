.. _installing:

*******************
Installation guide
*******************

Download and extract precompiled binary for your OS and platform from `the releases page on GitHub <https://github.com/ansible/receptor/releases>`_

Alternatively, you can compile Receptor from source code (Golang 1.20+ required)

.. code-block:: bash

    make receptor

Test the installation with the following commands:

.. code-block:: bash

    receptor --help
    receptor --version

The preferred way to interact with Receptor nodes is to use the ``receptorctl`` command line tool

.. code-block:: bash

    pip install receptorctl

``receptorctl`` will be used in various places throughout this documentation.
