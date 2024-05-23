FROM --platform=$TARGETPLATFORM alpine:3.20.0

LABEL org.opencontainers.image.source=https://github.com/Argelbargel/vault-raft-snapshot-agent
LABEL org.opencontainers.image.description="vault-raft-snapshot-agent ($TARGETPLATFORM)"
LABEL org.opencontainers.image.licenses=MIT

RUN apk --no-cache add ca-certificates \
    && rm -rf /var/cache/apk/*

VOLUME /etc/vault.d/ /tmp/certs

ARG DIST_DIR
ARG TARGETOS
ARG TARGETARCH
COPY ${DIST_DIR}/entrypoint /sbin/entrypoint
COPY ${DIST_DIR}/vault-raft-snapshot-agent_${TARGETOS}_${TARGETARCH} /bin/vault-raft-snapshot-agent
RUN chmod +x /sbin/entrypoint /bin/vault-raft-snapshot-agent

WORKDIR /
ENTRYPOINT ["/sbin/entrypoint"]
