#!/bin/bash

minikube logs > /tmp/receptor-testing/minikube.log

PODS_DIR=/tmp/receptor-testing/K8sPods

mkdir "$PODS_DIR"
PODS="$(kubectl get pods --template '{{range.items}}{{.metadata.name}}{{"\n"}}{{end}}')"

for pod in $PODS ; do
    mkdir "$PODS_DIR/$pod"
    kubectl get pod "$pod" --output=json > "$PODS_DIR/$pod/pod"
    kubectl logs "$pod" > "$PODS_DIR/$pod/logs"
done
