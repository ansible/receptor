import os
import subprocess

__OIDReceptorName = "1.3.6.1.4.1.2312.19.1"
__OIDReceptorNameFormat = "UTF8"


def __init__():
    pass


def create_certificate(tmp_dir: str, commonName: str = "localhost"):
    def generate_cert(name, commonName):
        keyPath = os.path.join(tmp_dir, name + ".key")
        crtPath = os.path.join(tmp_dir, name + ".crt")
        subprocess.check_output(["openssl", "genrsa", "-out", keyPath, "2048"])
        subprocess.check_output(
            [
                "openssl",
                "req",
                "-x509",
                "-new",
                "-nodes",
                "-key",
                keyPath,
                "-subj",
                "/C=/ST=/L=/O=/OU=ReceptorTesting/CN=ca",
                "-sha256",
                "-out",
                crtPath,
            ]
        )
        return keyPath, crtPath

    def generate_cert_with_ca(name, caKeyPath, caCrtPath, commonName):
        keyPath = os.path.join(tmp_dir, name + ".key")
        crtPath = os.path.join(tmp_dir, name + ".crt")
        csrPath = os.path.join(tmp_dir, name + ".csa")
        extPath = os.path.join(tmp_dir, name + ".ext")

        # create x509 extension
        with open(extPath, "w") as ext:
            # DNSName to SAN
            ext.write("subjectAltName=DNS:" + commonName)
            # Receptor NodeID (otherName) to SAN
            ext.write(
                ",otherName:"
                + __OIDReceptorName
                + ";"
                + __OIDReceptorNameFormat
                + ":"
                + commonName
            )
            ext.close()
        subprocess.check_output(["openssl", "genrsa", "-out", keyPath, "2048"])

        # create cert request
        subprocess.check_output(
            [
                "openssl",
                "req",
                "-new",
                "-sha256",
                "-key",
                keyPath,
                "-subj",
                "/C=/ST=/L=/O=/OU=ReceptorTesting/CN=" + commonName,
                "-out",
                csrPath,
            ]
        )

        # sign cert request
        subprocess.check_output(
            [
                "openssl",
                "x509",
                "-req",
                "-extfile",
                extPath,
                "-in",
                csrPath,
                "-CA",
                caCrtPath,
                "-CAkey",
                caKeyPath,
                "-CAcreateserial",
                "-out",
                crtPath,
                "-sha256",
            ]
        )

        return keyPath, crtPath

    # Create a new CA
    caKeyPath, caCrtPath = generate_cert("ca", "ca")
    clientKeyPath, clientCrtPath = generate_cert_with_ca(
        "client", caKeyPath, caCrtPath, commonName
    )
    generate_cert_with_ca("server", caKeyPath, caCrtPath, commonName)

    return {
        "caKeyPath": caKeyPath,
        "caCrtPath": caCrtPath,
        "clientKeyPath": clientKeyPath,
        "clientCrtPath": clientCrtPath,
    }
