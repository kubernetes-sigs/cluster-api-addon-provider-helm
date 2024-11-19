/*
Copyright 2024 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package internal

import (
	addonsv1alpha1 "sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
)

// HasHelmReleaseBeenSuccessfullyInstalled returns true if the Helm chart has been successfully installed at least once
// for a HelmReleaseProxy.
func HasHelmReleaseBeenSuccessfullyInstalled(hrp *addonsv1alpha1.HelmReleaseProxy) bool {
	if hrp == nil {
		return false
	}

	if annotations := hrp.GetAnnotations(); annotations != nil {
		_, ok := annotations[addonsv1alpha1.ReleaseSuccessfullyInstalledAnnotation]
		return ok
	}

	return false
}
