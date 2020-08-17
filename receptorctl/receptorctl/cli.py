import sys
import os
import time
import select
import fcntl
import tty
import termios
import click
from pprint import pprint
from functools import partial
import dateutil.parser
from .socket_interface import ReceptorControl


class IgnoreRequiredWithHelp(click.Group):
    def parse_args(self, ctx, args):
        try:
            return super(IgnoreRequiredWithHelp, self).parse_args(ctx, args)
        except click.MissingParameter as exc:
            if '--help' not in args:
                raise

            # remove the required params so that help can display
            for param in self.params:
                param.required = False
            return super(IgnoreRequiredWithHelp, self).parse_args(ctx, args)

            
@click.group(cls=IgnoreRequiredWithHelp)
@click.pass_context
@click.option('--socket', envvar='RECEPTORCTL_SOCKET', required=True, show_envvar=True,
              help="Control socket address to connect to Receptor (defaults to Unix socket, use tcp:// for TCP socket)")
def cli(ctx, socket):
    ctx.obj = dict()
    ctx.obj['socket'] = socket


def get_rc(ctx):
    rc = ReceptorControl()
    rc.connect(ctx.obj['socket'])
    return rc


@cli.command(help="Show the status of the Receptor network.")
@click.pass_context
def status(ctx):
    rc = get_rc(ctx)
    status = rc.simple_command("status")

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


@cli.command(help="Ping a Receptor node.")
@click.pass_context
@click.argument('node')
@click.option('--count', default=4, help="Number of pings to send", show_default=True)
@click.option('--delay', default=1.0, help="Time to wait between pings", show_default=True)
def ping(ctx, node, count, delay):
    rc = get_rc(ctx)
    for i in range(count):
        success, detail = rc.ping(node)
        if success:
            print(detail)
        else:
            print("FAILED:", detail)
        if i < count-1:
            time.sleep(delay)


@cli.command(help="Connect the local terminal to a Receptor service on a remote node.")
@click.pass_context
@click.argument('node')
@click.argument('service')
@click.option('--raw', '-r', default=False, is_flag=True, help="Set terminal to raw mode")
def connect(ctx, node, service, raw):
    rc = get_rc(ctx)
    rc.connect_to_service(node, service)

    stdin_tattrs = termios.tcgetattr(sys.stdin)
    stdin_fcntl = fcntl.fcntl(sys.stdin, fcntl.F_GETFL)
    fcntl.fcntl(sys.stdin, fcntl.F_SETFL, stdin_fcntl | os.O_NONBLOCK)
    if raw and sys.stdin.isatty():
        tty.setraw(sys.stdin.fileno(), termios.TCSAFLUSH)
        new_term = termios.tcgetattr(sys.stdin)
        new_term[3] = new_term[3] & ~termios.ISIG
        termios.tcsetattr(sys.stdin, termios.TCSAFLUSH, new_term)

    try:
        while True:
            r, _, _ = select.select([rc.socket, sys.stdin], [], [])
            for readable in r:
                if readable is rc.socket:
                    data = rc.socket.recv(4096)
                    if not data:
                        return
                    sys.stdout.write(data.decode())
                    sys.stdout.flush()
                else:
                    data = sys.stdin.read()
                    if not data:
                        return
                    rc.socket.send(data.encode())
    finally:
        termios.tcsetattr(sys.stdin, termios.TCSAFLUSH, stdin_tattrs)


@cli.group(help="Commands related to unit-of-work processing")
def work():
    pass


@work.command(help="List known units of work.")
@click.option('--quiet', '-q', is_flag=True, help="Only list unit IDs with no detail")
@click.pass_context
def list(ctx, quiet):
    rc = get_rc(ctx)
    work = rc.simple_command("work list")
    if quiet:
        for k in work.keys():
            print(k)
    else:
        pprint(work)


@work.command(help="Submit a new unit of work.")
@click.pass_context
@click.argument('worktype', type=str, required=True)
@click.option('--node', type=str, help="Receptor node to run the work on. Defaults to the local node.")
@click.option('--payload', '-p', type=str, help="File containing unit of work data. Use - for stdin.")
@click.option('--payload-literal', '-l', type=str, help="Use the command line string as the literal unit of work data.")
@click.option('--follow', '-f', help="Remain attached to the job and print its results to stdout", is_flag=True)
@click.option('--rm', help="Release unit after completion", is_flag=True)
@click.argument('params', nargs=-1, type=click.UNPROCESSED)
def submit(ctx, worktype, node, payload, payload_literal, follow, rm, params):
    if not payload and not payload_literal:
        print("Must provide one of --payload or --payload-literal.")
        sys.exit(1)
    if payload and payload_literal:
        print("Cannot provide both --payload and --payload-literal.")
        sys.exit(1)
    if rm and not follow:
        print("Warning: using --rm without --follow. Unit results will never be seen.")
    if payload_literal:
        payload_data = f"{payload_literal}\n".encode()
    else:
        if payload == "-":
            payload_data = sys.stdin.buffer
        else:
            payload_data = open(payload, 'rb')
    unitid = None
    try:
        rc = get_rc(ctx)
        if node == "":
            node = None
        work = rc.submit_work(node, worktype, " ".join(params), payload_data)
        result = work.pop('result')
        unitid = work.pop('unitid')
        if follow:
            ctx.invoke(results, unit_id=unitid)
        else:
            print("Result: ", result)
            print("Unit ID:", unitid)
    finally:
        if rm and unitid:
            op_on_unit_ids(ctx, "release", [unitid])


@work.command(help="Get results for a previously run unit of work.")
@click.pass_context
@click.argument('unit_id', type=str, required=True)
def results(ctx, unit_id):
    rc = get_rc(ctx)
    resultsfile = rc.get_work_results(unit_id)
    try:
        for text in iter(partial(resultsfile.readline, 256), b''):
            sys.stdout.buffer.write(text)
            sys.stdout.buffer.flush()
    except Exception as e:
        print("Exception:", e)


def op_on_unit_ids(ctx, op, unit_ids):
    rc = get_rc(ctx)
    for unit_id in unit_ids:
        try:
            rc.simple_command(f"work {op} {unit_id}")
        except Exception as e:
            print(f"{unit_id}: ERROR: {e}")


@work.command(help="Cancel (kill) one or more units of work.")
@click.argument('unit_ids', nargs=-1)
@click.pass_context
def cancel(ctx, unit_ids):
    if len(unit_ids) == 0:
        print("No unit IDs supplied: Not doing anything")
        return
    op_on_unit_ids(ctx, "cancel", unit_ids)
    print("Cancelled:", unit_ids)


@work.command(help="Release (delete) one or more units of work.")
@click.argument('unit_ids', nargs=-1)
@click.pass_context
def release(ctx, unit_ids):
    if len(unit_ids) == 0:
        print("No unit IDs supplied: Not doing anything")
        return
    op_on_unit_ids(ctx, "release", unit_ids)
    print("Released:", unit_ids)


def run():
    try:
        cli.main(sys.argv[1:], standalone_mode=False)
    except click.exceptions.Abort:
        pass
    except Exception as e:
        print("Error:", e)
        sys.exit(1)
    sys.exit(0)
