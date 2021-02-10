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
import pkg_resources
from .socket_interface import ReceptorControl


class IgnoreRequiredWithHelp(click.Group):
    # allows user to call --help without needing to provide required=true parameters
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
@click.option('--config', '-c', default=None, envvar='RECEPTORCTL_CONFIG', required=False, show_envvar=True,
              help="Config filename configured for receptor")
@click.option('--tls-client', 'tlsclient', default=None, envvar='RECEPTORCTL_TLSCLIENT', required=False, show_envvar=True,
              help="TLS client name specified in config")
@click.option('--rootcas', default=None, help="Root CA bundle to use instead of system trust when connecting with tls")
@click.option('--key', default=None, help="Client private key filename")
@click.option('--cert', default=None, help="Client certificate filename")
@click.option('--insecureskipverify', default=False, help="Accept any server cert", show_default=True)
def cli(ctx, socket, config, tlsclient, rootcas, key, cert, insecureskipverify):
    ctx.obj = dict()
    ctx.obj['rc'] = ReceptorControl(socket, config=config, tlsclient=tlsclient, rootcas=rootcas, key=key, cert=cert, insecureskipverify=insecureskipverify)
def get_rc(ctx):
    return ctx.obj['rc']


@cli.command(help="Show the status of the Receptor network.")
@click.pass_context
def status(ctx):
    rc = get_rc(ctx)
    status = rc.simple_command("status")

    node_id = status.pop('NodeID')
    print(f"Node ID: {node_id}")
    version = status.pop('Version')
    print(f"Version: {version}")
    sysCPU = status.pop('SystemCPUCount')
    print(f"System CPU Count: {sysCPU}")
    sysMemory = status.pop('SystemMemoryMiB')
    print(f"System Memory MiB: {sysMemory}")

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
        print(f"{'Node':<{longest_node}} Service   Type       Last Seen           Tags            Work Types")
        for ad in ads:
            time = dateutil.parser.parse(ad['Time'])
            if ad['ConnType'] == 0:
                conn_type = 'Datagram'
            elif ad['ConnType'] == 1:
                conn_type = 'Stream'
            elif ad['ConnType'] == 2:
                conn_type = 'StreamTLS'
            print(
                f"{ad['NodeID']:<{longest_node}} {ad['Service']:<9} {conn_type:<10} {time:%Y-%m-%d %H:%M:%S} {(ad['Tags'] or '-'):<16}",
                end=""
            )
            commands = ad['WorkCommands']
            if commands:
                commands = ', '.join(commands)
            else:
                commands = '-'
            print(commands)

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
        results = rc.simple_command(f"ping {node}")
        if "Success" in results and results["Success"]:
            print(f"Reply from {results['From']} in {results['TimeStr']}")
        else:
            if "From" in results and "TimeStr" in results:
                print(f"Error {results['Error']} from {results['From']} in {results['TimeStr']}")
            else:
                print(f"Error: {results['Error']}")
        if i < count-1:
            time.sleep(delay)


@cli.command(help="Do a traceroute to a Receptor node.")
@click.pass_context
@click.argument('node')
def traceroute(ctx, node):
    rc = get_rc(ctx)
    results = rc.simple_command(f"traceroute {node}")
    for resno in sorted(results, key=lambda r: int(r)):
        resval = results[resno]
        if 'Error' in resval:
            print(f"{resno}: Error {resval['Error']} from {resval['From']} in {resval['TimeStr']}")
        else:
            print(f"{resno}: {resval['From']} in {resval['TimeStr']}")


@cli.command(help="Connect the local terminal to a Receptor service on a remote node.")
@click.pass_context
@click.argument('node')
@click.argument('service')
@click.option('--raw', '-r', default=False, is_flag=True, help="Set terminal to raw mode")
@click.option('--tls-client', 'tlsclient', type=str, default="", help="TLS client config name used when connecting to remote node")
def connect(ctx, node, service, raw, tlsclient):
    rc = get_rc(ctx)
    rc.connect_to_service(node, service, tlsclient)

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
            r, _, _ = select.select([rc._socket, sys.stdin], [], [])
            for readable in r:
                if readable is rc._socket:
                    data = rc._socket.recv(4096)
                    if not data:
                        return
                    sys.stdout.write(data.decode())
                    sys.stdout.flush()
                else:
                    data = sys.stdin.read()
                    if not data:
                        return
                    rc._socket.send(data.encode())
    finally:
        termios.tcsetattr(sys.stdin, termios.TCSAFLUSH, stdin_tattrs)
        print()


@cli.group(help="Commands related to unit-of-work processing")
def work():
    pass

@cli.command(help="Show version information for receptorctl and the receptor node")
@click.pass_context
def version(ctx):
    rc = get_rc(ctx)
    receptorVersion = rc.simple_command('{"command":"status","requested_fields":["Version"]}')["Version"]
    receptorctlVersion = pkg_resources.get_distribution('receptorctl').version
    delim = ""
    if receptorVersion != receptorctlVersion:
        delim = "\t"
        print("Warning: receptorctl and receptor are different versions, they may not be compatible")
    print(f"{delim}receptorctl  {receptorctlVersion}")
    print(f"{delim}receptor     {receptorVersion}")


@work.command(help="List known units of work.")
@click.option('--quiet', '-q', is_flag=True, help="Only list unit IDs with no detail")
@click.option('--node', default=None, type=str, help="Receptor node to list work from. Defaults to the local node.")
@click.option('--tls-client', 'tlsclient', type=str, default="", help="TLS client config name used when connecting to remote node")
@click.pass_context
def list(ctx, node, tlsclient, quiet):
    rc = get_rc(ctx)
    if node:
        rc.connect_to_service(node, "control", tlsclient)
        rc.handshake()
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
@click.option('--no-payload', '-n', is_flag=True, help="Send an empty payload.")
@click.option('--tls-client', 'tlsclient', type=str, default="", help="TLS client used when submitting work to a remote node")
@click.option('--ttl', type=str, default="", help="Time to live until remote work must start, e.g. 1h20m30s or 30m10s")
@click.option('--follow', '-f', help="Remain attached to the job and print its results to stdout", is_flag=True)
@click.option('--rm', help="Release unit after completion", is_flag=True)
@click.option('--param', '-a', help="Additional Receptor parameter (key=value format)", multiple=True)
@click.argument('cmdparams', type=str, required=False, nargs=-1)
def submit(ctx, worktype, node, payload, no_payload, payload_literal, tlsclient, ttl, follow, rm, param, cmdparams):
    pcmds = 0
    if payload:
        pcmds += 1
    if no_payload:
        pcmds += 1
    if payload_literal:
        pcmds += 1
    if pcmds < 1:
        print("Must provide one of --payload, --no-payload or --payload-literal.")
        sys.exit(1)
    if pcmds > 1:
        print("Cannot provide more than one of --payload, --no-payload and --payload-literal.")
        sys.exit(1)
    if rm and not follow:
        print("Warning: using --rm without --follow. Unit results will never be seen.")
    if payload_literal:
        payload_data = f"{payload_literal}\n".encode()
    elif no_payload:
        payload_data = "".encode()
    else:
        if payload == "-":
            payload_data = sys.stdin.buffer
        else:
            payload_data = open(payload, 'rb')
    unitid = None
    try:
        params = dict(s.split('=', 1) for s in param)
        if cmdparams:
            allparams = []
            if "params" in params:
                allparams.append(params["params"])
            allparams.extend(cmdparams)
            params["params"] = " ".join(allparams)
        if node == "":
            node = None
        rc = get_rc(ctx)
        work = rc.submit_work(worktype, payload_data, node=node, tlsclient=tlsclient, ttl=ttl, params=params)
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


@work.command(help="Get results for a previously or currently running unit of work.")
@click.pass_context
@click.argument('unit_id', type=str, required=True)
def results(ctx, unit_id):
    rc = get_rc(ctx)
    resultsfile = rc.get_work_results(unit_id)
    for text in iter(partial(resultsfile.readline, 256), b''):
        sys.stdout.buffer.write(text)
        sys.stdout.buffer.flush()
    rc = get_rc(ctx)
    status = rc.simple_command(f"work status {unit_id}")
    state = status.pop("State", 0)
    if state == 3:    # Failed
        detail = status.pop("Detail", "Unknown")
        sys.stderr.write(f"Remote unit failed: {detail}\n")
        sys.exit(1)


def op_on_unit_ids(ctx, op, unit_ids):
    rc = get_rc(ctx)
    for unit_id in unit_ids:
        try:
            rc.simple_command(f"work {op} {unit_id}")
        except Exception as e:
            print(f"{unit_id}: ERROR: {e}")
            sys.exit(1)


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
@click.option('--force', help="Delete locally even if we can't reach the remote node", is_flag=True)
@click.argument('unit_ids', nargs=-1)
@click.pass_context
def release(ctx, force, unit_ids):
    if len(unit_ids) == 0:
        print("No unit IDs supplied: Not doing anything")
        return
    op = "release" if not force else "force-release"
    op_on_unit_ids(ctx, op, unit_ids)
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
