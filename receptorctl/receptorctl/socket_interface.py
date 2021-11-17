import os
import re
import io
import socket
import shutil
import json
import ssl
import yaml


def shutdown_write(sock):
    if isinstance(sock, ssl.SSLSocket):
        super(ssl.SSLSocket, sock).shutdown(socket.SHUT_WR)
    else:
        sock.shutdown(socket.SHUT_WR)


class ReceptorControl:
    def __init__(
        self,
        socketaddress,
        config=None,
        tlsclient=None,
        rootcas=None,
        key=None,
        cert=None,
        insecureskipverify=False,
    ):
        if config and any((rootcas, key, cert)):
            raise RuntimeError("Cannot specify both config and rootcas, key, cert")
        if config and not tlsclient:
            raise RuntimeError("Must specify both config and tlsclient")
        self._socket = None
        self._sockfile = None
        self._remote_node = None
        self._socketaddress = socketaddress
        self._rootcas = rootcas
        self._key = key
        self._cert = cert
        self._insecureskipverify = insecureskipverify
        if config and tlsclient:
            self.readconfig(config, tlsclient)

    def readstr(self):
        return self._sockfile.readline().decode().strip()

    def writestr(self, str):
        self._sockfile.write(str.encode())
        self._sockfile.flush()

    def handshake(self):
        m = re.compile("Receptor Control, node (.+)").fullmatch(self.readstr())
        if not m:
            raise RuntimeError("Failed to connect to Receptor socket")
        self._remote_node = m[1]

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
                    self._rootcas = key.get("rootcas", self._rootcas)
                    self._key = key.get("key", self._key)
                    self._cert = key.get("cert", self._cert)
                    self._insecureskipverify = key.get(
                        "insecureskipverify", self._insecureskipverify
                    )
                    break

    def simple_command(self, command):
        self.connect()
        self.writestr(f"{command}\n")
        return self.read_and_parse_json()

    def connect(self):
        if self._socket is not None:
            return
        m = re.compile(
            "(tcp|tls):(//)?([a-zA-Z0-9-.:]+):([0-9]+)|(unix:(//)?)?([^:]+)"
        ).fullmatch(self._socketaddress)
        if m:
            unixsocket = m[7]
            host = m[3]
            port = m[4]
            protocol = m[1]
            if unixsocket:
                path = os.path.expanduser(unixsocket)
                if not os.path.exists(path):
                    raise ValueError(f"Socket path does not exist: {path}")
                self._socket = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
                self._socket.connect(path)
                self._sockfile = self._socket.makefile("rwb")
                self.handshake()
                return
            elif host and port:
                self._socket = None
                addrs = socket.getaddrinfo(
                    host,
                    port,
                    socket.AF_UNSPEC,
                    socket.SOCK_STREAM,
                    0,
                    socket.AI_PASSIVE,
                )
                for addr in addrs:
                    family, type, proto, canonname, sockaddr = addr
                    try:
                        self._socket = socket.socket(family, type, proto)
                    except OSError:
                        self._socket = None
                        continue
                    try:
                        if protocol == "tls":
                            context = ssl.create_default_context(
                                purpose=ssl.Purpose.SERVER_AUTH, cafile=self._rootcas
                            )
                            if self._key and self._cert:
                                context.load_cert_chain(
                                    certfile=self._cert, keyfile=self._key
                                )
                            if self._insecureskipverify:
                                context.check_hostname = False
                            self._socket = context.wrap_socket(
                                self._socket, server_hostname=host
                            )
                        self._socket.connect(sockaddr)
                    except OSError:
                        self._socket.close()
                        self._socket = None
                        continue
                    self._sockfile = self._socket.makefile("rwb")
                    break
                if self._socket is None:
                    raise ValueError(f"Could not connect to host {host} port {port}")
                self.handshake()
                return
        raise ValueError(f"Invalid socket address {self._socketaddress}")

    def close(self):
        if self._sockfile is not None:
            try:
                self._sockfile.close()
            finally:
                self._sockfile = None

        if self._socket is not None:
            try:
                self._socket.close()
            finally:
                self._socket = None

    def connect_to_service(self, node, service, tlsclient):
        self.connect()
        self.writestr(f"connect {node} {service} {tlsclient}\n")
        text = self.readstr()
        if not str.startswith(text, "Connecting"):
            raise RuntimeError(text)

    def submit_work(
        self,
        worktype,
        payload,
        node=None,
        tlsclient=None,
        ttl=None,
        signwork=False,
        params=None,
    ):
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
            commandMap["tlsclient"] = tlsclient

        if ttl:
            commandMap["ttl"] = ttl

        if signwork:
            commandMap["signwork"] = "true"

        if params:
            for k, v in params.items():
                if k not in commandMap:
                    if v[0] == "@" and v[:2] != "@@":
                        fname = v[1:]
                        if not os.path.exists(fname):
                            raise FileNotFoundError("{} does not exist".format(fname))
                        try:
                            with open(fname, "r") as f:
                                v_contents = f.read()
                        except Exception:
                            raise OSError("could not read from file {}".format(fname))
                        commandMap[k] = v_contents
                    else:
                        commandMap[k] = v
                else:
                    raise RuntimeError(f"Duplicate or illegal parameter {k}")
        commandJson = json.dumps(commandMap)
        command = f"{commandJson}\n"
        self.writestr(command)
        text = self.readstr()
        m = re.compile(
            "Work unit created with ID (.+). Send stdin data and EOF."
        ).fullmatch(text)
        if not m:
            errmsg = "Failed to start work unit"
            if str.startswith(text, "ERROR: "):
                errmsg = errmsg + ": " + text[7:]
            raise RuntimeError(errmsg)
        if isinstance(payload, io.IOBase):
            shutil.copyfileobj(payload, self._sockfile)
        elif isinstance(payload, str):
            self.writestr(payload)
        elif isinstance(payload, bytes):
            self._sockfile.write(payload)
        else:
            raise RuntimeError("Unknown payload type")
        self._sockfile.flush()
        shutdown_write(self._socket)
        text = self.readstr()
        self.close()
        if text.startswith("ERROR:"):
            raise RuntimeError(f"Remote error: {text}")
        result = json.loads(text)
        return result

    def get_work_results(self, unit_id, return_socket=False, return_sockfile=True):
        self.connect()
        self.writestr(f"work results {unit_id}\n")
        text = self.readstr()
        m = re.compile("Streaming results for work unit (.+)").fullmatch(text)
        if not m:
            errmsg = "Failed to get results"
            if str.startswith(text, "ERROR: "):
                errmsg = errmsg + ": " + text[7:]
            raise RuntimeError(errmsg)
        shutdown_write(self._socket)

        # We return the filelike object created by makefile() by default, or optionally
        # the socket itself.  Either way, we close the other dup'd handle so the caller's
        # close will be effective.

        socket = self._socket
        sockfile = self._sockfile
        try:
            if not return_socket:
                self._socket.close()
            if not return_sockfile:
                self.sockfile.close()
        finally:
            self._socket = None
            self._sockfile = None

        if return_socket and return_sockfile:
            return socket, sockfile
        elif return_socket:
            return socket
        elif return_sockfile:
            return sockfile
        else:
            return
