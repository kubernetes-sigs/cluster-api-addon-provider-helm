/*
Copyright 2022.

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
	"reflect"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// log is for logging in this package.
var helmreleaseproxylog = logf.Log.WithName("helmreleaseproxy-resource")

func (r *HelmReleaseProxy) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

//+kubebuilder:webhook:path=/mutate-addons-cluster-x-k8s-io-v1alpha1-helmreleaseproxy,mutating=true,failurePolicy=fail,sideEffects=None,groups=addons.cluster.x-k8s.io,resources=helmreleaseproxies,verbs=create;update,versions=v1alpha1,name=mhelmreleaseproxy.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &HelmReleaseProxy{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (p *HelmReleaseProxy) Default() {
	helmreleaseproxylog.Info("default", "name", p.Name)

	if p.Spec.Namespace == "" {
		p.Spec.Namespace = "default"
	}
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-addons-cluster-x-k8s-io-v1alpha1-helmreleaseproxy,mutating=false,failurePolicy=fail,sideEffects=None,groups=addons.cluster.x-k8s.io,resources=helmreleaseproxies,verbs=create;update,versions=v1alpha1,name=vhelmreleaseproxy.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &HelmReleaseProxy{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *HelmReleaseProxy) ValidateCreate() error {
	helmreleaseproxylog.Info("validate create", "name", r.Name)

	// TODO(user): fill in your validation logic upon object creation.
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *HelmReleaseProxy) ValidateUpdate(oldRaw runtime.Object) error {
	helmreleaseproxylog.Info("validate update", "name", r.Name)

	var allErrs field.ErrorList
	old := oldRaw.(*HelmReleaseProxy)

	if !reflect.DeepEqual(r.Spec.RepoURL, old.Spec.RepoURL) {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "RepoURL"),
				r.Spec.RepoURL, "field is immutable"),
		)
	}

	if !reflect.DeepEqual(r.Spec.ChartName, old.Spec.ChartName) {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "ChartName"),
				r.Spec.ChartName, "field is immutable"),
		)
	}

	if !reflect.DeepEqual(r.Spec.Namespace, old.Spec.Namespace) {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "Namespace"),
				r.Spec.Namespace, "field is immutable"),
		)
	}

	// TODO: add webhook for ReleaseName. Currently it's being set if the release name is generated.

	if len(allErrs) > 0 {
		return apierrors.NewInvalid(GroupVersion.WithKind("HelmReleaseProxy").GroupKind(), r.Name, allErrs)
	}

	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *HelmReleaseProxy) ValidateDelete() error {
	helmreleaseproxylog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}
