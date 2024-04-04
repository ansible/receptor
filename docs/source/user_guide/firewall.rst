.. _firewall_rules:

Firewall Rules
==============

Receptor has the ability to accept, drop, or reject traffic based on any combination of the following:

- ``FromNode``
- ``ToNode``
- ``FromService``
- ``ToService``

Firewall rules are added under the ``node`` entry in a Receptor configuration file:

.. code-block:: yaml

    # Accepts everything
    ---
    node:
      firewallrules:
        - action: "accept"

.. code-block:: yaml

    # Drops traffic from `foo` to `bar`'s control service
    ---
    node:
      firewallrules:
        - action: "drop"
          fromnode: "foo"
          tonode: "bar"
          toservice: "control"

.. code-block:: yaml

    # Rejects traffic originating from nodes like abcb, adfb, etc
    ---
    node:
      firewallrules:
        - action: "reject"
          fromnode: "/a.*b/"

.. code-block:: yaml

    # Rejects traffic destined for nodes like abcb, AdfB, etc
    ---
    node:
      firewallrules:
        - action: "reject"
          tonode: "/(?i)a.*b/"
