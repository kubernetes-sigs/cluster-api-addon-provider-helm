#!/usr/bin/env bash

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

set -o errexit
set -o nounset

BASE_DIR=$(dirname "$0")
CAPI_DIR=${BASE_DIR}/../../cluster-api
TILTD_DIR=${CAPI_DIR}/tilt.d
TILT_FILE=${TILTD_DIR}/calico_tiltfile

# Check and create directories.
[ -d "${CAPI_DIR}" ] && [ ! -d "${TILTD_DIR}" ] && mkdir -p "${TILTD_DIR}"

# Generate the calico_tiltfile.
cat <<EOF > "${TILT_FILE}"
# -*- mode: Python -*-

yaml_file = "../../cluster-api-addon-provider-helm/config/samples/calico-cni.yaml"
k8s_yaml(yaml_file)

# Create k8s resource.
k8s_resource(objects=['calico-cni:helmchartproxy'], labels = ['CAAPH.helmchartproxy'], resource_deps = ['caaph_controller','uncategorized'],new_name='calico-cni')
EOF
