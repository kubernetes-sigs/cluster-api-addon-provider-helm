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

package internal

import (
	"context"
	"os"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/cmd/clusterctl/client"
	"sigs.k8s.io/cluster-api/controllers/external"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
)

func WriteClusterKubeconfigToFile(ctx context.Context, cluster *clusterv1.Cluster) (string, error) {
	log := ctrl.LoggerFrom(ctx)
	c, err := client.New("")
	if err != nil {
		return "", err
	}

	options := client.GetKubeconfigOptions{
		Kubeconfig: client.Kubeconfig{},
		// Kubeconfig:          client.Kubeconfig{Path: gk.kubeconfig, Context: gk.kubeconfigContext},
		WorkloadClusterName: cluster.Name,
		Namespace:           cluster.Namespace,
	}

	log.V(4).Info("Getting kubeconfig for cluster", "cluster", cluster.Name)
	kubeconfig, err := c.GetKubeconfig(options)
	if err != nil {
		return "", err
	}
	log.V(4).Info("cluster", "cluster", cluster.Name, "kubeconfig is:", kubeconfig)

	path := "tmp"
	filePath := path + "/" + cluster.Name
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		err := os.Mkdir(path, os.ModePerm)
		if err != nil {
			return "", errors.Wrapf(err, "failed to create directory %s", path)
		}
	}
	f, err := os.Create(filePath)
	if err != nil {
		return "", errors.Wrapf(err, "failed to create file %s", filePath)
	}

	log.V(4).Info("Writing kubeconfig to file", "cluster", cluster.Name)
	_, err = f.WriteString(kubeconfig)
	if err != nil {
		f.Close()
		return "", errors.Wrapf(err, "failed to close kubeconfig file")
	}
	err = f.Close()
	if err != nil {
		return "", errors.Wrapf(err, "failed to close kubeconfig file")
	}

	log.V(4).Info("Path is", "path", path)
	return filePath, nil
}

func GetCustomResource(ctx context.Context, c ctrlClient.Client, kind string, apiVersion string, namespace string, name string) (*unstructured.Unstructured, error) {
	objectRef := corev1.ObjectReference{
		Kind:       kind,
		Namespace:  namespace,
		Name:       name,
		APIVersion: apiVersion,
	}
	object, err := external.Get(context.TODO(), c, &objectRef, namespace)
	if err != nil {
		return nil, nil
	}

	return object, nil
}

func GetClusterField(ctx context.Context, c ctrlClient.Client, cluster *clusterv1.Cluster, fields []string) (string, error) {
	log := ctrl.LoggerFrom(ctx)

	object, err := GetCustomResource(ctx, c, cluster.Kind, cluster.APIVersion, cluster.Namespace, cluster.Name)
	if err != nil {
		return "", err
	}
	objectMap := object.UnstructuredContent()
	field, found, err := unstructured.NestedString(objectMap, fields...)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get cluster name from cluster object")
	}
	if !found {
		return "", errors.New("failed to get cluster name from cluster object")
	}
	log.V(2).Info("Resolved cluster field to", "field", fields, "value", field)

	return field, nil
}
