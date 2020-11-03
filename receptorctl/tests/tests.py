import pytest
import subprocess
import os
from receptorctl import ReceptorControl
import time

connDict = {
    "socket":None,
    "rootcas":None,
    "key":None,
    "cert":None,
    "insecureskipverify":False,
}

@pytest.fixture(scope="class")
def receptor_mesh(request):
    caKeyPath = "/tmp/receptorctltest_ca.key"
    caCrtPath = "/tmp/receptorctltest_ca.crt"
    keyPath = "/tmp/receptorctltest.key"
    crtPath = "/tmp/receptorctltest.crt"
    csrPath = "/tmp/receptorctltest.csa"
    extPath = "/tmp/receptorctltest.ext"
    # create x509 extension
    with open(extPath, "w") as ext:
        ext.write("subjectAltName=DNS:localhost")
        ext.close()
    # create CA
    os.system("openssl genrsa -out " + caKeyPath + " 1024")
    os.system("openssl req -x509 -new -nodes -key " + caKeyPath + " -subj /C=/ST=/L=/O=/OU=ReceptorTesting/CN=ca -sha256 -out " + caCrtPath)
    # create key
    os.system("openssl genrsa -out " + keyPath + " 1024")
    # create cert request
    os.system("openssl req -new -sha256 -key " + keyPath + " -subj /C=/ST=/L=/O=/OU=ReceptorTesting/CN=localhost -out " + csrPath)
    # sign cert request
    os.system("openssl x509 -req -extfile " + extPath + " -in " + csrPath + " -CA " + caCrtPath + " -CAkey " + caKeyPath + " -CAcreateserial -out " + crtPath + " -sha256")

    node1 = subprocess.Popen(["receptor", "-c", "tests/mesh-definitions/mesh1/node1.yaml"])
    node2 = subprocess.Popen(["receptor", "-c", "tests/mesh-definitions/mesh1/node2.yaml"])

    time.sleep(0.5)
    node1_controller = ReceptorControl()
    connDict["socket"] = "unix:///tmp/node1.sock"
    node1_controller.connect(connDict)

    while True:
        status = node1_controller.simple_command("status")
        if status["RoutingTable"] == {"node2":"node2"}:
            break

    node1_controller.close()
    yield

    node1.kill()
    node2.kill()
    node1.wait()
    node2.wait()

@pytest.mark.usefixtures('receptor_mesh')
class TestReceptorCTL:
    def test_simple_command(self):
        node1_controller = ReceptorControl()
        connDict["socket"] = "unix:///tmp/node1.sock"
        node1_controller.connect(connDict)
        status = node1_controller.simple_command("status")
        node1_controller.close()
        assert not (set(["Advertisements", "Connections", "KnownConnectionCosts", "NodeID", "RoutingTable"]) - status.keys())

    def test_simple_command_fail(self):
        node1_controller = ReceptorControl()
        connDict["socket"] = "unix:///tmp/node1.sock"
        node1_controller.connect(connDict)
        with pytest.raises(RuntimeError):
            node1_controller.simple_command("doesnotexist")
        node1_controller.close()

    def test_tcp_control_service(self):
        node1_controller = ReceptorControl()
        connDict["socket"] = "tcp://localhost:11112"
        node1_controller.connect(connDict)
        status = node1_controller.simple_command("status")
        node1_controller.close()
        assert not (set(["Advertisements", "Connections", "KnownConnectionCosts", "NodeID", "RoutingTable"]) - status.keys())

    def test_tcp_control_service_tls(self):
        node1_controller = ReceptorControl()
        connDict["socket"] = "tls://localhost:11113"
        connDict["rootcas"] = "/tmp/receptorctltest_ca.crt"
        connDict["insecureskipverify"] = True
        node1_controller.connect(connDict)
        status = node1_controller.simple_command("status")
        node1_controller.close()
        assert not (set(["Advertisements", "Connections", "KnownConnectionCosts", "NodeID", "RoutingTable"]) - status.keys())

    def test_connect_to_service(self):
        node1_controller = ReceptorControl()
        connDict["socket"] = "unix:///tmp/node1.sock"
        node1_controller.connect(connDict)
        node1_controller.connect_to_service("node2", "control", "")
        node1_controller.handshake()
        status = node1_controller.simple_command("status")
        node1_controller.close()
        assert status["NodeID"] == "node2"
