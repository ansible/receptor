FROM quay.io/centos/centos:stream8 as builder

RUN dnf -y update && dnf install -y make python3 python3-pip python3-wheel

RUN curl -LO https://go.dev/dl/go1.16.13.linux-amd64.tar.gz

RUN tar -C /usr/local -xzf go1.16.13.linux-amd64.tar.gz

ADD source.tar.gz /source
WORKDIR /source
ENV PATH=/usr/local/go/bin:$PATH 
RUN make

FROM quay.io/centos/centos:stream8
ARG VERSION

LABEL license="ASL2"
LABEL name="receptor"
LABEL vendor="ansible"
LABEL version="${VERSION}"

RUN dnf -y update && \
    dnf -y install epel-release && \
    dnf -y install tini python3-click python3-pyyaml python3-dateutil python3-pip python3-wheel && \
    dnf clean all

COPY receptorctl-${VERSION}-py3-none-any.whl /tmp
COPY receptor_python_worker-${VERSION}-py3-none-any.whl /tmp
RUN pip3 install /tmp/*.whl
RUN rm /tmp/*.whl

COPY receptor.conf /etc/receptor/receptor.conf
COPY --from=builder /source/receptor /usr/bin/receptor

ENV RECEPTORCTL_SOCKET=/tmp/receptor.sock

EXPOSE 7323

ENTRYPOINT ["/usr/bin/tini", "--"]
CMD ["/usr/bin/receptor", "-c", "/etc/receptor/receptor.conf"]
