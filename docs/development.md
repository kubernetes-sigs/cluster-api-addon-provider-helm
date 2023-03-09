# Development

## Prerequisites

The prerequisites include:
- [Go](https://go.dev/dl/) 1.18
- [kustomize](https://kubectl.docs.kubernetes.io/installation/kustomize/)
- make
- [Docker](https://www.docker.com/)
- [Kind](https://kind.sigs.k8s.io/)
- [Kubectl](https://kubernetes.io/docs/tasks/tools/#kubectl)
- [Tilt](https://docs.tilt.dev/install.html) (for running with Tilt)

In addition, you also need to install the prerequisites for [Cluster API](https://cluster-api.sigs.k8s.io/developer/tilt.html) as well in order to run it alongside this controller.

## Rapid, iterative development with Tilt

Tilt is a tool that allows hot reloading for Kubernetes controllers. Cluster API includes its own Tiltfile that can be used to run CAAPH on a local Kind cluster. It's strongly recommended to use Tilt for development work.

#### 1. Clone the Cluster API and CAAPH repositories

Clone the Cluster API and CAAPH repositories into your Go src folder:

```bash
$ git clone git@github.com:kubernetes-sigs/cluster-api.git ${GOPATH}/src/cluster-api
$ git clone git@github.com:Jont828/cluster-api-addon-provider-helm.git ${GOPATH}/src/cluster-api-addon-provider-helm
```

Afterwards your folder structure should look like this:

```
src/
├── cluster-api
└── cluster-api-addon-provider-helm
```

#### 2. Set up Tilt settings in `src/cluster-api`

Refer to [this guide](https://cluster-api.sigs.k8s.io/developer/tilt.html) to set up Tilt for Cluster API.

In particular, for our purposes we only need to set up `tilt-settings.yaml` in Cluster API to enable CAAPH as a provider. Add the following fields to the lists in `tilt-settings.yaml`:

```yaml
provider_repos:
- "../cluster-api-addon-provider-helm"
enable_providers:
- helm 
```

#### 3. Set up automatic installation for calico. (Optional)

Calico is automatically installed when workload clusters are started.  

From `src/cluster-api-addon-provider-helm` run:

```bash
$ ./hack/setup-calico-autoinstallation.sh
```

See [Automatically install calico in workload clusters with Tilt](#automatically-install-calico-in-workload-clusters-with-tilt) for more details.

#### 4. Run Tilt

From `src/cluster-api` run:

```bash
$ make tilt-up
```

From within Tilt, you should be able to see the CAAPH controller running alongside the Cluster API controllers with the CRDs installed.

### Automatically install calico in workload clusters with Tilt

Automatically install [calico](https://docs.tigera.io/calico) in workload clusters using the [yaml manifest](https://github.com/kubernetes-sigs/cluster-api-addon-provider-helm/blob/main/config/samples/calico-cni.yaml) provided by CAAPH.  
The setup script(`setup-calico-autoinstallation.sh`) generates a tiltfile to incorporate the above yaml manifest as a Tilt resource.  

**Setup script (`setup-calico-autoinstallation.sh`) Summary:**  
- Generate the tiltfile to incorporate the above yaml manifest as a Tilt resource.
  - Refer to [Tiltfile API Reference](https://docs.tilt.dev/api.html) for functions used in the tiltfile.
- The tiltfile is output to the `src/cluster-api/tilt.d` directory.
  - Run `make tilt-up` ,files under the `src/cluster-api/tilt.d` are automatically started. (excluding subdirectories)
  

The yaml manifest (`src/cluster-api-addon-provider-helm/config/samples/calico-cni.yaml`) is changed, it is redeployed by Tilt.  

**For the target workload clusters**  
The yaml manifest (`src/cluster-api-addon-provider-helm/config/samples/calico-cni.yaml`) is for all workload clusters.  <br>

An example targeting workload clusters with a specific label.
```yaml
spec:
  clusterSelector:
    matchLabels:
      cni: calico 
```
