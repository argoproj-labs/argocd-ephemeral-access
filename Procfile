controller: COMMAND=./bin/ephemeral-access && sh -c "EPHEMERAL_CONTROLLER_HEALTH_PROBE_ADDR=:8989 EPHEMERAL_CONTROLLER_PORT=9091 $COMMAND controller"
backend: COMMAND=./bin/ephemeral-access && sh -c "EPHEMERAL_BACKEND_NAMESPACE=ephemeral KUBECONFIG=${KUBECONFIG:-~/.kube/config} $COMMAND backend"
tracing: sh -c "docker run --rm --name ephemeral-access-jaeger -p 16686:16686 -p 4317:4317 -p 4318:4318 jaegertracing/all-in-one:latest"
