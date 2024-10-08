ARG BIN_NAME=midea2influx
ARG BIN_VERSION=<unknown>

FROM golang:1-alpine AS builder
ARG BIN_NAME
ARG BIN_VERSION

RUN update-ca-certificates

WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-X main.version=${BIN_VERSION}" -o ./out/${BIN_NAME} .

FROM python:3-slim
ARG BIN_NAME
COPY --from=builder /src/out/${BIN_NAME} /usr/bin/${BIN_NAME}
ENTRYPOINT ["/usr/bin/midea2influx"]
CMD ["-config", "/config.json"]

COPY requirements.txt /opt/requirements.txt
ENV PIP_ROOT_USER_ACTION=ignore
RUN pip install --upgrade --no-cache-dir pip && \
    pip install --no-cache-dir --no-dependencies -r /opt/requirements.txt && \
    rm /opt/requirements.txt

LABEL license="MIT"
LABEL org.opencontainers.image.licenses="MIT"
LABEL maintainer="Chris Dzombak <https://www.dzombak.com>"
LABEL org.opencontainers.image.authors="Chris Dzombak <https://www.dzombak.com>"
LABEL org.opencontainers.image.url="https://github.com/cdzombak/midea2influx"
LABEL org.opencontainers.image.documentation="https://github.com/cdzombak/midea2influx/blob/main/README.md"
LABEL org.opencontainers.image.source="https://github.com/cdzombak/midea2influx.git"
LABEL org.opencontainers.image.version="${BIN_VERSION}"
LABEL org.opencontainers.image.title="${BIN_NAME}"
LABEL org.opencontainers.image.description="Write status of Midea dehumidifers to InfluxDB"
