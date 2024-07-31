FROM receptor:latest

RUN dnf install tc -y

ENTRYPOINT ["/bin/bash"]
CMD ["-c", "/usr/bin/receptor --legacy --config /etc/receptor/receptor.conf > /etc/receptor/stdout 2> /etc/receptor/stderr"]
