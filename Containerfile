FROM debian:bookworm-slim

ARG NTFY_VERSION=2.22.0
ARG TARGETARCH=amd64

RUN apt-get update && apt-get install -y --no-install-recommends \
        ca-certificates \
        wget \
    && rm -rf /var/lib/apt/lists/*

RUN wget -qO /tmp/ntfy.tar.gz \
        "https://github.com/binwiederhier/ntfy/releases/download/v${NTFY_VERSION}/ntfy_${NTFY_VERSION}_linux_${TARGETARCH}.tar.gz" \
    && tar -xzf /tmp/ntfy.tar.gz -C /tmp \
    && cp /tmp/ntfy_${NTFY_VERSION}_linux_${TARGETARCH}/ntfy /usr/local/bin/ntfy \
    && chmod +x /usr/local/bin/ntfy \
    && mkdir -p /etc/ntfy \
    && cp /tmp/ntfy_${NTFY_VERSION}_linux_${TARGETARCH}/client/client.yml /etc/ntfy/ \
    && cp /tmp/ntfy_${NTFY_VERSION}_linux_${TARGETARCH}/server/server.yml /etc/ntfy/ \
    && rm -rf /tmp/ntfy*

# Persist data and cache
VOLUME ["/var/cache/ntfy", "/var/lib/ntfy"]

# ntfy HTTP (and WebSocket) port
EXPOSE 77

ENTRYPOINT ["ntfy"]
CMD ["serve"]
