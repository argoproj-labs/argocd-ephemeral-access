# Plugin Example

## Overview

This folder provides an example of how EphemeralAccess plugins can be
implemented.

## Prereqs

- Plugin is a feature introduced in the EphemeralAccess v0.1.5
- Plugins need to be implemented in Go

## Details

EphemeralAccess plugins are binaries compiled from a Go code. The
plugin binary needs to be mounted in the pre-configured
EphemeralAccess controller volume.

The strategy used to get the plugin binary available to the
EphemeralAccess controller is:

1. Create a docker image containing the plugin binary.
1. Patch the EphemeralAccess controller to add an init-container that
   uses your plugin docker image.
1. Configure a volume for the plugin directory.
1. Mount the plugin volume in the init-container and in the controller filesystems.
1. Copy the binary from the docker image in the plugin directory.

## How to Create a Plugin

While the steps above may sound complicated and error prone, we
provide a fully functional plugin as part of this directory. As a
plugin writer all you have to do is following the steps below:

1. Copy this plugin directory in a separate Git repo where your plugin
   is going to live.
1. Change the Go module name to where your git repo is defined.
   Example: `go mod edit -module github.com/some-org/plugin-repo`
1. Implement the plugin logic in the `cmd/main.go` following the
   documentation provided in that file.
1. Create a new image building the provided `Dockerfile`. Example:
   `docker build -t myplugin:latest .`
1. Push the new image to your Docker registry. Example: `docker push
   myplugin:latest`
1. Change the `manifests/plugin/controller-patch.yaml` file replacing
   the text `CHANGE_THIS_TO_POINT_TO_YOUR_PLUGIN_IMAGE` with the image
   created in the previous step. Example `myplugin:latest`

If you had success executing the steps above, you can install the
EphemeralAccess extension with your plugin configured by running the
following command:

    kustomize build ./manifests | kubectl apply -f -
