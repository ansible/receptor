import os
import subprocess


def __init__():
    pass


def create_certificate(tmp_dir: str):
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
            ext.write("subjectAltName=DNS:" + commonName)
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
        "client", caKeyPath, caCrtPath, "localhost"
    )
    generate_cert_with_ca("server", caKeyPath, caCrtPath, "localhost")

    return {
        "caKeyPath": caKeyPath,
        "caCrtPath": caCrtPath,
        "clientKeyPath": clientKeyPath,
        "clientCrtPath": clientCrtPath,
    }
