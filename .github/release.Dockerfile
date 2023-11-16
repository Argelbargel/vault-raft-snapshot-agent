FROM --platform=$TARGETPLATFORM alpine:3.18.4

LABEL org.opencontainers.image.source=https://github.com/Argelbargel/vault-raft-snapshot-agent
LABEL org.opencontainers.image.description="vault-raft-snapshot-agent ($TARGETPLATFORM)"
LABEL org.opencontainers.image.licenses=MIT

ENTRYPOINT ["/bin/vault-raft-snapshot-agent"]
VOLUME /etc/vault.d/
WORKDIR /

ARG DIST_DIR
ARG TARGETOS
ARG TARGETARCH
COPY ${DIST_DIR}/vault-raft-snapshot-agent_${TARGETOS}_${TARGETARCH} /bin/vault-raft-snapshot-agent
RUN chmod +x /bin/vault-raft-snapshot-agent
