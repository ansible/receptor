TLS support
===========

Receptor supports mutual TLS authentication and encryption for above and below the
mesh connections.

.. contents::
   :local:

Configuring TLS
---------------

Add ``tls-servers`` and ``tls-clients`` definitions to Receptor configuration files.

``foo.yml``

.. code-block:: yaml

    ---
    version: 2
    node:
      id: foo

    log-level:
      level: Debug

    tls-servers:
      - name: myserver
        cert: /full/path/foo.crt
        key: /full/path/foo.key
        requireclientcert: true
        clientcas: /full/path/ca.crt

    tcp-listeners:
      - port: 2222
        tls: myserver

Defining ``tls-servers`` has no effect, but it can be referenced elsewhere in the Receptor configuration file.
In the preceding configuration snippet, ``tls`` in the ``tcp-listeners`` is set to use ``myserver``.
In general, ``tls-servers`` should be referenced anywhere Receptor is expecting an incoming connection, such as ``*-listeners`` backends or on the ``control-services``.
Similarly, ``tls-clients`` should be referenced anywhere Receptor is expecting to make an outgoing connection, such as ``*-peers`` backends or in ``receptorctl`` (the command-line client for Receptor).

``bar.yml``

.. code-block:: yaml

    ---
    version: 2
    node:
      id: bar

    log-level:
      level: Debug

    tls-clients:
      - name: myclient
        rootcas: /full/path/ca.crt
        insecureskipverify: false
        cert: /full/path/bar.crt
        key: /full/path/bar.key

    tcp-peers:
      - address: localhost:2222
        tls: myclient


``myclient`` is referenced in ``tcp-peers``. Once started, `foo` and `bar` will authenticate each other, and the connection will be fully encrypted.

Generating certs
-----------------

Receptor supports X.509 compliant certificates and provides a built-in tool to generate valid certificates.
Running and configuring Receptor with the ``cert-init``, ``cert-makereqs``, and ``cert-signreqs`` properties creates certificate authorities, make requests, and sign requests.

``makecerts.sh``

.. code-block:: bash

    #!/bin/bash
    mkdir -p certs
    receptor --cert-init commonname="test CA" bits=2048 outcert=certs/ca.crt outkey=certs/ca.key
    for node in foo bar; do
      receptor --cert-makereq bits=2048 commonname="$node test cert" dnsname=localhost nodeid=$node outreq=certs/$node.csr outkey=certs/$node.key
      receptor --cert-signreq req=certs/$node.csr cacert=certs/ca.crt cakey=certs/ca.key outcert=certs/$node.crt
    done

The preceding script will create a CA, and for each node ``foo`` and ``bar``, create a certificate request and sign it with the CA.
These certificates and keys can then create ``tls-servers`` and ``tls-clients`` definitions in the Receptor configuration files.

Pinned certificates
--------------------

In a case where a TLS connection is only ever going to be made between two well-known nodes, it may be preferable to
require a specific certificate rather than accepting any certificate signed by a CA.  Receptor supports certificate
pinning for this purpose.  Here is an example of a pinned certificate configuration:

.. code-block:: yaml

    ---
    version: 2
    node:
      id: foo

    tls-servers:
      - name: myserver
        cert: /full/path/foo.crt
        key: /full/path/foo.key
        requireclientcert: true
        clientcas: /full/path/ca.crt
        pinnedclientcert:
          - E6:9B:98:A7:A5:DB:17:D6:E4:2C:DE:76:45:42:A8:79:A3:0A:C5:6D:10:42:7A:6A:C4:54:57:83:F1:0F:E2:95

    tcp-listeners:
      - port: 2222
        tls: myserver

Certificate pinning is an added requirement, and does not eliminate the need to meet other stated requirements.  In the above example, the client certificate must both be signed by a CA in the `ca.crt` bundle, and also have the listed fingerprint.  Multiple fingerprints may be specified, in which case a certificate matching any one of them will be accepted.

To find the fingerprint of a given certificate, use the following OpenSSL command:

.. code-block:: bash

   openssl x509 -in my-cert.pem -noout -fingerprint -sha256

SHA256 and SHA512 fingerprints are supported.  SHA1 fingerprints are not supported due to the insecurity of the SHA1 algorithm.


Above the mesh TLS
-------------------

Below-the-mesh TLS deals with connections that are being made to an IP address or DNS name, and so it can use normal X.509 certificates which include DNS names or IP addresses in their ``subjectAltName`` field.
Above-the-mesh TLS deals with connections that use Receptor node IDs as endpoint addresses, which require generating certificates that include Receptor node IDs as names in the ``subjectAltName`` extension.
You can use the ``otherName`` field of ``subjectAltName`` to specify Receptor node IDs.
The ``otherName`` field accepts arbitrary names of any type, and includes an ISO Object Identifier (OID) that defines what type of name this is, followed by arbitrary data that is meaningful for that type.
Red Hat has its own OID namespace, which is controlled by RHANANA, the Red Hat Assigned Names And Number Authority.
Receptor has an assignment within the overall Red Hat namespace.

If you use TLS authentication in your mesh, the certificates OIDs (1.3.6.1.4.1.2312.19.1) will be verified against the `node.id` specified in the configuration file. If there is no match, the Receptor binary will hard exit. To avoid this check, visit the `Skip Certificate Validation`_ section for more details.


Skip certificate validation
----------------------------

You can turn off certificate validation by adding a `skipreceptornamescheck` key-value pair to your configuration.  Depending on the specifics of your environment(s), you may need to add the ``skipreceptornamescheck`` key-value pair to the configuration file for `tls-server`, `tls-config`, or both.
The default behavior for this option is `false` which means that the certificate's OIDs will be verified against the node ID.

.. code-block:: yaml

    ---
    version: 2
    node:
      id: bar

    log-level:
      level: Debug

    tls-clients:
      - name: myclient
        rootcas: /full/path/ca.crt
        insecureskipverify: false
        cert: /full/path/bar.crt
        key: /full/path/bar.key
        skipreceptornamescheck: true

    tls-servers:
      - name: myserver
        cert: /full/path/foo.crt
        key: /full/path/foo.key
        requireclientcert: true
        clientcas: /full/path/ca.crt
        pinnedclientcert:
          - E6:9B:98:A7:A5:DB:17:D6:E4:2C:DE:76:45:42:A8:79:A3:0A:C5:6D:10:42:7A:6A:C4:54:57:83:F1:0F:E2:95
        skipreceptornamescheck: true

    tcp-peers:
      - address: localhost:2222
        tls: myclient
