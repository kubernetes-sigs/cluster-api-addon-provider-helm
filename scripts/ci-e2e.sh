#!/bin/bash

# Copyright 2024 The Kubernetes Authors.
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

###############################################################################

set -o errexit
set -o nounset
set -o pipefail

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..

# shellcheck source=hack/ensure-go.sh
source "${REPO_ROOT}/hack/ensure-go.sh"

export USE_LOCAL_KIND_REGISTRY=${USE_LOCAL_KIND_REGISTRY:-"true"} 
export BUILD_MANAGER_IMAGE=${BUILD_MANAGER_IMAGE:-"true"}

if [[ "${USE_LOCAL_KIND_REGISTRY}" == "true" ]]; then
  export REGISTRY="localhost:5000/ci-e2e"
fi

if [[ "${BUILD_MANAGER_IMAGE}" == "true" ]]; then
  defaultTag=$(date -u '+%Y%m%d%H%M%S')
  export TAG="${defaultTag:-dev}"
fi

export GINKGO_NODES=10

# Image is configured as `${CONTROLLER_IMG}-${ARCH}:${TAG}` where `CONTROLLER_IMG` is defaulted to `${REGISTRY}/${IMAGE_NAME}`.
if [[ "${BUILD_MANAGER_IMAGE}" == "false" ]]; then
  # Load an existing image, skip docker-build and docker-push.
  make test-e2e-run
elif [[ "${USE_LOCAL_KIND_REGISTRY}" == "true" ]]; then
  # Build an image with kind local registry, skip docker-push. REGISTRY is set to `localhost:5000/ci-e2e`. TAG is set to `$(date -u '+%Y%m%d%H%M%S')`.
  make test-e2e-local
else
  # Build an image and push to the registry. TAG is set to `$(date -u '+%Y%m%d%H%M%S')`.
  make test-e2e
fi
