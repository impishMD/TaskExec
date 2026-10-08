# syntax=docker/dockerfile:1
FROM --platform=$BUILDPLATFORM node:24-alpine3.24 AS frontend
WORKDIR /src/web
COPY web/package*.json ./
RUN --mount=type=cache,target=/root/.npm npm ci --no-audit --no-fund
COPY web/ ./
COPY api-docs.yml /src/api-docs.yml
RUN npm run build

FROM --platform=$BUILDPLATFORM golang:1.26-alpine3.24 AS builder
WORKDIR /src
COPY go.mod go.sum ./
COPY pro/ ./pro/
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
COPY --from=frontend /src/api/public ./api/public
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -tags netgo \
    -ldflags "-s -w -X github.com/impishMD/jeh/util.Ver=$VERSION -X github.com/impishMD/jeh/util.Commit=$COMMIT -X github.com/impishMD/jeh/util.Date=$BUILD_DATE" \
    -o /out/jeh ./cli

FROM --platform=$BUILDPLATFORM alpine:3.24 AS tools
RUN apk add --no-cache curl unzip
ARG TARGETARCH
# renovate: datasource=github-releases depName=opentofu/opentofu
ENV OPENTOFU_VERSION=1.11.0
# renovate: datasource=github-releases depName=hashicorp/terraform
ENV TERRAFORM_VERSION=1.11.3
# renovate: datasource=github-releases depName=gruntwork-io/terragrunt
ENV TERRAGRUNT_VERSION=0.78.0
#ENV PULUMI_VERSION="3.116.1"
#ENV POWERSHELL_VERSION="3.116.1"

RUN wget https://github.com/opentofu/opentofu/releases/download/v${OPENTOFU_VERSION}/tofu_${OPENTOFU_VERSION}_linux_${TARGETARCH}.tar.gz && \
    tar xf tofu_${OPENTOFU_VERSION}_linux_${TARGETARCH}.tar.gz -C /tmp && \
    rm tofu_${OPENTOFU_VERSION}_linux_${TARGETARCH}.tar.gz

RUN curl -O https://releases.hashicorp.com/terraform/${TERRAFORM_VERSION}/terraform_${TERRAFORM_VERSION}_linux_${TARGETARCH}.zip && \
    unzip terraform_${TERRAFORM_VERSION}_linux_${TARGETARCH}.zip -d /tmp && \
    rm terraform_${TERRAFORM_VERSION}_linux_${TARGETARCH}.zip

RUN wget -O /tmp/terragrunt https://github.com/gruntwork-io/terragrunt/releases/download/v${TERRAGRUNT_VERSION}/terragrunt_linux_${TARGETARCH} && \
    chmod +x /tmp/terragrunt

FROM alpine:3.24 AS runtime

ARG TARGETARCH="amd64"
# renovate: datasource=pypi depName=ansible
ARG ANSIBLE_VERSION=13.5.0
ENV ANSIBLE_VERSION=${ANSIBLE_VERSION}
ARG ANSIBLE_VENV_PATH=/opt/jeh/apps/ansible/${ANSIBLE_VERSION}/venv

RUN apk add --no-cache -U \
    bash curl git gnupg mysql-client openssh-client-default python3 py3-pip rsync sshpass tar tini tzdata unzip wget zip jq && \
    rm -rf /var/cache/apk/* && \
    adduser -D -u 1001 -G root jeh && \
    mkdir -p /tmp/jeh && \
    mkdir -p /etc/jeh && \
    mkdir -p /var/lib/jeh && \
    mkdir -p /opt/jeh && \
    chown -R jeh:0 /tmp/jeh && \
    chown -R jeh:0 /etc/jeh && \
    chown -R jeh:0 /var/lib/jeh && \
    chown -R jeh:0 /opt/jeh && \
    find /usr/lib/python* -iname __pycache__ | xargs rm -rf

RUN echo $'Host *\n  StrictHostKeyChecking no\n  UserKnownHostsFile /dev/null' > /etc/ssh/ssh_config.d/jeh.conf

COPY --chown=1001:0 ./deployment/docker/server/ansible.cfg /etc/ansible/ansible.cfg
COPY deployment/docker/server/server-wrapper /usr/local/bin/
COPY --from=builder /out/jeh /usr/local/bin/
COPY --from=tools /tmp/tofu /usr/local/bin/
COPY --from=tools /tmp/terraform /usr/local/bin/
COPY --from=tools /tmp/terragrunt /usr/local/bin/

RUN chown -R jeh:0 /usr/local/bin/server-wrapper && \
    chmod +x /usr/local/bin/server-wrapper && \
    chown -R jeh:0 /usr/local/bin/jeh && \
    chmod +x /usr/local/bin/jeh

WORKDIR /home/jeh

RUN apk add --no-cache -U python3-dev build-base openssl-dev libffi-dev cargo && \
     mkdir -p ${ANSIBLE_VENV_PATH} && \
     python3 -m venv ${ANSIBLE_VENV_PATH} --system-site-packages && \
     source ${ANSIBLE_VENV_PATH}/bin/activate && \
     pip3 install --upgrade pip ansible==${ANSIBLE_VERSION} boto3 botocore requests pywinrm passlib paramiko && \
     apk del python3-dev build-base openssl-dev libffi-dev cargo && \
     rm -rf /var/cache/apk/* && \
     find ${ANSIBLE_VENV_PATH} -iname __pycache__ | xargs rm -rf && \
     chown -R jeh:0 /opt/jeh

USER 1001
EXPOSE 3000

ENV VIRTUAL_ENV="$ANSIBLE_VENV_PATH"
ENV PATH="$ANSIBLE_VENV_PATH/bin:$PATH"

# Preventing ansible zombie processes. Tini kills zombies.
ENTRYPOINT ["/sbin/tini", "--"]

LABEL org.opencontainers.image.title="Job Executor Hub" \
      org.opencontainers.image.description="JEH — job automation for Ansible, Terraform, OpenTofu and scripts" \
      org.opencontainers.image.source="https://github.com/impishMD/jeh" \
      org.opencontainers.image.licenses="MIT"
COPY LICENSE NOTICE THIRD-PARTY-LICENSES.md /usr/share/licenses/jeh/
COPY --chown=1001:0 --chmod=755 deployment/docker/runner/runner-wrapper /usr/local/bin/

FROM runtime AS runner
CMD ["/usr/local/bin/runner-wrapper"]

FROM runtime AS server
CMD ["/usr/local/bin/server-wrapper"]
