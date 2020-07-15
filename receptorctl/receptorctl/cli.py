import sys
import os
import time
import select
import fcntl
import tty
import termios
import click
from .socket_interface import ReceptorControl


@click.group()
@click.pass_context
@click.option('--socket', envvar='RECEPTORCTL_SOCKET', required=True)
def cli(ctx, socket):
    rc = ReceptorControl()
    rc.connect(socket)
    ctx.obj = dict()
    ctx.obj['rc'] = rc


@cli.command()
@click.pass_context
def status(ctx):
    rc = ctx.obj['rc']
    rc.print_status()


@cli.command()
@click.pass_context
@click.argument('node')
@click.option('--count', default=4)
@click.option('--delay', default=1.0)
def ping(ctx, node, count, delay):
    rc = ctx.obj['rc']
    for i in range(count):
        success, detail = rc.ping(node)
        if success:
            print(detail)
        else:
            print("FAILED:", detail)
        if i < count-1:
            time.sleep(delay)


@cli.command()
@click.pass_context
@click.argument('node')
@click.argument('service')
@click.option('--raw', '-r', default=False, is_flag=True, help="Set terminal to raw mode")
def connect(ctx, node, service, raw):
    rc = ctx.obj['rc']
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


def run():
    try:
        cli.main(sys.argv[1:], standalone_mode=False)
    except click.exceptions.Abort:
        pass
    except Exception as e:
        print("Error:", e)
        sys.exit(1)
    sys.exit(0)
