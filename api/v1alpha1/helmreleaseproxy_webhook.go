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
	"fmt"
	"reflect"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var helmreleaseproxylog = logf.Log.WithName("helmreleaseproxy-resource")

func (r *HelmReleaseProxy) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

//+kubebuilder:webhook:path=/mutate-addons-cluster-x-k8s-io-v1alpha1-helmreleaseproxy,mutating=true,failurePolicy=fail,sideEffects=None,groups=addons.cluster.x-k8s.io,resources=helmreleaseproxies,verbs=create;update,versions=v1alpha1,name=helmreleaseproxy.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &HelmReleaseProxy{}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (p *HelmReleaseProxy) Default() {
	helmreleaseproxylog.Info("default", "name", p.Name)

	if p.Spec.ReleaseNamespace == "" {
		p.Spec.ReleaseNamespace = "default"
	}
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-addons-cluster-x-k8s-io-v1alpha1-helmreleaseproxy,mutating=false,failurePolicy=fail,sideEffects=None,groups=addons.cluster.x-k8s.io,resources=helmreleaseproxies,verbs=create;update,versions=v1alpha1,name=vhelmreleaseproxy.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &HelmReleaseProxy{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (p *HelmReleaseProxy) ValidateCreate() (admission.Warnings, error) {
	helmreleaseproxylog.Info("validate create", "name", p.Name)

	// TODO(user): fill in your validation logic upon object creation.
	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (p *HelmReleaseProxy) ValidateUpdate(oldRaw runtime.Object) (admission.Warnings, error) {
	helmreleaseproxylog.Info("validate update", "name", p.Name)

	var allErrs field.ErrorList
	old, ok := oldRaw.(*HelmReleaseProxy)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a HelmReleaseProxy but got a %T", old))
	}

	if !reflect.DeepEqual(p.Spec.RepoURL, old.Spec.RepoURL) {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "RepoURL"),
				p.Spec.RepoURL, "field is immutable"),
		)
	}

	if !reflect.DeepEqual(p.Spec.ChartName, old.Spec.ChartName) {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "ChartName"),
				p.Spec.ChartName, "field is immutable"),
		)
	}

	if !reflect.DeepEqual(p.Spec.ReleaseNamespace, old.Spec.ReleaseNamespace) {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "ReleaseNamespace"),
				p.Spec.ReleaseNamespace, "field is immutable"),
		)
	}

	if p.Spec.ReconcileStrategy != old.Spec.ReconcileStrategy {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "ReconcileStrategy"),
				p.Spec.ReconcileStrategy, "field is immutable"),
		)
	}

	// TODO: add webhook for ReleaseName. Currently it's being set if the release name is generated.

	if len(allErrs) > 0 {
		return nil, apierrors.NewInvalid(GroupVersion.WithKind("HelmReleaseProxy").GroupKind(), p.Name, allErrs)
	}

	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (r *HelmReleaseProxy) ValidateDelete() (admission.Warnings, error) {
	helmreleaseproxylog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil, nil
}
