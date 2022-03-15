import receptorctl

import pytest
import subprocess
import os
import shutil
import time
import json
import yaml
from click.testing import CliRunner

from lib import create_certificate


@pytest.fixture(scope="session")
def base_tmp_dir():
    receptor_tmp_dir = "/tmp/receptor"
    base_tmp_dir = "/tmp/receptorctltest"

    # Clean up tmp directory and create a new one
    if os.path.exists(base_tmp_dir):
        shutil.rmtree(base_tmp_dir)
    os.mkdir(base_tmp_dir)

    yield base_tmp_dir

    # Tear-down
    # if os.path.exists(base_tmp_dir):
    #     shutil.rmtree(base_tmp_dir)

    if os.path.exists(receptor_tmp_dir):
        shutil.rmtree(receptor_tmp_dir)

    subprocess.call(["killall", "receptor"])


@pytest.fixture(scope="class")
def receptor_mesh(base_tmp_dir):
    class ReceptorMeshSetup:
        # Relative dir to the receptorctl tests
        mesh_definitions_dir = "tests/mesh-definitions"

        def __init__(self):
            # Default vars
            self.base_tmp_dir = base_tmp_dir

            # Required dependencies
            self.__check_dependencies()

        def setup(self, mesh_name: str = "mesh1", socket_file_name: str = "node1.sock"):
            self.mesh_name = mesh_name
            self.__change_config_files_dir(mesh_name)
            self.__create_tmp_dir()
            self.__create_certificates()
            self.socket_file_name = socket_file_name

            # HACK this should be a dinamic way to select a node socket
            self.default_socket_unix = "unix://" + os.path.join(
                self.get_mesh_tmp_dir(), socket_file_name
            )

        def default_receptor_controller_unix(self):
            return receptorctl.ReceptorControl(self.default_socket_unix)

        def __change_config_files_dir(self, mesh_name: str):
            self.config_files_dir = "{}/{}".format(self.mesh_definitions_dir, mesh_name)
            self.config_files = []

            # Iterate over all the files in the config_files_dir
            # and create a list of all files that end with .yaml or .yml
            for f in os.listdir(self.config_files_dir):
                if f.endswith(".yaml") or f.endswith(".yml"):
                    self.config_files.append(os.path.join(self.config_files_dir, f))

        def __create_certificates(self):
            self.certificate_files = create_certificate(
                self.get_mesh_tmp_dir(), "node1"
            )

        def get_mesh_name(self):
            return self.config_files_dir.split("/")[-1]

        def get_mesh_tmp_dir(self):
            mesh_tmp_dir = "{}/{}".format(self.base_tmp_dir, self.mesh_name)
            return mesh_tmp_dir

        def __check_dependencies(self):
            """Check if we have the required dependencies
            raise an exception if we don't
            """

            # Check if openssl binary is on the path
            try:
                subprocess.check_output(["openssl", "version"])
            except FileNotFoundError:
                raise Exception(
                    "openssl binary not found\n"
                    'Consider run "sudo dnf install openssl"'
                )

        def __create_tmp_dir(self):
            mesh_tmp_dir_path = self.get_mesh_tmp_dir()

            # Clean up tmp directory and create a new one
            if os.path.exists(mesh_tmp_dir_path):
                shutil.rmtree(mesh_tmp_dir_path)
            os.mkdir(mesh_tmp_dir_path)

    return ReceptorMeshSetup()


@pytest.fixture(scope="class")
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
        "../../tests/artifacts-output/",
        "receptor",
    )
    if os.path.exists(receptor_bin_path_from_test_dir):
        return receptor_bin_path_from_test_dir

    # Check if the receptor binary is in '../../' and returns
    # the path to the binary if it is found.
    receptor_bin_path_from_test_dir = os.path.join(
        os.path.dirname(os.path.abspath(__file__)),
        "../../",
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
def default_socket_tcp():
    return "tcp://localhost:11112"


@pytest.fixture(scope="class")
def default_socket_file(receptor_mesh):
    return receptor_mesh.get_mesh_tmp_dir() + "/node1.sock"


@pytest.fixture(scope="class")
def default_receptor_controller_socket_file(default_socket_file):
    return receptorctl.ReceptorControl(default_socket_file)


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
def receptor_nodes():
    class ReceptorNodes:
        nodes = []
        log_files = []

    return ReceptorNodes()


def receptor_nodes_kill(nodes):
    for node in nodes:
        node.kill()

    for node in nodes:
        node.wait(3)


def import_config_from_node(node):
    """Receive a node and return the config file as a dict"""
    stream = open(node.args[2], "r")
    try:
        config_unflatten = yaml.safe_load(stream)
    except yaml.YAMLError as e:
        raise e
    stream.close()

    config = {}
    for c in config_unflatten:
        config.update(c)

    return config


def receptor_mesh_wait_until_ready(nodes, receptor_controller):
    time.sleep(0.5)

    # Try up to 6 times
    tries = 0
    while True:
        status = receptor_controller.simple_command("status")
        # Check if it has three known nodes
        if len(status["KnownConnectionCosts"]) == 3:
            break
        tries += 1
        if tries > 6:
            raise Exception("Receptor Mesh did not start up")
        time.sleep(1)

    receptor_controller.close()


@pytest.fixture(scope="class")
def certificate_files(receptor_mesh):
    return receptor_mesh.certificate_files


@pytest.fixture(scope="class")
def default_receptor_controller_unix(receptor_mesh):
    return receptor_mesh.default_receptor_controller_unix()


def start_nodes(receptor_mesh, receptor_nodes, receptor_bin_path):
    for i, config_file in enumerate(receptor_mesh.config_files):
        log_file_name = (
            config_file.split("/")[-1].replace(".yaml", ".log").replace(".yml", ".log")
        )
        receptor_nodes.log_files.append(
            open(
                os.path.join(receptor_mesh.get_mesh_tmp_dir(), log_file_name),
                "w",
            )
        )
        receptor_nodes.nodes.append(
            subprocess.Popen(
                [receptor_bin_path, "-c", config_file],
                stdout=receptor_nodes.log_files[i],
                stderr=receptor_nodes.log_files[i],
            )
        )


@pytest.fixture(scope="class")
def receptor_mesh_mesh1(
    receptor_bin_path,
    receptor_nodes,
    receptor_mesh,
):
    # Set custom config files dir
    receptor_mesh.setup("mesh1")

    # Start the receptor nodes processes
    start_nodes(receptor_mesh, receptor_nodes, receptor_bin_path)

    receptor_mesh_wait_until_ready(
        receptor_nodes.nodes, receptor_mesh.default_receptor_controller_unix()
    )

    yield

    receptor_nodes_kill(receptor_nodes.nodes)


@pytest.fixture(scope="class")
def receptor_mesh_access_control(
    receptor_bin_path,
    receptor_nodes,
    receptor_mesh,
):
    # Set custom config files dir
    receptor_mesh.setup("access_control", "node2.sock")

    # Create PEM key for signed work
    key_path = os.path.join(receptor_mesh.get_mesh_tmp_dir(), "signwork_key")
    subprocess.check_output(
        [
            "ssh-keygen",
            "-b",
            "2048",
            "-t",
            "rsa",
            "-f",
            key_path,
            "-q",
            "-N",
            "",
        ]
    )

    # Start the receptor nodes processes
    start_nodes(receptor_mesh, receptor_nodes, receptor_bin_path)

    receptor_mesh_wait_until_ready(
        receptor_nodes.nodes, receptor_mesh.default_receptor_controller_unix()
    )

    yield

    receptor_nodes_kill(receptor_nodes.nodes)


@pytest.fixture(scope="function")
def receptor_control_args(receptor_mesh):
    args = {
        "--socket": f"{receptor_mesh.get_mesh_tmp_dir()}/{receptor_mesh.socket_file_name}",
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

        # Since we may log errors/warnings on stderr we want to split stdout and stderr
        runner = CliRunner(mix_stderr=False)

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
            # JSON data should only be on stdout
            json_output = json.loads(result.stdout)
        except json.decoder.JSONDecodeError:
            pytest.fail("The command is not in json format")
        return result, json_output

    return f_invoke_as_json
