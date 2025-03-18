FROM golang:1.22 AS builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to
# re-download as much and so that source changes don't invalidate our
# downloaded layer
RUN go mod download

# Copy the go source
COPY cmd/ cmd/

# Build the GOARCH has not a default value to allow the binary be built
# according to the host where the command was called. For example, if we call
# make docker-build in a local env which has the Apple Silicon M1 SO the docker
# BUILDPLATFORM arg will be linux/arm64 when for Apple x86 it will be
# linux/amd64. Therefore, by leaving it empty we can ensure that the container
# and binary shipped on it will have the same platform.
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -a -o ./plugin ./cmd/...

# PLUGIN SECTION

# Base your plugin from the latest tag available of the
# argocd-ephemeral-access-plugin image. This image ships with the necessary
# user permissions as well as the script to properly install the plugin. It is
# recommended to pin a specific version instead of using 'latest'.
FROM quay.io/argoprojlabs/argocd-ephemeral-access-plugin:latest

# The plugin binary should be copied in the /workspace which is the only directory
# where the user nonroot (65532) has access to.
WORKDIR /workspace

# Just add the plugin binary to the current work directory. By convention we
# expect the binary to be named as 'plugin' and living in the /workspace
# directory.
COPY --from=builder --chmod=500 --chown=65532 /workspace/plugin .

# Set the user to the nonroot user defined in the
# argoprojlabs/argocd-ephemeral-access-plugin image.
USER 65532
