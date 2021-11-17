import sys

sys.path.append("../receptorctl")

import receptorctl

import pytest
import subprocess
import os
import shutil
import time
import json
from click.testing import CliRunner

tmpDir = "/tmp/receptorctltest"


@pytest.fixture(scope="session")
def create_empty_dir():
    def check_dependencies():
        """Check if we have the required dependencies
        raise an exception if we don't
        """

        # Check if openssl binary is on the path
        try:
            subprocess.check_output(["openssl", "version"])
        except Exception:
            raise Exception(
                "openssl binary not found\n" 'Consider run "sudo dnf install openssl"'
            )

    check_dependencies()

    # Clean up tmp directory and create a new one
    if os.path.exists(tmpDir):
        shutil.rmtree(tmpDir)
    os.mkdir(tmpDir)


@pytest.fixture(scope="session")
def create_certificate(create_empty_dir):
    def generate_cert(name, commonName):
        keyPath = os.path.join(tmpDir, name + ".key")
        crtPath = os.path.join(tmpDir, name + ".crt")
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
        keyPath = os.path.join(tmpDir, name + ".key")
        crtPath = os.path.join(tmpDir, name + ".crt")
        csrPath = os.path.join(tmpDir, name + ".csa")
        extPath = os.path.join(tmpDir, name + ".ext")

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


@pytest.fixture(scope="session")
def certificate_files(create_certificate):
    """Returns a dict with the certificate files

    The dict contains the following keys:
        caKeyPath
        caCrtPath
        clientKeyPath
        clientCrtPath
    """
    return create_certificate


@pytest.fixture(scope="session")
def prepare_environment(certificate_files):
    pass


@pytest.fixture(scope="session")
def receptor_bin_path():
    """Returns the path to the receptor binary

        This fixture was created to make possible the use of
        multiple receptor binaries files.

        The default priority order is:
        - ../../tests/artifacts-output
        - The "receptor" available in the PATH

    Returns:
        str: Path to the receptor binary
    """

    # Check if the receptor binary is in '../../tests/artifacts-output' and returns
    # the path to the binary if it is found.
    receptor_bin_path_from_test_dir = os.path.join(
        os.path.dirname(os.path.abspath(__file__)),
        "../../tests/artifacts-output",
        "receptor",
    )
    if os.path.exists(receptor_bin_path_from_test_dir):
        return receptor_bin_path_from_test_dir

    # Check if the receptor binary is in the path
    try:
        subprocess.check_output(["receptor", "--version"])
        return "receptor"
    except subprocess.CalledProcessError:
        raise Exception(
            "Receptor binary not found in $PATH or in '../../tests/artifacts-output'"
        )


@pytest.fixture(scope="class")
def default_socket_unix():
    return "unix://" + os.path.join(tmpDir, "node1.sock")


@pytest.fixture(scope="class")
def default_receptor_controller_unix(default_socket_unix):
    return receptorctl.ReceptorControl(default_socket_unix)


@pytest.fixture(scope="class")
def default_socket_tcp():
    return "tcp://localhost:11112"


@pytest.fixture(scope="class")
def default_receptor_controller_tcp(default_socket_tcp):
    return receptorctl.ReceptorControl(default_socket_tcp)


@pytest.fixture(scope="class")
def default_receptor_controller_tcp_tls(default_socket_tcp, certificate_files):
    rootcas = certificate_files["caCrtPath"]
    key = certificate_files["clientKeyPath"]
    cert = certificate_files["clientCrtPath"]
    insecureskipverify = True

    controller = receptorctl.ReceptorControl(
        default_socket_tcp,
        rootcas=rootcas,
        key=key,
        cert=cert,
        insecureskipverify=insecureskipverify,
    )

    return controller


@pytest.fixture(scope="class")
def receptor_mesh(
    prepare_environment, receptor_bin_path, default_receptor_controller_unix
):

    node1 = subprocess.Popen(
        [receptor_bin_path, "-c", "tests/mesh-definitions/mesh1/node1.yaml"]
    )
    node2 = subprocess.Popen(
        [receptor_bin_path, "-c", "tests/mesh-definitions/mesh1/node2.yaml"]
    )
    node3 = subprocess.Popen(
        [receptor_bin_path, "-c", "tests/mesh-definitions/mesh1/node3.yaml"]
    )

    time.sleep(0.5)
    node1_controller = default_receptor_controller_unix

    while True:
        status = node1_controller.simple_command("status")
        if status["RoutingTable"] == {"node2": "node2", "node3": "node2"}:
            break
        time.sleep(0.5)

    node1_controller.close()

    # Debug mesh data
    print("# Mesh nodes: {}".format(str(status["KnownConnectionCosts"].keys())))

    yield

    node1.kill()
    node2.kill()
    node1.wait()
    node2.wait()


@pytest.fixture(scope="function")
def receptor_control_args():
    args = {
        "--socket": "/tmp/receptorctltest/node1.sock",
        "--config": None,
        "--tls": None,
        "--rootcas": None,
        "--key": None,
        "--cert": None,
        "--insecureskipverify": None,
    }
    return args


@pytest.fixture(scope="function")
def invoke(receptor_control_args):
    def f_invoke(command, args: list = []):
        """Invoke a command and return the original result.

        Args:
            command (click command): The command to invoke.
            args (list<str>): The arguments to pass to the command.

        Returns:
            click.testing: The original result.
        """

        def parse_args_to_list(args: dict):
            """Parse the args (dict) to a list of strings."""
            arg_list = []
            for k, v in args.items():
                if v is not None:
                    arg_list.append(str(k))
                    arg_list.append(str(v))
            return arg_list

        runner = CliRunner()

        out = runner.invoke(
            receptorctl.cli.cli,
            parse_args_to_list(receptor_control_args) + [command.name] + args,
        )
        return out

    return f_invoke


@pytest.fixture(scope="function")
def invoke_as_json(invoke):
    def f_invoke_as_json(command, args: list = []):
        """Invoke a command and return the original result and the json output.

        Args:
            command (click command): The command to invoke.
            args (list<str>): The arguments to pass to the command.

        Returns:
            tuple<click.testing, dict>: Tuple of the original result and the json output.
        """
        result = invoke(command, ["--json"] + args)
        try:
            json_output = json.loads(result.output)
        except json.decoder.JSONDecodeError:
            pytest.fail("The command is not in json format")
        return result, json_output

    return f_invoke_as_json
