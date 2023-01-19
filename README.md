<a href="https://cluster-api.sigs.k8s.io"><img alt="capi" src="./logos/kubernetes-cluster-logos_final-02.svg" width="160x" /></a>
<p>
<a href="https://godoc.org/sigs.k8s.io/cluster-api"><img src="https://godoc.org/sigs.k8s.io/cluster-api?status.svg"></a>
<!-- join kubernetes slack channel for cluster-api -->
<a href="http://slack.k8s.io/">
<img src="https://img.shields.io/badge/join%20slack-%23cluster--api-brightgreen"></a>
</p>

# Cluster API Add-on Provider for Helm

### 👋 Welcome to our project! Here are some links to help you get started:

- [Quick start guide](./docs/quick-start.md)
- [Development guide](./docs/development.md)
- [Project design outline](https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/proposals/20220712-cluster-api-addon-orchestration.md)

## ✨ What is Cluster API Add-on Provider for Helm?

Cluster API Add-on Provider for Helm is a Cluster API provider that extends the functionality of Cluster API by providing a solution for managing the installation, configuration, upgrade, and deletion of Cluster add-ons using Helm charts. This project is based on the [Cluster API Add-on Orchestration Proposal](https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/proposals/20220712-cluster-api-addon-orchestration.md)] and is part of an larger, ongoing effort to bring add-ons orchestration to Cluster API using existing package management tools. 

This is a concrete implementation of a ClusterAddonProvider, a new pluggable component to be deployed on the Management Cluster. A ClusterAddonProvider must be implemented using a specified package management tool and will act as an intermediary/proxy between Cluster API and the chosen package management solution. As such, Cluster API Add-on Provider for Helm is the first concrete implementation and can serve as a reference implementation for ClusterAddonProviders using other package managers.

The aims of the ClusterAddonProvider project are as follows:

#### Goals

- To design a solution for orchestrating Cluster add-ons.
- To leverage existing package management tools such as Helm for all the foundational capabilities of add-on management, i.e. add-on packages/repository, templating/configuration, add-on creation, upgrade and deletion etc. 
- To make add-on management in Cluster API modular and pluggable, and to make it simple for developers to build a Cluster API Add-on Provider based on any package management tool, just like with infrastructure and bootstrap providers.

#### Non goals

- To implement a full fledged package management tool in Cluster API; there are already several awesome package management tools in the ecosystem, and CAPI should not reinvent the wheel. 
- To provide a mechanism for altering, customizing, or dealing with single Kubernetes resources defining a Cluster add-on, i.e. Deployments, Services, ServiceAccounts. Cluster API should treat add-ons as opaque components and delegate all the operations impacting add-on internals to the package management tool.
- To expect users to use a specific package management tool.
- To implement a solution for installing add-ons on the management cluster itself.

## 🤗 Community, discussion, contribution, and support

Cluster API Add-on Provider for Helm is developed as a part of the [Cluster API project](https://github.com/kubernetes-sigs/cluster-api). As such, it will share the same communication channels and meeting as Cluster API.

This work is made possible due to the efforts of users, contributors, and maintainers. If you have questions or want to get the latest project news, you can connect with us in the following ways:

- Chat with us on the Kubernetes [Slack](http://slack.k8s.io/) in the [#cluster-api][#cluster-api slack] channel
- Subscribe to the [SIG Cluster Lifecycle](https://groups.google.com/forum/#!forum/kubernetes-sig-cluster-lifecycle) Google Group for access to documents and calendars
- Join our Cluster API working group sessions
    - Weekly on Wednesdays @ 10:00 PT on [Zoom][zoomMeeting]
    - Previous meetings: \[ [notes][notes] | [recordings][recordings] \]

Pull Requests and feedback on issues are very welcome!
See the [issue tracker] if you're unsure where to start, especially the [Good first issue] and [Help wanted] tags, and
also feel free to reach out to discuss.

See also our [contributor guide](CONTRIBUTING.md) and the Kubernetes [community page] for more details on how to get involved.

### Code of conduct

Participation in the Kubernetes community is governed by the [Kubernetes Code of Conduct](code-of-conduct.md).

