TLS support
===========

Receptor supports mutual TLS authentication and encryption for above and below the
mesh connections.

Configuring TLS
^^^^^^^^^^^^^^^

Add ``tls-server`` and ``tls-client`` definitions to receptor config files.

foo.yml

.. code-block:: yaml

    ---
    - node:
        id: foo

    - log-level:
        level: Debug

    - tls-server:
        name: myserver
        cert: foo.crt
        key: foo.key
        requireclientcert: true
        clientcas: ca.crt

    - tcp-listener:
        port: 2222
        tls: myserver

Having ``tls-server`` defined does nothing on its own, but it can be referenced elsewhere in the receptor config file. Above, ``tls`` in the ``tcp-listener`` is set to use ``myserver``. In general, ``tls-server`` should be referenced anywhere receptor is expecting an incoming connection, i.e. ``*-listener`` backends or on the ``control-service``. Similarly, ``tls-client`` should be referenced anywhere receptor is expecting to make an outgoing connection, i.e. ``*-peer`` backends or in receptorctl (the command-line client for receptor).

bar.yml

.. code-block:: yaml

    ---
    - node:
        id: bar

    - log-level:
        level: Debug

    - tls-client:
        name: myclient
        rootcas: /full/path/ca.crt
        insecureskipverify: false
        cert: /full/path/bar.crt
        key: /full/path/bar.key

    - tcp-peer:
        address: localhost:2222
        tls: myclient


``myclient`` is referenced in ``tcp-peer``. Once started, `foo` and `bar` will authenticate each other, and the connection will be fully encrypted.

Generating certs
^^^^^^^^^^^^^^^^

Receptor supports X.509 compliant certificates. Although numerous tools can be used to generate valid certificates, receptor has a built-in tool to help with this process. Running receptor with the ``cert-init``, ``cert-makereq``, and ``cert-signreq`` actions will create certificate authorities, make requests, and sign requests, respectively.

makecerts.sh

.. code::

    #!/bin/bash
    mkdir -p certs
    receptor --cert-init commonname="test CA" bits=2048 outcert=certs/ca.crt outkey=certs/ca.key
    for node in foo bar; do
      receptor --cert-makereq bits=2048 commonname="$node test cert" dnsname=localhost nodeid=$node outreq=certs/$node.csr outkey=certs/$node.key
      receptor --cert-signreq req=certs/$node.csr cacert=certs/ca.crt cakey=certs/ca.key outcert=certs/$node.crt
    done

The above script will create a CA, and for each node `foo` and `bar`, create a certificate request and sign it with the CA. These certs and keys can then be used to create ``tls-server`` and ``tls-client`` definitions in the receptor config files.

Above the mesh TLS
^^^^^^^^^^^^^^^^^^

Below-the-mesh TLS deals with connections that are being made to an IP address or DNS name, and so it can use normal X.509 certificates which include DNS names or IP addresses in their subjectAltName field.  However, above-the-mesh TLS deals with connections whose endpoint addresses are receptor node IDs.  This requires generating certificates that include receptor node IDs as names in the subjectAltName extension.  To do this, the otherName field of subjectAltName can be utilized.  This field is designed to accept arbitrary names of any type, and includes an ISO Object Identifier (OID) that defines what type of name this is, followed by arbitrary data that is meaningful for that type.  Red Hat has its own OID namespace, which is controlled by RHANANA, the Red Hat Assigned Names And Number Authority.  Receptor has an assignment within the overall Red Hat namespace.
