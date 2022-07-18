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

package v1beta1

import (
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var helmchartproxylog = logf.Log.WithName("helmchartproxy-resource")

func (r *HelmChartProxy) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

//+kubebuilder:webhook:path=/mutate-addons-cluster-x-k8s-io-v1beta1-helmchartproxy,mutating=true,failurePolicy=fail,sideEffects=None,groups=addons.cluster.x-k8s.io,resources=helmchartproxies,verbs=create;update,versions=v1beta1,name=mhelmchartproxy.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &HelmChartProxy{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (p *HelmChartProxy) Default() {
	helmchartproxylog.Info("default", "name", p.Name)

	if p.Spec.Namespace == "" {
		p.Spec.Namespace = "default"
	}
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-addons-cluster-x-k8s-io-v1beta1-helmchartproxy,mutating=false,failurePolicy=fail,sideEffects=None,groups=addons.cluster.x-k8s.io,resources=helmchartproxies,verbs=create;update,versions=v1beta1,name=vhelmchartproxy.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &HelmChartProxy{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *HelmChartProxy) ValidateCreate() error {
	helmchartproxylog.Info("validate create", "name", r.Name)

	// TODO(user): fill in your validation logic upon object creation.
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *HelmChartProxy) ValidateUpdate(old runtime.Object) error {
	helmchartproxylog.Info("validate update", "name", r.Name)

	// TODO(user): fill in your validation logic upon object update.
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *HelmChartProxy) ValidateDelete() error {
	helmchartproxylog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}
