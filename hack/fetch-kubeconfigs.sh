#!/bin/bash

ROOT=$(dirname "${BASH_SOURCE[0]}")/..
TEMP_PATH=$ROOT/tmp

if [ ! -d $TEMP_PATH ]; then
  mkdir $TEMP_PATH
fi

CLUSTERS=$(kubectl get clusters -A -o jsonpath="{range .items[*].metadata}{.namespace}{'\t'}{.name}{'\n'}{end}")
# Read each Cluster's namespace and names on a line

IFS=$'\n'; for CLUSTER in $CLUSTERS; do
  IFS=$'\t' read -r NAMESPACE NAME <<< $CLUSTER
  clusterctl get kubeconfig -n $NAMESPACE $NAME > $TEMP_PATH/$NAMESPACE-$NAME.kubeconfig
  echo "Fetched kubeconfig for $NAMESPACE/$NAME"
done