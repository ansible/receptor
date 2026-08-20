FROM alpine

WORKDIR /opt

RUN set -x \
    && apk add --no-cache php py3-pip py3-build git bash \
    && git clone --depth 1 https://github.com/ansible/receptor.git \
    && python3 -m build --wheel ./receptor/receptorctl \
    && pip3 install ./receptor/receptorctl/dist/*.whl --only-binary :all:

WORKDIR /app

CMD ["php", "-S", "0.0.0.0:8080"]
