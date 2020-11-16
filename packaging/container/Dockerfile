FROM fedora:32 as builder

RUN dnf -y update && dnf install -y golang make

ADD source.tar.gz /source
WORKDIR /source
RUN make

FROM fedora:32
ARG VERSION

LABEL license="ASL2"
LABEL name="receptor"
LABEL vendor="project-receptor"
LABEL version="${VERSION}"

RUN dnf -y update && \
    dnf -y install dumb-init python3-click python3-pyyaml python3-dateutil python3-pip && \
    dnf clean all

COPY receptorctl-${VERSION}-py3-none-any.whl /tmp
COPY receptor_python_worker-${VERSION}-py3-none-any.whl /tmp
RUN pip3 install /tmp/*.whl
RUN rm /tmp/*.whl

COPY receptor.conf /etc/receptor/receptor.conf
COPY --from=builder /source/receptor /usr/bin/receptor

ENV RECEPTORCTL_SOCKET=/tmp/receptor.sock

EXPOSE 7323

ENTRYPOINT ["/usr/bin/dumb-init", "--"]
CMD ["/usr/bin/receptor", "-c", "/etc/receptor/receptor.conf"]
