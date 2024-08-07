###################
Try Sample Commands
###################

.. note::
    You must complete the prior steps of network setup and Receptor installation for these commands to work.

1. Show network status

.. code-block:: bash

    receptorctl --socket /tmp/foo.sock status

2. Ping node baz from node foo

.. code-block:: bash

    receptorctl --socket /tmp/foo.sock ping baz

3. Submit work from foo to baz and stream results back to foo

.. code-block:: bash

    seq 10 | receptorctl --socket /tmp/foo.sock work submit --node baz echo --payload - -f

4. List work units

.. code-block:: bash

    receptorctl --socket /tmp/foo.sock work list --node foo

5. Get work unit id using jq

.. code-block:: bash

    receptorctl --socket /tmp/foo.sock work list --node foo | jq --raw-output '.|keys|first'

6. Re-stream the work results from work unit

.. code-block:: bash

    receptorctl --socket /tmp/foo.sock work results work_unit_id

Congratulations, Receptor is now ready to use!

.. seealso::

    :ref:`control_service_commands`
        Control service commands
    :ref:`creating_a_basic_network`
        Creating a Basic Network
    :ref:`installing_receptor`
        Installing Receptor
