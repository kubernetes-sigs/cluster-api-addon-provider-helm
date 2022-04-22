# Cluster API Visualizer Helm Chart

This Helm chart allows you to deploy the Cluster API Visualizer app to a Kubernetes cluster using Helm.

### Install using the Helm repo

A Helm repo for this project is maintained at 
```
https://raw.githubusercontent.com/Jont828/cluster-api-visualizer/main/helm/repo
``` 

To deploy using the repo, run the following script from the root directory:

```
./hack/deploy-repo-to-kind.sh <name-of-management-cluster>
```

### Install using the local Helm chart

A local copy of the chart is also available at `./helm/cluster-api-visualizer`. To deploy using this chart, run the following script from the root directory:

```
./hack/deploy-local-to-kind.sh <name-of-management-cluster>
```

### Uninstall

To uninstall, kill the script used to deploy the chart. Then, run the following commands.

```
helm list
helm delete <name-of-cluster-api-visualizer-release>
```

### Configurable values

| Configuration value | Default value | Description |
| --- | --- | --- |
| `namespace` | `"default"` | Namespace the app will run on. |
| `image.repository` | `"docker.io/jont828"` | Repository where the containerized app is hosted. |
| `image.name` | `"cluster-api-visualizer"` | Name of the containerized app |
| `image.tag` | `"0.0.7"` | Version of the app to run |
| `image.pullPolicy` | `"Always"` | Image pull policy for the deployment |
| `label.key` | `"app"` | Internal selector key for selecting components. Default values recommended. |
| `label.value` | `"capi-visualizer"` | Internal selector value for selecting components. Default values recommended. |