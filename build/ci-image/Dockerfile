FROM centos:8
RUN dnf -y update && \
  dnf -y install epel-release && \
  dnf -y install git golang python3 python3-pip python3-pip-wheel make && \
  dnf clean all
RUN pip3 install pre-commit
RUN go get -u golang.org/x/lint/golint

# Warm up the Go build cache
RUN git clone https://github.com/project-receptor/receptor && cd receptor && make build-all && cd .. && rm -rf receptor

COPY entrypoint.sh /entrypoint.sh
ENTRYPOINT ["/entrypoint.sh"]
