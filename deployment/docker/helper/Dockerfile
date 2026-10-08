FROM debian:13-slim

ARG ANSIBLE_VERSION=9.5.1
ARG DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y --no-install-recommends \
        openssh-client \
        util-linux \
        git \
        ca-certificates \
        python3 \
        python3-pip \
        python3-venv \
        sshpass \
        tini \
        curl \
    && rm -rf /var/lib/apt/lists/*

RUN usermod -d /workspace nobody \
 && install -d -o nobody -g 0 -m 0775 /workspace \
 && chmod g=u /etc/passwd

ENV HOME=/workspace
WORKDIR /workspace
USER nobody:0

ENTRYPOINT ["/usr/bin/tini", "--"]
CMD ["sh"]
LABEL org.opencontainers.image.source="https://github.com/impishMD/jeh" \
      org.opencontainers.image.title="Job Executor Hub — helper" \
      org.opencontainers.image.licenses="MIT"
COPY LICENSE NOTICE THIRD-PARTY-LICENSES.md /usr/share/licenses/jeh/
