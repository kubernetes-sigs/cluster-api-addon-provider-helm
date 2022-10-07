# Cluster API Addon Provider for Helm (CAAPH)

Cluster API Add-on Provider for Helm is a Cluster API provider that extends the functionality of Cluster API by providing a solution for managing the installation, configuration, upgrade, and deletion of cluster add-ons using Helm charts. It is implemented using Kubernetes controllers watching with two custom resources: `HelmChartProxy` and `HelmReleaseProxy`.

In particular, this project is a prototype following the [Cluster API Addon Orchestration Proposal](https://github.com/kubernetes-sigs/cluster-api/pull/6905). As such, it is a work in progress at this stage and may change at any time.

### Goals

- To design a solution for orchestrating Cluster add-ons.
- To leverage existing package management tools such as Helm for all the foundational capabilities of add-on management, i.e. add-on packages/repository, templating/configuration, add-on creation, upgrade and deletion etc. 
- To make add-on management in Cluster API modular and pluggable, and to make it simple for developers to build a Cluster API Add-on Provider based on any package management tool, just like with infrastructure and bootstrap providers.

### Non goals

- To implement a full fledged package management tool in Cluster API; there are already several awesome package management tools in the ecosystem, and CAPI should not reinvent the wheel. 
- To provide a mechanism for altering, customizing, or dealing with single Kubernetes resources defining a Cluster add-on, i.e. Deployments, Services, ServiceAccounts. Cluster API should treat add-ons as opaque components and delegate all the operations impacting add-on internals to the package management tool.
- To expect users to use a specific package management tool.
- To implement a solution for installing add-ons on the management cluster itself.

### Quick start

See the [quick start guide](docs/quick-start.md) for instructions on how to get started with CAAPH.