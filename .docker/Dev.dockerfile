FROM debian:trixie-slim AS rustup

ENV HOME=/opt
RUN apt-get update && apt-get install -y curl && \
    curl -o /tmp/rustup-installer.sh --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs && \
    sh /tmp/rustup-installer.sh -y

RUN BIN_DIR=$(find /opt/.rustup/ -name rustc | grep /bin/ | xargs -I {} dirname {}) && \
    echo "Rust found in: ${BIN_DIR}" && \
    cd $(dirname ${BIN_DIR}) && mkdir /opt/rust && cp -r . /opt/rust/ && \
    echo "Files copied: $(find /opt/rust -type f | wc -l)" && \
    find /opt/rust -type f | grep -e /bin/ -e /lib/

# ------------------------------------------------------------
FROM golang:1.25 AS golang

ARG GO_USER_ID=1001
ARG GO_USER_NAME

ENV GO_USER_ID=${GO_USER_ID}
ENV GO_USER_NAME=${GO_USER_NAME}

ENV GOCACHE=/tmp/.cache/go/build
ENV GOMODCACHE=/tmp/.cache/go/pkg/mod

# COPY --from=rustup /opt/ /opt/
COPY --from=rustup /opt/rust/ /usr/local/
COPY --chmod=0755 .docker/scripts/go-* /usr/local/bin/

RUN apt-get update && apt-get install -y make protobuf-compiler sudo zsh && \
    \
    groupadd -g "${GO_USER_ID}" "${GO_USER_NAME}" && \
    useradd -m -u "${GO_USER_ID}" -g "${GO_USER_ID}" "${GO_USER_NAME}" && \
    echo "${GO_USER_NAME} ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers && \
    echo "User ${GO_USER_NAME} created with ID: ${GO_USER_ID}"

# ------------------------------------------------------------
FROM golang AS devcontainer

RUN go env
RUN bash /usr/local/bin/go-install-vscode-tools

COPY .docker/zshrc /home/${GO_USER_NAME}/.zshrc

CMD ["/bin/bash"]
