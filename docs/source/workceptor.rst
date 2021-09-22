.. _workceptor:

Workceptor
==========

Workceptor is a component of receptor that handles units of work.

``work-command`` defines a type of work that can run on the node.

foo.yml

.. code-block:: yaml

    ---
    - node:
        id: foo

    - log-level:
        level: Debug

    - tcp-listener:
        port: 2222

    - control-service:
        service: control
        filename: /tmp/foo.sock

    - work-command:
        workType: echoint
        command: bash
        params:  "-c \"for i in {1..5}; do echo $i; sleep 1; done\""

bar.yml

.. code-block:: yaml

    ---
    - node:
        id: bar

    - log-level:
        level: Debug

    - tcp-peer:
        address: localhost:2222

    - control-service:
        service: control

    - work-command:
        worktype: echoint
        command: bash
        params:  "-c \"for i in {1..10}; do echo $i; sleep 1; done\""

    - work-command:
        workType: echopayload
        command: bash
        params: "-c \"while read -r line; do echo ${line^^}; sleep 3; done\""


Configuring work commands
^^^^^^^^^^^^^^^^^^^^^^^^^

``worktype`` User-defined name to give this work definition

``command`` The executable that is invoked when running this work

``params`` Command-line options passed to this executable


Local work
^^^^^^^^^^

Start the work by connecting to the ``control-service`` and issuing a "work submit" command

.. code::

    $ receptorctl --socket /tmp/foo.sock work submit echoint --no-payload
    Result:  Job Started
    Unit ID: t1BlAB18

Receptor started an instance of this work type, and labeled it with a unique "Unit ID"

Work results
^^^^^^^^^^^^

Use the "Unit ID" to get work results

.. code::

    receptorctl --socket /tmp/foo.sock work results t1BlAB18
    1
    2
    3
    4
    5
    6
    7
    8
    9
    10


Remote work
^^^^^^^^^^^
Although connected to `foo`, by providing the "--node" option the work can be started on node `bar`.

The work type must be defined on the node it is intended to run on, e.g. `bar` must have a ``work-command`` called "echoint", in this case.

.. code::

    $ receptorctl --socket /tmp/foo.sock work submit echoint --node bar --no-payload
    Result:  Job Started
    Unit ID: 87Vwqb6A

Remote work submission ultimately results in two work units running at the same time; a local work unit and the remote work unit. These two units have their own Unit IDs. The local work unit's goal is to monitor and stream results back from the running remote work unit.

Sequence of events for remote work submission

- `foo` starts a local work unit of work type "remote". This is a special work type that is built into receptor.
- This work unit attempts to connect to `bar`'s control service and issue a "work submit echoint" command. From `bar`'s perspective, this is the exact same operation as if a user connected to `bar` directly and issued a work submit command. `bar` is not aware that `foo` is the one that issued the command.
- Once submitted, `foo` will stream work results back to itself and store it on disk. It also periodically gets the ``work status`` of the work running on `bar`. Status includes information about the work state and the stdout size.
- `foo` continues streaming stdout results until the size stored on disk matches the StdoutSize reported in `bar`'s status.


.. _work_payload:

Payload
^^^^^^^^^^^^
in `bar.yml`

.. code-block:: yaml

    - work-command:
        workType: echopayload
        command: bash
        params: "-c \"while read -r line; do echo ${line^^}; sleep 5; done\""

Here the bash command expects to read a line from stdin, echo the line in all uppercase letters, and sleep for 3 seconds.

Payloads can be passed into receptor using the "--payload" option.

.. code::

    $ echo -e "hi\ni am foo\nwhat is your name" | receptorctl --socket /tmp/foo.sock work submit echopayload --node bar --payload - -f
    HI
    I AM FOO
    WHAT IS YOUR NAME

"--payload -" means the payload should be whatever the stdin is, which is piped in from the "echo -e ..." command.

Note: "-f" instructs receptorctl to follow the work unit immediately, i.e. stream results to stdout. One could also use "work results" to stream the results.


Work list
^^^^^^^^^
"work list" returns information about all work units that have ran on this receptor node. The following shows two work units, ``12L8s8h2`` and ``T0oN0CAp``

.. code::

    $ receptorctl --socket /tmp/foo.sock work list
    {'12L8s8h2': {'Detail': 'exit status 0',
                  'ExtraData': None,
                  'State': 2,
                  'StateName': 'Succeeded',
                  'StdoutSize': 21,
                  'WorkType': 'echoint'},
     'T0oN0CAp': {'Detail': 'Running: PID 1700818',
                  'ExtraData': {'Expiration': '0001-01-01T00:00:00Z',
                                'LocalCancelled': False,
                                'LocalReleased': False,
                                'RemoteNode': 'bar',
                                'RemoteParams': {},
                                'RemoteStarted': True,
                                'RemoteUnitID': 'ATDzdViR',
                                'RemoteWorkType': 'echoint',
                                'TLSClient': ''},
                  'State': 1,
                  'StateName': 'Running',
                  'StdoutSize': 4,
                  'WorkType': 'remote'},


Notice that ``T0oN0CAp`` was a remote work submission, therefore its work type is "remote". On `bar` there is a local unit ``ATDzdViR``, with the "echoint" work type.


Work cancel
^^^^^^^^^^^

Cancel will stop any running work unit. Upon canceling a "remote" work unit, the local node will attempt to connect to the remote node's control service and issue a work cancel. If the remote node is down, receptor will periodically attempt to connect to the remote node to do the cancellation.

Work release
^^^^^^^^^^^^

Release will cancel the work and then delete files on disk associated with that work unit. For remote work submission, release will attempt to delete files both locally and on the remote machine. Like work cancel, the release can be pending if the remote node is down. In that situation, the local files will remain on disk until the remote node can be contacted.

Work force-release
^^^^^^^^^^^^^^^^^^

It might be preferable to force a release, using the ``work force-release`` command. This will do a one-time attempt to connect to the remote node and issue a work release there. After this one attempt, it will then proceed to delete all local files associated with the work unit.

States
^^^^^^^^^^^

A unit of work can be in Pending, Running, Succeeded, or Failed state

For local work, transitioning from Pending to Running occurs the moment the ``command`` executable is started

For remote work, transitioning from Pending to Running occurs when the status reported from the remote node has a Running state.

Signed work
^^^^^^^^^^^^^^^^^^

Remote work submissions can be digitally signed by the sender. The target node will verify the signature of the work command before starting the work unit.

A pair of RSA public and private keys are created offline and distributed to the nodes. The public key should be on the node receiving work (PKIX format). The private key should be on the node submitting work (PKCS1 format).

The following commands can be used to create keys for signing work:

.. code::

    openssl genrsa -out signworkprivate.pem 2048
    openssl rsa -in signworkprivate.pem -pubout -out signworkpublic.pem

in `bar.yml`

.. code-block:: yaml

    # PKIX
    - work-verification:
        publickey: /full/path/signworkpublic.pem

    - work-command:
        workType: echopayload
        command: bash
        params: "-c \"while read -r line; do echo ${line^^}; sleep 5; done\""
        verifysignature: true

in `foo.yml`

.. code-block:: yaml

    # PKCS1
    - work-signing:
        privatekey: /full/path/signworkprivate.pem
        tokenexpiration: 30m

Tokenexpiration determines how long a the signature is valid for. This expiration directly corresponds to the "expiresAt" field in the generated JSON web token. Valid units include "h" and "m", e.g. 1h30m for one hour and 30 minutes.

Use the "--signwork" parameter to sign the work.

.. code::

    $ receptorctl --socket /tmp/foo.sock work submit echoint --node bar --no-payload --signwork

Units on disk
^^^^^^^^^^^^^^^^^^

Netceptor, the main component of receptor that handles mesh connectivity and traffic, operates entirely in memory. That is, it does not store any state information on disk. However, Workceptor functionality is designed to be persistent across receptor restarts. Work units might be running commands that could take hours to complete, and as such needs to store some relevant information on disk in case the receptor process restarts.

By default receptor stores data under ``/tmp/receptor`` but can be changed by setting the ``datadir`` param under the ``node`` action in the config file.

For a given work unit, receptor will store files in ``{datadir}/{nodeID}/{unitID}/``.

Here is the receptor directory tree after running ``work submit echopayload`` described in :ref:`work_payload`.

.. code::

    $ tree /tmp/receptor
    /tmp/receptor
    ├── bar
    │   └── NImim5WA
    │       ├── status
    │       ├── status.lock
    │       ├── stdin
    │       └── stdout
    └── foo
        └── BsAjS4wi
            ├── status
            ├── status.lock
            ├── stdin
            └── stdout

The main purpose of work unit ``BsAjS4wi`` on `foo` is to copy stdin, stdout, and status from ``NImim5WA`` on `bar` back to its own working directory.

``stdin`` is a copy of the submitted payload. The contents of this file is the same on both the local (`foo`) and remote (`bar`) machines.

.. code::

    $ cat /tmp/receptor/bar/NImim5WA/stdin
    hi
    i am foo
    what is your name

``stdout`` contains the work unit results; the stdout of the command execution. It will also be the same on both the local node and remote node.

.. code::

    $ cat /tmp/receptor/bar/NImim5WA/stdout
    HI
    I AM FOO
    WHAT IS YOUR NAME

``status`` contains additional information related to the work unit. The contents of status are different on `foo` and `bar`.

.. code::

    $ cat /tmp/receptor/bar/NImim5WA/stdout
    {
       "State":2,
       "Detail":"exit status 0",
       "StdoutSize":30,
       "WorkType":"echopayload",
       "ExtraData":null
    }

.. code::

    $ cat /tmp/receptor/foo/BsAjS4wi/stdout
    {
       "State":2,
       "Detail":"exit status 0",
       "StdoutSize":30,
       "WorkType":"remote",
       "ExtraData":{
          "RemoteNode":"bar",
          "RemoteWorkType":"echopayload",
          "RemoteParams":{},
          "RemoteUnitID":"NImim5WA",
          "RemoteStarted":true,
          "LocalCancelled":false,
          "LocalReleased":false,
          "TLSClient":"",
          "Expiration":"0001-01-01T00:00:00Z"
       }
    }


.. image:: remote.png

The sequence of events during a work remote submission. Blue lines indicate moments when receptor writes files to disk.
