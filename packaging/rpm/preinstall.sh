getent group receptor >/dev/null || groupadd -r receptor
getent passwd receptor >/dev/null || \
    useradd -r -g receptor -d /var/lib/receptor -s /bin/bash receptor
