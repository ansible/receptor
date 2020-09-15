FROM fedora:32
ARG VERSION

RUN dnf -y update && \
    dnf -y install dumb-init python3-click python3-dateutil && \
    dnf clean all

COPY receptorctl-${VERSION}-py3-none-any.whl /tmp
COPY receptor_python_worker-${VERSION}-py3-none-any.whl /tmp
RUN pip3 install /tmp/*.whl
RUN rm /tmp/*.whl

COPY receptor.conf /etc/receptor/receptor.conf
COPY receptor /usr/bin/receptor

ENV RECEPTORCTL_SOCKET=/tmp/receptor.sock

EXPOSE 7323

ENTRYPOINT ["/usr/bin/dumb-init", "--"]
CMD ["/usr/bin/receptor", "-c", "/etc/receptor/receptor.conf"]

