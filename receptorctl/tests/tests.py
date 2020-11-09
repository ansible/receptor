import pytest
import subprocess
import os
import shutil
from receptorctl import ReceptorControl
import time

connDict = {
    "socket":None,
    "rootcas":None,
    "key":None,
    "cert":None,
    "insecureskipverify":False,
}

tmpDir = "/tmp/receptorctltest"
if os.path.exists(tmpDir):
    shutil.rmtree(tmpDir)
os.mkdir(tmpDir)

def generate_cert(name, commonName):
    keyPath = os.path.join(tmpDir, name + ".key")
    crtPath = os.path.join(tmpDir, name + ".crt")
    os.system("openssl genrsa -out " + keyPath + " 1024")
    os.system("openssl req -x509 -new -nodes -key " + keyPath + " -subj /C=/ST=/L=/O=/OU=ReceptorTesting/CN=ca -sha256 -out " + crtPath)
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
    os.system("openssl genrsa -out " + keyPath + " 1024")
    # create cert request
    os.system("openssl req -new -sha256 -key " + keyPath + " -subj /C=/ST=/L=/O=/OU=ReceptorTesting/CN=" + commonName + " -out " + csrPath)
    # sign cert request
    os.system("openssl x509 -req -extfile " + extPath + " -in " + csrPath + " -CA " + caCrtPath + " -CAkey " + caKeyPath + " -CAcreateserial -out " + crtPath + " -sha256")
    return keyPath, crtPath

caKeyPath, caCrtPath = generate_cert("ca", "ca")
clientKeyPath, clientCrtPath = generate_cert_with_ca("client", caKeyPath, caCrtPath, "localhost")
generate_cert_with_ca("server", caKeyPath, caCrtPath, "localhost")

@pytest.fixture(scope="class")
def receptor_mesh(request):

    node1 = subprocess.Popen(["receptor", "-c", "tests/mesh-definitions/mesh1/node1.yaml"])
    node2 = subprocess.Popen(["receptor", "-c", "tests/mesh-definitions/mesh1/node2.yaml"])

    time.sleep(0.5)
    node1_controller = ReceptorControl()
    connDict["socket"] = "unix://" + os.path.join(tmpDir, "node1.sock")
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
        connDict["socket"] = "unix://" + os.path.join(tmpDir, "node1.sock")
        node1_controller.connect(connDict)
        status = node1_controller.simple_command("status")
        node1_controller.close()
        assert not (set(["Advertisements", "Connections", "KnownConnectionCosts", "NodeID", "RoutingTable"]) - status.keys())

    def test_simple_command_fail(self):
        node1_controller = ReceptorControl()
        connDict["socket"] = "unix://" + os.path.join(tmpDir, "node1.sock")
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
        connDict["rootcas"] = caCrtPath
        connDict["insecureskipverify"] = True
        connDict["key"] = clientKeyPath
        connDict["cert"] = clientCrtPath
        node1_controller.connect(connDict)
        status = node1_controller.simple_command("status")
        node1_controller.close()
        assert not (set(["Advertisements", "Connections", "KnownConnectionCosts", "NodeID", "RoutingTable"]) - status.keys())

    def test_connect_to_service(self):
        node1_controller = ReceptorControl()
        connDict["socket"] = "unix://" + os.path.join(tmpDir, "node1.sock")
        node1_controller.connect(connDict)
        node1_controller.connect_to_service("node2", "control", "")
        node1_controller.handshake()
        status = node1_controller.simple_command("status")
        node1_controller.close()
        assert status["NodeID"] == "node2"
