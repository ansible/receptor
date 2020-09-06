FROM fedora:32

RUN dnf -y update

COPY RPMS/ /receptor-RPMS/
COPY receptor.repo /etc/yum.repos.d/receptor.repo
COPY receptor.conf /etc/receptor/receptor.conf

RUN dnf -y install dumb-init receptor receptorctl && \
    dnf clean all

RUN rm -rf /receptor-RPMs/ /etc/yum.repos.d/receptor.repo

ENV RECEPTORCTL_SOCKET=/tmp/receptor.sock

EXPOSE 7323

ENTRYPOINT ["/usr/bin/dumb-init", "--"]
CMD ["/usr/bin/receptor", "-c", "/etc/receptor/receptor.conf"]

