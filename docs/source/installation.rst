.. _installing:

*******************
Installation guide
*******************

Download and extract precompiled binary for your OS and platform from `the releases page on GitHub <https://github.com/ansible/receptor/releases>`_

Alternatively, you can compile from source code (Golang 1.20+ required)

.. code::

    make receptor

Test the installation with

.. code::

    receptor --help
    receptor --version

The preferred way to interact with receptor nodes is to use the receptorctl command line tool

.. code::

    pip install receptorctl

receptorctl will be used in various places throughout this documentation.
