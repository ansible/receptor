FROM fedora:32
RUN dnf -y update && \
  dnf -y install git golang python3 python3-pip make && \
  dnf clean all
RUN pip3 install pre-commit

RUN useradd -u 1001 -ms /bin/bash test-user && mkdir /home/test-user/receptor \
    && chown test-user /home/test-user/receptor

USER test-user

RUN go get -u golang.org/x/lint/golint
# Warm up the Go build cache
RUN cd /home/test-user && git clone https://github.com/project-receptor/receptor receptor-tmp && cd receptor-tmp && go get -t ./... && make build-all && cd .. && rm -rf receptor-tmp

WORKDIR /home/test-user/receptor
COPY entrypoint.sh /home/test-user/entrypoint.sh
ENTRYPOINT ["/home/test-user/entrypoint.sh"]
