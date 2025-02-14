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
	"net/url"
	"time"

	"github.com/Masterminds/semver/v3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var helmchartproxylog = logf.Log.WithName("helmchartproxy-resource")

func (r *HelmChartProxy) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

//+kubebuilder:webhook:path=/mutate-addons-cluster-x-k8s-io-v1alpha1-helmchartproxy,mutating=true,failurePolicy=fail,sideEffects=None,groups=addons.cluster.x-k8s.io,resources=helmchartproxies,verbs=create;update,versions=v1alpha1,name=helmchartproxy.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &HelmChartProxy{}

const helmTimeout = 10 * time.Minute

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (p *HelmChartProxy) Default() {
	helmchartproxylog.Info("default", "name", p.Name)

	if p.Spec.ReleaseNamespace == "" {
		p.Spec.ReleaseNamespace = "default"
	}

	if p.Spec.Options.Atomic {
		p.Spec.Options.Wait = true
	}

	// Note: timeout is also needed to ensure that Spec.Options.Wait works.
	if p.Spec.Options.Timeout == nil {
		p.Spec.Options.Timeout = &metav1.Duration{Duration: helmTimeout}
	}
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-addons-cluster-x-k8s-io-v1alpha1-helmchartproxy,mutating=false,failurePolicy=fail,sideEffects=None,groups=addons.cluster.x-k8s.io,resources=helmchartproxies,verbs=create;update,versions=v1alpha1,name=vhelmchartproxy.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &HelmChartProxy{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (p *HelmChartProxy) ValidateCreate() (admission.Warnings, error) {
	helmchartproxylog.Info("validate create", "name", p.Name)

	var allErrs field.ErrorList

	if err := isUrlValid(p.Spec.RepoURL); err != nil {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "RepoURL"),
				p.Spec.ReleaseNamespace, err.Error()),
		)
	}

	if errs := validateVersionMap(p); len(errs) > 0 {
		allErrs = append(allErrs, errs...)
	}

	if len(allErrs) > 0 {
		return nil, apierrors.NewInvalid(GroupVersion.WithKind("HelmChartProxy").GroupKind(), p.Name, allErrs)
	}

	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (p *HelmChartProxy) ValidateUpdate(oldRaw runtime.Object) (admission.Warnings, error) {
	helmchartproxylog.Info("validate update", "name", p.Name)

	var allErrs field.ErrorList
	old, ok := oldRaw.(*HelmChartProxy)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a HelmChartProxy but got a %T", old))
	}

	if err := isUrlValid(p.Spec.RepoURL); err != nil {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "RepoURL"),
				p.Spec.ReleaseNamespace, err.Error()),
		)
	}

	if errs := validateVersionMap(p); len(errs) > 0 {
		allErrs = append(allErrs, errs...)
	}

	if p.Spec.ReconcileStrategy != old.Spec.ReconcileStrategy {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "ReconcileStrategy"),
				p.Spec.ReconcileStrategy, "field is immutable"),
		)
	}

	if len(allErrs) > 0 {
		return nil, apierrors.NewInvalid(GroupVersion.WithKind("HelmChartProxy").GroupKind(), p.Name, allErrs)
	}

	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (p *HelmChartProxy) ValidateDelete() (admission.Warnings, error) {
	helmchartproxylog.Info("validate delete", "name", p.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil, nil
}

// isUrlValid returns nil if specified repoURL is valid as per go doc https://pkg.go.dev/net/url#ParseRequestURI and an error otherwise.
func isUrlValid(repoURL string) error {
	_, err := url.ParseRequestURI(repoURL)
	if err != nil {
		return fmt.Errorf("specified repoURL %s is not valid: %w", repoURL, err)
	}

	return nil
}

// validateVersionMap validates that if the versionMap is set, the keys and values are valid semver constraints and that the version is not also set.
func validateVersionMap(p *HelmChartProxy) field.ErrorList {
	var allErrs field.ErrorList
	if p.Spec.VersionMap != nil {
		if p.Spec.Version != "" {
			allErrs = append(allErrs, field.Forbidden(field.NewPath("spec", "Version"), "version and versionMap cannot be set at the same time"))
		}

		for k8sVersion, chartVersion := range p.Spec.VersionMap {
			if _, err := semver.NewConstraint(k8sVersion); err != nil {
				allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "VersionMap"), k8sVersion, fmt.Sprintf("failed to parse Kubernetes version constraint '%s'", k8sVersion)))
			}

			if _, err := semver.NewConstraint(chartVersion); err != nil {
				allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "VersionMap"), chartVersion, fmt.Sprintf("failed to parse Helm chart version constraint '%s'", chartVersion)))
			}
		}
	}

	return allErrs
}
