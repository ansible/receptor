FROM fedora:32

RUN dnf -y update && \
    dnf -y install 'dnf-command(copr)' && \
    dnf -y copr enable ghjm/Receptor && \
    dnf -y install make python3 python3-dateutil rpm-build spectool \
    iproute openssl go-rpm-macros golang jq git createrepo \
    'golang(github.com/creack/pty)' \
    'golang(github.com/fortytw2/leaktest)' \
    'golang(github.com/fsnotify/fsnotify)' \
    'golang(github.com/google/shlex)' \
    'golang(github.com/gorilla/websocket)' \
    'golang(github.com/jupp0r/go-priority-queue)' \
    'golang(github.com/lucas-clemente/quic-go)' \
    'golang(github.com/minio/highwayhash)' \
    'golang(github.com/rogpeppe/go-internal/lockedfile)' \
    'golang(github.com/songgao/water)' \
    'golang(github.com/vishvananda/netlink)' \
    'golang(gopkg.in/yaml.v2)' \
    golang-k8s-api-devel \
    golang-k8s-apimachinery-devel \
    golang-k8s-client-devel && \
    dnf clean all

COPY rpmmacros /root/.rpmmacros
COPY buildrpms.sh /root/buildrpms.sh
WORKDIR /receptor
ENTRYPOINT ["/root/buildrpms.sh"]

