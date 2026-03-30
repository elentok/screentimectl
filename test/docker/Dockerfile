FROM golang:1.26-bookworm AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /screentimectl .

FROM ubuntu:24.04

RUN apt-get update && \
    apt-get install -y --no-install-recommends sudo systemd ca-certificates && \
    rm -rf /var/lib/apt/lists/*

# Fake binaries for testing
COPY test/docker/fake-loginctl /usr/bin/loginctl
RUN chmod +x /usr/bin/loginctl

# Fake notify-send and espeak-ng (no-ops)
RUN echo '#!/bin/bash' > /usr/bin/notify-send && chmod +x /usr/bin/notify-send
RUN echo '#!/bin/bash' > /usr/bin/espeak-ng && chmod +x /usr/bin/espeak-ng

# Create a fake PAM file for testing
RUN mkdir -p /etc/pam.d && echo 'auth required pam_unix.so' > /etc/pam.d/gdm-password

COPY --from=build /screentimectl /usr/local/bin/screentimectl

COPY test/docker/run-tests.sh /run-tests.sh
RUN chmod +x /run-tests.sh

CMD ["/run-tests.sh"]
