###################
Try Sample Commands
###################

.. note::
    The prior steps of network setup and receptor installation need to be completed in order for these command to work.

1. Show network status

.. code-block:: bash

    receptorctl --socket /tmp/foo.sock status

2. Ping node mal from node foo

.. code-block:: bash

    receptorctl --socket /tmp/foo.sock ping mal

3. Submit work from foo to mal and stream results back to foo

.. code-block:: bash

    seq 10 | receptorctl --socket /tmp/foo.sock work submit --node mal echo --payload - -f

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
