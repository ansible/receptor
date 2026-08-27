FROM alpine

WORKDIR /opt

RUN set -x \
    && apk add --no-cache php py3-pip py3-build git bash \
    && git clone --depth 1 https://github.com/ansible/receptor.git \
    && cd receptor \
    && git fetch --depth 1 origin 4faa2fbd8e4a3c6b4aafd11bd281b9117224d359 \
    && git checkout 4faa2fbd8e4a3c6b4aafd11bd281b9117224d359 \
    && cd .. \
    && python3 -m build --no-isolation --wheel ./receptor/receptorctl \
    && pip3 install ./receptor/receptorctl/dist/*.whl --only-binary :all:

WORKDIR /app

CMD ["php", "-S", "0.0.0.0:8080"]
