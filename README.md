# Cluster API Addon Provider for Helm

Cluster API manages the lifecycle of Kubernetes Clusters while users rely on their own package management tool of choice for Kubernetes application management such as Helm or carvel-kapp. 

This project aims to extend the functionality of Cluster API by providing an API for managing the installation, configuration, upgrade, and deletion of cluster addons using Helm charts. Cluster API Addon Provider for Helm is implemented as a Kubernetes controller watching the custom resource `HelmChartProxy` which contains proxy configurations to be used by Helm charts for specific addons. 

In particular, this project is a prototype following the [Cluster API Addon Orchestration Proposal](https://docs.google.com/document/d/1TdbfXC2_Hhg0mH7-7hXcT1Gg8h6oXKrKbnJqbpFFvjw/edit?usp=sharing). As such, it is a work in progress at this stage and may change at any time.

### Definitions

**Package management tool**: a tool ultimately responsible for installing Kubernetes applications such as Helm or carvel-kapp.

**Add-on**: an application that extends the functionality of Kubernetes.

**Cluster add-on**: an add-on that is essential to the proper function of the Cluster like networking (CNI) or integration with the underlying infrastructure provider (CPI, CSI). Additionally, in some cases the lifecycle of Cluster add-on is strictly linked to the Cluster lifecycle (e.g. CPI must be upgraded in lock step with the Cluster upgrade).

### Goals

- To design a solution for orchestrating Cluster add-ons with a lifecycle strictly linked to the Cluster lifecycle managed by CAPI.
- To identify a set of core requirements that should be addressed for plugging-in a package management tool into the above orchestration system.
- To allow implementation of such a solution in the shortest term possible/with minimal or no changes to Cluster API.
- To provide a path forward for iteratively enhancing user experience for add-on orchestration in CAPI, with focus on Cluster creation, operatibility on existing Cluster and observability about add-ons state.

### Non-goals

- To implement a full fledged package management tool in Cluster API; there are already several awesome package management tools in the ecosystem, and CAPI should not reinvent the wheel. 
- To provide mechanism for altering, customizing or dealing with single Kubernetes resources defining a Cluster add-on (Deployments, Services, ServiceAccounts etc); CAPI should treat add-ons as opaque components and delegate all the operations impacting add-on internals to the package management tool.
- To manage compatibility matrix between add-ons version and the Kubernetes version, or to manage the compatibility matrix between different add-ons versions; those information should be embedded in the package definition; also, in most cases, version compatibility matrix are affected by external factors like software supply chain processes in place in each organization/product, and CAPI should not be responsible for this.
- To manage dependencies or installation order between Cluster add-ons; this is a responsibility of the package management tool.

### Future work
- To implement a first integration with a package management tool; the problem space is complex, and it is required to reach community agreement on the overall design before committing to next steps; nevertheless, we are using Helm as a target for validating the proposed design; we believe that the community will step up as usual and quickly fill the gap to get a first concrete implementation of this proposal by leveraging on the preliminary work in this document.
- Improve user experience, operatibility, observability for add-ons orchestration in CAPI.
- To eventually reconsider the future of ClusterResourceSet experimental features. 

