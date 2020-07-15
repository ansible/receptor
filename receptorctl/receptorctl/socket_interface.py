import sys
import os
import re
import socket
import json
import time
import dateutil.parser
from pprint import pprint


class ReceptorControl:
    def __init__(self):
        self.socket = None
        self.sockfile = None
        self.remote_node = None

    def readstr(self):
        return self.sockfile.readline().strip()

    def writestr(self, str):
        self.sockfile.write(str)
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
                self.sockfile = self.socket.makefile('rw')
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
            self.sockfile.close()
            self.sockfile = None

        if self.socket is not None:
            self.socket.close()
            self.socket = None

    def print_status(self):
        status = self.simple_command("status")
        node_id = status.pop('NodeID')
        print(f"Node ID: {node_id}")

        longest_node = 12

        connections = status.pop('Connections', None)
        if connections:
            for conn in connections:
                l = len(conn['NodeID'])
                if l > longest_node:
                    longest_node = l

        costs = status.pop('KnownConnectionCosts', None)
        if costs:
            for node in costs:
                if len(node) > longest_node:
                    longest_node = len(node)

        if connections:
            for conn in connections:
                l = len(conn['NodeID'])
                if l > longest_node:
                    longest_node = l
            print()
            print(f"{'Connection':<{longest_node}} Cost")
            for conn in connections:
                print(f"{conn['NodeID']:<{longest_node}} {conn['Cost']}")

        if costs:
            print()
            print(f"{'Known Node':<{longest_node}} Known Connections")
            for node in costs:
                print(f"{node:<{longest_node}} ", end="")
                pprint(costs[node])

        routes = status.pop('RoutingTable', None)
        if routes:
            print()
            print(f"{'Route':<{longest_node}} Via")
            for node in routes:
                print(f"{node:<{longest_node}} {routes[node]}")

        ads = status.pop('Advertisements', None)
        if ads:
            print()
            print(f"{'Node':<{longest_node}} Service   Last Seen            Tags")
            for ad in ads:
                time = dateutil.parser.parse(ad['Time'])
                print(f"{ad['NodeID']:<{longest_node}} {ad['Service']:<8}  {time:%Y-%m-%d %H:%M:%S}  ", end="")
                pprint(ad['Tags'])

        if status:
            print("Additional data returned from Receptor:")
            pprint(status)

    def ping(self, node):
        try:
            result = self.simple_command(f"ping {node}")
            success = str.startswith(result['Result'], "Reply")
            return success, result['Result']
        except RuntimeError as e:
            return False, e

    def connect_to_service(self, node, service):
        self.writestr(f"connect {node} {service}\n")
        text = self.readstr()
        if not str.startswith(text, "Connecting"):
            raise RuntimeError(text)