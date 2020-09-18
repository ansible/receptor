import sys
import os
import re
import io
import socket
import shutil
import json


class ReceptorControl:
    def __init__(self):
        self.socket = None
        self.sockfile = None
        self.remote_node = None

    def readstr(self):
        return self.sockfile.readline().decode().strip()

    def writestr(self, str):
        self.sockfile.write(str.encode())
        self.sockfile.flush()

    def handshake(self):
        m = re.compile("Receptor Control, node (.+)").fullmatch(self.readstr())
        if not m:
            raise RuntimeError("Failed to handshake with Receptor socket")
        self.remote_node = m[1]

    def read_and_parse_json(self):
        text = self.readstr()
        if str.startswith(text, "ERROR:"):
            raise RuntimeError(text[7:])
        data = json.loads(text)
        return data

    def simple_command(self, command):
        self.writestr(f"{command}\n")
        return self.read_and_parse_json()

    def connect(self, address):
        if self.socket is not None:
            raise ValueError("Already connected")
        m = re.compile("tcp:(//)?([a-zA-Z0-9-]+):([0-9]+)|(unix:(//)?)?([^:]+)").fullmatch(address)
        if m:
            if m[6]:
                path = os.path.expanduser(m[6])
                if not os.path.exists(path):
                    raise ValueError(f"Socket path does not exist: {path}")
                self.socket = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
                self.socket.connect(path)
                self.sockfile = self.socket.makefile('rwb')
                self.handshake()
                return
            elif m[2] and m[3]:
                host = m[2]
                port = m[3]
                print(f"TCP socket, host {host} port {port}")
                self.handshake()
                return
        raise ValueError(f"Invalid socket address {address}")

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

    def connect_to_service(self, node, service):
        self.writestr(f"connect {node} {service}\n")
        text = self.readstr()
        if not str.startswith(text, "Connecting"):
            raise RuntimeError(text)

    def submit_work(self, node, worktype, params, payload, tlsclient):
        if node is None:
            node = "localhost"
        commandMap = {
            "command": "work",
            "subcommand": "submit",
            "node": node,
            "worktype": worktype,
            "tlsclient": tlsclient,
            "params": params,
        }
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
            self.writestr(str)
        elif isinstance(payload, bytes):
            self.sockfile.write(payload)
        else:
            raise RuntimeError("Unknown payload type")
        self.sockfile.flush()
        self.socket.shutdown(socket.SHUT_WR)
        text = self.readstr()
        self.close()
        result = json.loads(text)
        return result

    def get_work_results(self, unit_id):
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
