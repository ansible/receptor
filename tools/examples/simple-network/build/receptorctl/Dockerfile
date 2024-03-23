FROM alpine

WORKDIR /opt

RUN set -x \
    && apk add php py3-pip git bash \
    && pip3 install receptorctl

WORKDIR /app

CMD ["php", "-S", "0.0.0.0:8080"]
