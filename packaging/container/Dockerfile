FROM quay.io/centos/centos:stream9 AS builder
ARG VERSION

RUN sed -i 's@^metalink=.*appstream.*@baseurl=https://mirror.stream.centos.org/9-stream/AppStream/aarch64/os@g' /etc/yum.repos.d/*.repo
RUN dnf -y update && \
    dnf install -y golang make python3.12 python3.12-pip git
RUN pip3.12 install wheel

ADD source.tar.gz /source
WORKDIR /source
RUN make VERSION=${VERSION}

FROM quay.io/centos/centos:stream9
ARG VERSION

LABEL license="ASL2"
LABEL name="receptor"
LABEL vendor="ansible"
LABEL version="${VERSION}"

COPY receptorctl-${VERSION}-py3-none-any.whl /tmp
COPY receptor_python_worker-${VERSION}-py3-none-any.whl /tmp
COPY receptor.conf /etc/receptor/receptor.conf

RUN dnf -y update && \
    dnf -y install python3.12-pip && \
    dnf clean all && \
    pip3.12 install --no-cache-dir wheel && \
    pip3.12 install --no-cache-dir dumb-init && \
    pip3.12 install --no-cache-dir /tmp/*.whl && \
    rm /tmp/*.whl

COPY --from=builder /source/receptor /usr/bin/receptor

ENV RECEPTORCTL_SOCKET=/tmp/receptor.sock

EXPOSE 7323

ENTRYPOINT ["/usr/local/bin/dumb-init", "--"]
CMD ["/usr/bin/receptor", "-c", "/etc/receptor/receptor.conf"]
