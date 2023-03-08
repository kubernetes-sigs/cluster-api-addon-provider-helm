# Quick Start

This quick start guide outlines the steps to install Cluster API Add-on Provider Helm to an existing Cluster API management cluster running on kind.

### 1. Install prerequisites

The prerequisites include:
- [Go](https://go.dev/dl/) 1.18
- [kustomize](https://kubectl.docs.kubernetes.io/installation/kustomize/)
- make
- [Docker](https://www.docker.com/)
- [Kind](https://kind.sigs.k8s.io/)
- [Kubectl](https://kubernetes.io/docs/tasks/tools/#kubectl)

You may need to install additional prerequisites to create a Cluster API management cluster.

### 2. Create a management cluster and workload cluster

Create a management cluster using kind and then create a workload cluster running on the management cluster. Alternatively, you can use an existing management cluster with your workload clusters as well.

### 3. Clone the CAAPH repository

Run the following command to clone the CAAPH repository into your Go src folder:

```bash
$ git clone git@github.com:kubernetes-sigs/cluster-api-addon-provider-helm.git ${GOPATH}/src/cluster-api-addon-provider-helm
```

### 4. Install CAAPH to the management cluster

From `src/cluster-api-addon-provider-helm` install the CRDs by running:

```bash
$ make install
```

Then, build the controller and push it to a container registry:

```bash
$ make docker-build docker-push REGISTRY=<my-registry>
```

Finally, deploy the controller and webhook to the management cluster:

```bash
$ make deploy REGISTRY=<my-registry>
```

### 5. Example: install `nginx-ingress` to the workload cluster

Add the following label to the workload cluster:

```yaml
nginxIngressChart: enabled
```

Then, from `src/cluster-api-addon-provider-helm` run:

```bash
$ kubectl apply -f config/samples/nginx-ingress.yaml
```

This will create a HelmChartProxy installing the `nginx-ingress` chart to the workload cluster. HelmChartProxy is the CRD that users interact with to

1. Specify a Helm chart to install
2. Determine which workload clusters to install the chart on
3. Configure the Helm chart values based on the workload cluster definition.

Here is a breakdown of the HelmChartProxy spec file we just applied:

```yaml
apiVersion: addons.cluster.x-k8s.io/v1alpha1
kind: HelmChartProxy
metadata:
  name: nginx-ingress
spec:
  clusterSelector:
    matchLabels:
      nginxIngressChart: enabled
  repoURL: https://helm.nginx.com/stable
  chartName: nginx-ingress
  valuesTemplate: |
    controller:
      name: "{{ .ControlPlane.metadata.name }}-nginx"
      nginxStatus:
        allowCidrs: 127.0.0.1,::1,{{ index .Cluster.spec.clusterNetwork.pods.cidrBlocks 0 }}
```

We use the `clusterSelector` to select the workload cluster to install the chart to. In this case, we install the chart to any workload cluster with the label `nginxIngressChart: enabled` found in the same namespace as `HelmChartProxy` resource. To provide specific namespace to install the chart to set `spec.namespace` field.

The `repoURL` and `chartName` are used to specify the chart to install. The `valuesTemplate` is used to specify the values to use when installing the chart. It supports Go templating, and here we set `controller.name` to the name of the selected cluster + `-nginx`. We also set `controller.nginxStatus.allowCidrs` to include the first entry in the workload cluster's pod CIDR blocks.

### 6. Verify that the chart was installed

Run the following command to verify that the HelmChartProxy is ready. The output should be similar to the following

```bash
$ kubectl get helmchartproxies
NAME            READY   REASON
nginx-ingress   True
```

The status of `nginx-ingress` should also look similar to the following. The `Ready` conditions signify that the chart was installed successfully, and the list of `matchingClusters` indicates which workload Clusters were selected by the `clusterSelector`, in this case it happens to be named `default-23995`.

```yaml
status:
conditions:
- lastTransitionTime: "2022-10-07T22:18:12Z"
  status: "True"
  type: Ready
- lastTransitionTime: "2022-10-07T22:18:12Z"
  status: "True"
  type: HelmReleaseProxiesReady
- lastTransitionTime: "2022-10-07T22:16:36Z"
  status: "True"
  type: HelmReleaseProxySpecsUpToDate
matchingClusters:
- apiVersion: cluster.x-k8s.io/v1beta1
  kind: Cluster
  name: default-23995
  namespace: default
```

Additionally, there is a second CRD called HelmReleaseProxy. While a HelmChartProxy is used to specify which Clusters to install a chart to, a single HelmReleaseProxy maintains an inventory of Helm releases managed by CAAPH.

Run the following command to verify that the HelmReleaseProxy is ready which should produce an output similar to the following:

```bash
$ kubectl get helmreleaseproxies
NAME                                CLUSTER         READY   REASON   STATUS     REVISION   NAMESPACE
nginx-ingress-default-23995-828jp   default-23995   True             deployed   1
```

The spec of the HelmReleaseProxy should look as follows:

```yaml
spec:
  chartName: nginx-ingress
  clusterRef:
    apiVersion: cluster.x-k8s.io/v1beta1
    kind: Cluster
    name: default-23995
    namespace: default
  namespace: default
  releaseName: nginx-ingress-1665181073
  repoURL: https://helm.nginx.com/stable
  values: |
    controller:
      name: "default-23995-nginx"
      nginxStatus:
        allowCidrs: 127.0.0.1,::1,192.168.0.0/16
```

Notice that a release name is generated for us, and the Go template we specified in `valuesTemplate` has been replaced with the actual values from the Cluster definition.

### 7. Uninstall `nginx-ingress` from the workload cluster

Remove the label `nginxIngressChart: enabled` from the workload cluster. On the next reconciliation, the HelmChartProxy will notice that the workload cluster no longer matches the `clusterSelector` and will delete the HelmReleaseProxy associated with the Cluster and uninstall the chart.

When you run the following command, you should see that the HelmReleaseProxy no longer exists.

```bash
$ kubectl get helmreleaseproxies
No resources found in default namespace.
```

Additionally, the HelmChartProxy status will show that no Clusters are matching in the `matchingClusters` list.

```yaml
status:
  conditions:
  - lastTransitionTime: "2022-10-07T23:35:09Z"
    status: "True"
    type: Ready
  - lastTransitionTime: "2022-10-07T23:35:09Z"
    status: "True"
    type: HelmReleaseProxiesReady
  - lastTransitionTime: "2022-10-07T23:31:49Z"
    status: "True"
    type: HelmReleaseProxySpecsUpToDate
  matchingClusters: []
```

### 8. Uninstall CAAPH

To uninstall CAAPH, run the following command from `src/cluster-api-addon-provider-helm`:

```bash
$ make undeploy
```