#!/bin/bash
SOCKCEPTOR=$HOME/Desktop/sockceptor/sockceptor
UDPTUNNEL=$HOME/udptunnel/src/udptunnel
SSH_HOST=10.1.1.1
LOCALHOST=127.0.0.1

tmux new-window $SOCKCEPTOR --debug --node-id foo --listen $LOCALHOST:2000 --peer $LOCALHOST:2001 --udp in:udptunnel_in:$LOCALHOST:3000:baz:udptunnel_out
tmux new-window $SOCKCEPTOR --debug --node-id bar --listen $LOCALHOST:2001 --peer $LOCALHOST:2002
tmux new-window $SOCKCEPTOR --debug --node-id baz --listen $LOCALHOST:2002 --peer $LOCALHOST:2003 --udp out:udptunnel_out:$LOCALHOST:3001:foo:udptunnel_in
tmux new-window $SOCKCEPTOR --debug --node-id long1 --listen $LOCALHOST:2003 --peer $LOCALHOST:2004
tmux new-window $SOCKCEPTOR --debug --node-id long2 --listen $LOCALHOST:2004 --peer $LOCALHOST:2000

tmux new-window $UDPTUNNEL -s 3001 -a $LOCALHOST,$SSH_HOST,22,allow
tmux new-window $UDPTUNNEL -c 2222 -t $LOCALHOST:3000 -r $SSH_HOST:22

