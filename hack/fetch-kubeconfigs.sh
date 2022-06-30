#!/bin/bash

ROOT=$(dirname "${BASH_SOURCE[0]}")/..
TEMP_PATH=$ROOT/tmp

if [ ! -d $TEMP_PATH ]; then
  mkdir $TEMP_PATH
fi

CLUSTER_NAMES=$(kubectl get clusters -o jsonpath="{.items[*].metadata.name}")
for CLUSTER_NAME in $(echo $CLUSTER_NAMES); do
    clusterctl get kubeconfig $CLUSTER_NAME > $TEMP_PATH/$CLUSTER_NAME.kubeconfig
done