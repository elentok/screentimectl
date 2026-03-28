FROM golang:1.26-bookworm AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /screentimectl .

FROM ubuntu:24.04

RUN apt-get update && \
    apt-get install -y --no-install-recommends sudo systemd && \
    rm -rf /var/lib/apt/lists/*

# Fake timekpra binary that returns parseable output
COPY test/docker/fake-timekpra /usr/bin/timekpra
RUN chmod +x /usr/bin/timekpra

COPY --from=build /screentimectl /usr/local/bin/screentimectl

COPY test/docker/run-tests.sh /run-tests.sh
RUN chmod +x /run-tests.sh

CMD ["/run-tests.sh"]
