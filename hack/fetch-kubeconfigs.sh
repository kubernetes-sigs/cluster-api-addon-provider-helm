#!/bin/bash

# Copyright 2023 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

ROOT=$(dirname "${BASH_SOURCE[0]}")/..
TEMP_PATH="${ROOT}"/tmp

if [ ! -d "${TEMP_PATH}" ]; then
  mkdir "${TEMP_PATH}"
fi

CLUSTERS=$(kubectl get clusters -A -o jsonpath="{range .items[*].metadata}{.namespace}{'\t'}{.name}{'\n'}{end}")
# Read each Cluster's namespace and names on a line

IFS=$'\n'; for CLUSTER in ${CLUSTERS}; do
  IFS=$'\t' read -r NAMESPACE NAME <<< "${CLUSTER}"
  clusterctl get kubeconfig -n "${NAMESPACE}" "${NAME}" > "${TEMP_PATH}/${NAMESPACE}-${NAME}.kubeconfig"
  echo "Fetched kubeconfig for ${NAMESPACE}/${NAME}"
done