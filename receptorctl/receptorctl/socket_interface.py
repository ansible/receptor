import sys
import os
import re
import io
import socket
import shutil
import json
import ssl
import yaml

class ReceptorControl:
    def __init__(self, socketaddress, config=None, tlsclient=None, rootcas=None, key=None, cert=None, insecureskipverify=False):
        if config and any((rootcas, key, cert)):
            raise RuntimeError("Cannot specify both config and rootcas, key, cert")
        if config and not tlsclient:
            raise RuntimeError("Must specify both config and tlsclient")
        self.socket = None
        self.sockfile = None
        self.remote_node = None
        self.socketaddress = socketaddress
        self.rootcas = rootcas
        self.key = key
        self.cert = cert
        self.insecureskipverify = insecureskipverify
        if config and tlsclient:
            self.readconfig(config, tlsclient)
        self.connect()

    def readstr(self):
        return self.sockfile.readline().decode().strip()

    def writestr(self, str):
        self.sockfile.write(str.encode())
        self.sockfile.flush()

    def handshake(self):
        m = re.compile("Receptor Control, node (.+)").fullmatch(self.readstr())
        if not m:
            raise RuntimeError("Failed to connect to Receptor socket")
        self.remote_node = m[1]

    def read_and_parse_json(self):
        text = self.readstr()
        if str.startswith(text, "ERROR:"):
            raise RuntimeError(text[7:])
        data = json.loads(text)
        return data

    def readconfig(self, config, tlsclient):
        with open(config, "r") as yamlfid:
            yamldata = yaml.load(yamlfid, Loader=yaml.FullLoader)
            yamlfid.close()
        for i in yamldata:
            key = i.get("tls-client", None)
            if key:
                 if key["name"] == tlsclient:
                     self.rootcas = key.get("rootcas", self.rootcas)
                     self.key = key.get("key", self.key)
                     self.cert = key.get("cert", self.cert)
                     self.insecureskipverify = key.get("insecureskipverify", self.insecureskipverify)
                     break

    def simple_command(self, command):
        self.connect()
        self.writestr(f"{command}\n")
        return self.read_and_parse_json()

    def connect(self):
        if self.socket is not None:
            return
        m = re.compile("(tcp|tls):(//)?([a-zA-Z0-9-]+):([0-9]+)|(unix:(//)?)?([^:]+)").fullmatch(self.socketaddress)
        if m:
            unixsocket = m[7]
            host = m[3]
            port = m[4]
            protocol = m[1]
            if unixsocket:
                path = os.path.expanduser(unixsocket)
                if not os.path.exists(path):
                    raise ValueError(f"Socket path does not exist: {path}")
                self.socket = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
                self.socket.connect(path)
                self.sockfile = self.socket.makefile('rwb')
                self.handshake()
                return
            elif host and port:
                self.socket = None
                addrs = socket.getaddrinfo(host, port, socket.AF_UNSPEC, socket.SOCK_STREAM, 0, socket.AI_PASSIVE)
                for addr in addrs:
                    family, type, proto, canonname, sockaddr = addr
                    try:
                        self.socket = socket.socket(family, type, proto)
                    except OSError:
                        self.socket = None
                        continue
                    try:
                        if protocol == "tls":
                            context = ssl.create_default_context(purpose=ssl.Purpose.SERVER_AUTH, cafile=self.rootcas)
                            if self.key and self.cert:
                                context.load_cert_chain(certfile=self.cert, keyfile=self.key)
                            if self.insecureskipverify:
                                context.check_hostname = False
                            self.socket = context.wrap_socket(self.socket, server_hostname=host)
                        self.socket.connect(sockaddr)
                    except OSError:
                        self.socket.close()
                        self.socket = None
                        continue
                    self.sockfile = self.socket.makefile('rwb')
                    break
                if self.socket is None:
                    raise ValueError(f"Could not connect to host {host} port {port}")
                self.handshake()
                return
        raise ValueError(f"Invalid socket address {self.socketfile}")

    def close(self):
        if self.sockfile is not None:
            try:
                self.sockfile.close()
            except:
                pass
            self.sockfile = None

        if self.socket is not None:
            try:
                self.socket.close()
            except:
                pass
            self.socket = None

    def connect_to_service(self, node, service, tlsclient):
        self.connect()
        self.writestr(f"connect {node} {service} {tlsclient}\n")
        text = self.readstr()
        if not str.startswith(text, "Connecting"):
            raise RuntimeError(text)

    def submit_work(self, worktype, payload, node=None, tlsclient=None, ttl=None, params=None):
        self.connect()
        if node is None:
            node = "localhost"

        commandMap = {
            "command": "work",
            "subcommand": "submit",
            "node": node,
            "worktype": worktype,
        }

        if tlsclient:
            commandMap['tlsclient'] = tlsclient

        if ttl:
            commandMap['ttl'] = ttl

        if params:
            for k,v in params.items():
                if k not in commandMap:
                    commandMap[k] = v
                else:
                    raise RuntimeError(f"Duplicate or illegal parameter {k}")
        commandJson = json.dumps(commandMap)
        command = f"{commandJson}\n"
        self.writestr(command)
        text = self.readstr()
        m = re.compile("Work unit created with ID (.+). Send stdin data and EOF.").fullmatch(text)
        if not m:
            errmsg = "Failed to start work unit"
            if str.startswith(text, "ERROR: "):
                errmsg = errmsg + ": " + text[7:]
            raise RuntimeError(errmsg)
        if isinstance(payload, io.IOBase):
            shutil.copyfileobj(payload, self.sockfile)
        elif isinstance(payload, str):
            self.writestr(payload)
        elif isinstance(payload, bytes):
            self.sockfile.write(payload)
        else:
            raise RuntimeError("Unknown payload type")
        self.sockfile.flush()
        self.socket.shutdown(socket.SHUT_WR)
        text = self.readstr()
        self.close()
        if text.startswith("ERROR:"):
            raise RuntimeError(f"Remote error: {text}")
        result = json.loads(text)
        return result

    def get_work_results(self, unit_id):
        self.connect()
        self.writestr(f"work results {unit_id}\n")
        text = self.readstr()
        m = re.compile("Streaming results for work unit (.+)").fullmatch(text)
        if not m:
            errmsg = "Failed to get results"
            if str.startswith(text, "ERROR: "):
                errmsg = errmsg + ": " + text[7:]
            raise RuntimeError(errmsg)
        self.socket.shutdown(socket.SHUT_WR)
        return self.sockfile
