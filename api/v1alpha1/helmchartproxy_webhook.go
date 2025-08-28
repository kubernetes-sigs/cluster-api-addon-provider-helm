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
	"fmt"
	"net/url"
	"time"

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
	w := new(helmChartProxyWebhook)

	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		WithDefaulter(w).
		WithValidator(w).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-addons-cluster-x-k8s-io-v1alpha1-helmchartproxy,mutating=true,failurePolicy=fail,sideEffects=None,groups=addons.cluster.x-k8s.io,resources=helmchartproxies,verbs=create;update,versions=v1alpha1,name=helmchartproxy.kb.io,admissionReviewVersions=v1

type helmChartProxyWebhook struct{}

var (
	_ webhook.CustomValidator = &helmChartProxyWebhook{}
	_ webhook.CustomDefaulter = &helmChartProxyWebhook{}
)

const helmTimeout = 10 * time.Minute

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (*helmChartProxyWebhook) Default(_ context.Context, objRaw runtime.Object) error {
	newObj, ok := objRaw.(*HelmChartProxy)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected a HelmChartProxy but got a %T", objRaw))
	}
	helmchartproxylog.Info("default", "name", newObj.Name)

	if newObj.Spec.ReleaseNamespace == "" {
		newObj.Spec.ReleaseNamespace = "default"
	}

	if newObj.Spec.Options.Atomic {
		newObj.Spec.Options.Wait = true
	}

	// Note: timeout is also needed to ensure that Spec.Options.Wait works.
	if newObj.Spec.Options.Timeout == nil {
		newObj.Spec.Options.Timeout = &metav1.Duration{Duration: helmTimeout}
	}

	return nil
}

//+kubebuilder:webhook:path=/validate-addons-cluster-x-k8s-io-v1alpha1-helmchartproxy,mutating=false,failurePolicy=fail,sideEffects=None,groups=addons.cluster.x-k8s.io,resources=helmchartproxies,verbs=create;update,versions=v1alpha1,name=vhelmchartproxy.kb.io,admissionReviewVersions=v1

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (*helmChartProxyWebhook) ValidateCreate(_ context.Context, objRaw runtime.Object) (admission.Warnings, error) {
	newObj, ok := objRaw.(*HelmChartProxy)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a HelmChartProxy but got a %T", objRaw))
	}

	helmchartproxylog.Info("validate create", "name", newObj.Name)

	if err := isUrlValid(newObj.Spec.RepoURL); err != nil {
		return nil, err
	}

	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (*helmChartProxyWebhook) ValidateUpdate(_ context.Context, oldRaw, newRaw runtime.Object) (admission.Warnings, error) {
	var allErrs field.ErrorList
	oldObj, ok := oldRaw.(*HelmChartProxy)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a HelmChartProxy but got a %T", oldRaw))
	}
	newObj, ok := newRaw.(*HelmChartProxy)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a HelmChartProxy but got a %T", newRaw))
	}

	helmchartproxylog.Info("validate update", "name", newObj.Name)

	if err := isUrlValid(newObj.Spec.RepoURL); err != nil {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "RepoURL"),
				newObj.Spec.ReleaseNamespace, err.Error()),
		)
	}

	if newObj.Spec.ReconcileStrategy != oldObj.Spec.ReconcileStrategy {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "ReconcileStrategy"),
				newObj.Spec.ReconcileStrategy, "field is immutable"),
		)
	}

	if len(allErrs) > 0 {
		return nil, apierrors.NewInvalid(GroupVersion.WithKind("HelmChartProxy").GroupKind(), newObj.Name, allErrs)
	}

	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (*helmChartProxyWebhook) ValidateDelete(_ context.Context, objRaw runtime.Object) (admission.Warnings, error) {
	obj, ok := objRaw.(*HelmChartProxy)
	if !ok {
		return nil, fmt.Errorf("expected a HelmChartProxy object but got %T", objRaw)
	}

	helmchartproxylog.Info("validate delete", "name", obj.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil, nil
}

// isUrlValid returns true if specified repoURL is valid as per go doc https://pkg.go.dev/net/url#ParseRequestURI.
func isUrlValid(repoURL string) error {
	if _, err := url.ParseRequestURI(repoURL); err != nil {
		return fmt.Errorf("specified repoURL %s is not valid: %w", repoURL, err)
	}

	return nil
}
