ARG TASKEXEC_TEST_BASE=taskexec:docker-preview
FROM ${TASKEXEC_TEST_BASE}
COPY --chown=1001:0 . /opt/taskexec-test-repo
RUN printf '%s\n' 'fixture-vault-password' > /tmp/fixture-vault-password && \
    ansible-vault encrypt --vault-password-file /tmp/fixture-vault-password /opt/taskexec-test-repo/vault.yml && \
    rm /tmp/fixture-vault-password && \
    git -C /opt/taskexec-test-repo init -b main && \
    git -C /opt/taskexec-test-repo add . && \
    git -C /opt/taskexec-test-repo -c user.name=Test -c user.email=test@example.invalid commit -m fixture
