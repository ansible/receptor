###################
Try Sample Commands
###################

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

Congratulations, you are now using Receptor!

.. seealso::

    :ref: `control_service_commands`
