/*
Copyright 2022 The Kubernetes Authors.

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

package v1alpha1

import (
	"context"
	"reflect"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var helmreleaseproxylog = logf.Log.WithName("helmreleaseproxy-resource")

func (r *HelmReleaseProxy) SetupWebhookWithManager(mgr ctrl.Manager) error {
	w := new(helmReleaseProxyWebhook)

	return ctrl.NewWebhookManagedBy(mgr, r).
		WithDefaulter(w).
		WithValidator(w).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-addons-cluster-x-k8s-io-v1alpha1-helmreleaseproxy,mutating=true,failurePolicy=fail,sideEffects=None,groups=addons.cluster.x-k8s.io,resources=helmreleaseproxies,verbs=create;update,versions=v1alpha1,name=helmreleaseproxy.kb.io,admissionReviewVersions=v1

type helmReleaseProxyWebhook struct{}

var (
	_ admission.Validator[*HelmReleaseProxy] = &helmReleaseProxyWebhook{}
	_ admission.Defaulter[*HelmReleaseProxy] = &helmReleaseProxyWebhook{}
)

// Default implements admission.Defaulter so a webhook will be registered for the type.
func (*helmReleaseProxyWebhook) Default(_ context.Context, obj *HelmReleaseProxy) error {
	helmreleaseproxylog.Info("default", "name", obj.Name)

	if obj.Spec.ReleaseNamespace == "" {
		obj.Spec.ReleaseNamespace = "default"
	}

	return nil
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-addons-cluster-x-k8s-io-v1alpha1-helmreleaseproxy,mutating=false,failurePolicy=fail,sideEffects=None,groups=addons.cluster.x-k8s.io,resources=helmreleaseproxies,verbs=create;update,versions=v1alpha1,name=vhelmreleaseproxy.kb.io,admissionReviewVersions=v1

// ValidateCreate implements admission.Validator so a webhook will be registered for the type.
func (*helmReleaseProxyWebhook) ValidateCreate(_ context.Context, newObj *HelmReleaseProxy) (admission.Warnings, error) {
	helmreleaseproxylog.Info("validate create", "name", newObj.Name)

	// TODO(user): fill in your validation logic upon object creation.
	return nil, nil
}

// ValidateUpdate implements admission.Validator so a webhook will be registered for the type.
func (*helmReleaseProxyWebhook) ValidateUpdate(_ context.Context, oldObj, newObj *HelmReleaseProxy) (admission.Warnings, error) {
	var allErrs field.ErrorList

	helmreleaseproxylog.Info("validate update", "name", newObj.Name)

	if !reflect.DeepEqual(newObj.Spec.RepoURL, oldObj.Spec.RepoURL) {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "RepoURL"),
				newObj.Spec.RepoURL, "field is immutable"),
		)
	}

	if !reflect.DeepEqual(newObj.Spec.ChartName, oldObj.Spec.ChartName) {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "ChartName"),
				newObj.Spec.ChartName, "field is immutable"),
		)
	}

	if !reflect.DeepEqual(newObj.Spec.ReleaseNamespace, oldObj.Spec.ReleaseNamespace) {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "ReleaseNamespace"),
				newObj.Spec.ReleaseNamespace, "field is immutable"),
		)
	}

	if newObj.Spec.ReconcileStrategy != oldObj.Spec.ReconcileStrategy {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "ReconcileStrategy"),
				newObj.Spec.ReconcileStrategy, "field is immutable"),
		)
	}

	// TODO: add webhook for ReleaseName. Currently it's being set if the release name is generated.

	if len(allErrs) > 0 {
		return nil, apierrors.NewInvalid(GroupVersion.WithKind("HelmReleaseProxy").GroupKind(), newObj.Name, allErrs)
	}

	return nil, nil
}

// ValidateDelete implements admission.Validator so a webhook will be registered for the type.
func (*helmReleaseProxyWebhook) ValidateDelete(_ context.Context, obj *HelmReleaseProxy) (admission.Warnings, error) {
	helmreleaseproxylog.Info("validate delete", "name", obj.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil, nil
}
