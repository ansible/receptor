Firewall Rules
==============


Blah blah blah

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
    # Rejects traffic originating from nodes like abcb, adfb, etc

.. code-block:: yaml
    ---
    node:
    firewallrules:
        - action: "reject"
        fromnode: "/a.*b/"
    # Rejects traffic destined for nodes like abcb, AdfB, etc

    ---
    node:
    firewallrules:
        - action: "reject"
        tonode: "/(?i)a.*b/"
