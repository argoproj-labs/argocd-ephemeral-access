# argocd-ephemeral-access

## Overview

This project is an Argo CD extension to allow ephemeral access in Argo
CD UI. It can be viewed as something similar to the functionality that
`sudo` command provides as users can execute actions that require
higher permissions. The exact access the user is allowed to be
elevated to and for how long the access should be granted are
configurable. The elevated access are automatically managed by
creating and updating Argo CD AppProject roles. 

Note: This project requires that the Argo CD `Applications` are
associated with an `AppProjects` different than `default`.

## How it Works

This project provides a set of CRDs that are used to configure the
behaviour of how the Argo CD access can be elevated. The CRDs provided
as part of this project are described below:

### RoleTemplate

The `RoleTemplate` defines a templated Argo CD RBAC policies. Once the
elevated access is requested and approved, the policies will be
rendered and dynamicaly associated with the AppProject related with
the access request. 

```yaml
apiVersion: ephemeral-access.argoproj-labs.io/v1alpha1
kind: RoleTemplate
metadata:
  name: devops
spec:
  description: write permission in application {{.application}}
  name: "devops"
  policies:
  - p, {{.role}}, applications, sync, {{.project}}/{{.application}}, allow
  - p, {{.role}}, applications, action/*, {{.project}}/{{.application}}, allow
  - p, {{.role}}, applications, delete/*/Pod/*, {{.project}}/{{.application}}, allow
```

### AccessBinding

```yaml
apiVersion: ephemeral-access.argoproj-labs.io/v1alpha1
kind: AccessBinding
metadata:
  name: some-access-binding
spec:
  roleTemplateRef:
    name: devops
  subjects: 
    - group1
  if: "true"
  ordinal: 1
  friendlyName: "Devops (Write)"
```

### AccessRequest

```yaml
apiVersion: ephemeral-access.argoproj-labs.io/v1alpha1
kind: AccessRequest
metadata:
  name: some-application-username
  namespace: ephemeral
spec:
  application:
    name: ephemeral
    namespace: argocd
  duration: '1m'
  role:
    friendlyName: Devops (Write)
    ordinal: 1
    templateName: devops
  subject:
    username: some_user@fakedomain.com
```

## Installation

The ephemeral-access functionality is provided by the following
components that needs to be configured properly to achieve the desired
behaviour:

- ui: Argo CD UI extension that provides users with the functionality
  to request elevated access to an Argo CD Application.
- backend: Serves the REST API used by the UI extension.
- controller: Responsible for reconciling the AccessRequest resource.

### Installing the Backend and the Controller

We provide a consolidated `install.yaml` asset file in every release.
The `install.yaml` file contains all the resources required to run the
backend service and the controller. Check the latest release in the
[releases page][1] and replace the `DESIRED_VERSION` in the command
below.

```bash
kubectl apply -f https://github.com/argoproj-labs/argocd-ephemeral-access/releases/download/<DESIRED_VERSION>/install.yaml
```

This command will create a new namespace `argocd-ephemeral-access` and
deploy the necessary resources.

### Install UI extension

The UI extension needs to be installed by mounting the React component
in Argo CD API server. This process can be automated by using the
[argocd-extension-installer][2]. This installation method will run an
init container that will download, extract and place the file in the
correct location.

The yaml file below is an example of how to define a kustomize patch
to install this UI extension:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: argocd-server
spec:
  template:
    spec:
      initContainers:
        - name: extension-metrics
          image: quay.io/argoprojlabs/argocd-extension-installer:v0.0.8@sha256:e7cb054207620566286fce2d809b4f298a72474e0d8779ffa8ec92c3b630f054
          env:
          - name: EXTENSION_URL
            value: https://github.com/argoproj-labs/argocd-ephemeral-access/releases/download/v0.0.1/extension.tar.gz
          - name: EXTENSION_CHECKSUM_URL
            value: https://github.com/argoproj-labs/argocd-ephemeral-access/releases/download/v0.0.1/extension_checksums.txt
          volumeMounts:
            - name: extensions
              mountPath: /tmp/extensions/
          securityContext:
            runAsUser: 1000
            allowPrivilegeEscalation: false
      containers:
        - name: argocd-server
          volumeMounts:
            - name: extensions
              mountPath: /tmp/extensions/
      volumes:
        - name: extensions
          emptyDir: {}
```

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
[2]: https://github.com/argoproj-labs/argocd-extension-installer
