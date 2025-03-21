controller: COMMAND=./bin/ephemeral-access && sh -c "EPHEMERAL_LOG_LEVEL=debug EPHEMERAL_CONTROLLER_HEALTH_PROBE_ADDR=:8989 EPHEMERAL_CONTROLLER_PORT=9091 $COMMAND controller"
backend: COMMAND=./bin/ephemeral-access && sh -c "EPHEMERAL_BACKEND_NAMESPACE=ephemeral KUBECONFIG=${KUBECONFIG:-~/.kube/config} $COMMAND backend"
