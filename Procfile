controller: COMMAND=./bin/ephemeral-access && sh -c "EPHEMERAL_LOG_LEVEL=debug EPHEMERAL_CONTROLLER_HEALTH_PROBE_ADDR=:8989 EPHEMERAL_CONTROLLER_PORT=9091 EPHEMERAL_METRICS_ADDR=:8090 $COMMAND controller"
backend: COMMAND=./bin/ephemeral-access && sh -c "EPHEMERAL_BACKEND_NAMESPACE=ephemeral KUBECONFIG=${KUBECONFIG:-~/.kube/config} EPHEMERAL_METRICS_ADDR=:8091 $COMMAND backend"
