#!/bin/sh
set -euox pipefail

# This script will validate and copy the file defined in the
# PLUGIN_PATH env var to the plugin folder (/tmp/plugins).

plugin_path="${PLUGIN_PATH:-}"
if [ "$plugin_path" = "" ]; then
    echo "error: the env var PLUGIN_PATH must be provided"
    exit 1
fi

if [ ! -f "$plugin_path" ]; then
    echo "error: no file found in the provided PLUGIN_PATH: $plugin_path"
    exit 1
fi

mkdir -p /tmp/plugin
cp "$plugin_path" /tmp/plugin/
echo "Plugin installed successfully"
