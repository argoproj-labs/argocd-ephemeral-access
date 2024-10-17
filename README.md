# argocd-ephemeral-access

## Overview

This project is an Argo CD extension to allow ephemeral access in Argo
CD UI. It can be viewed as something similar to the functionality that
`sudo` command provides as users can execute actions that require
higher permissions.

## How it Works

## Installing

We provide a consolidated `install.yaml` asset file in every release.
Check the latest release in the [releases page][1] and replace the
`DESIRED_VERSION` in the command below.

```bash
kubectl apply -f https://github.com/argoproj-labs/argocd-ephemeral-access/releases/download/<DESIRED_VERSION>/install.yaml
```

This command will create a new namespace `argocd-ephemeral-access` and
deploy the necessary resources.

## Usage

## Contributing

### Development

All necessary development tasks are available as `make targets`. Some
of the targets will make use of external tools. All external tools
used by this project will be automatically downloaded in the `bin`
folder of this repo as required.

#### Building

For building all component of this project simply run `make` (which is
an alias for `make build`). This target will:
- Build the Go project. The binary will be placed in the `bin` folder
  of this repo.
- Build the UI extension. The UI extension will be packaged in the
  `ui/extension.tar` file in this repo.

There are specific targets that can be used to build each component:

```bash
make build-go # for building the Go project only
make build-ui # for building the UI extension only
```

To build a docker image run one of the following commands:

```bash
make docker-build
make docker-buildx
```

To build a docker image with custom namespace and tag run

```bash
IMAGE_NAMESPACE="my.company.com/argoproj-labs" IMAGE_TAG="$(git rev-parse --abbrev-ref HEAD)" make docker-build
```

#### Running

```bash
make run
```

#### Deploying

Install in kubernetes cluster

```bash
IMAGE_NAMESPACE="my.company.com/argoproj-labs" IMAGE_TAG="$(git rev-parse --abbrev-ref HEAD)" make deploy
```

#### Releasing

[1]: https://github.com/argoproj-labs/argocd-ephemeral-access/releases
