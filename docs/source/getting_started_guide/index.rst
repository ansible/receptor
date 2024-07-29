#############################
Getting started with Receptor
#############################

Receptor is an overlay network that distributes work across large and dispersed collections of worker nodes.
Receptor nodes establish peer-to-peer connections through existing networks.
Once connected, the Receptor mesh provides datagram (UDP-like) and stream (TCP-like) capabilities to applications, as well as robust unit-of-work handling with resiliency against transient network failures.

.. image:: mesh.png

.. toctree::
    :maxdepth: 1
    :caption: Contents:

    introduction
    installing_receptor
    creating_a_basic_network
    trying_sample_commands

.. seealso::

    :ref:`interacting_with_nodes`
        Further examples of working with nodes
    :ref:`connecting_nodes`
        Detail on connecting receptor nodes
