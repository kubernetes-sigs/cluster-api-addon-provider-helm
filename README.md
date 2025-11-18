<a href="https://cluster-api.sigs.k8s.io"><img alt="capi" src="./logos/kubernetes-cluster-logos_final-02.svg" width="160x" /></a>
<p>
<a href="https://godoc.org/sigs.k8s.io/cluster-api"><img src="https://godoc.org/sigs.k8s.io/cluster-api?status.svg"></a>
<!-- join kubernetes slack channel for cluster-api -->
<a href="http://slack.k8s.io/">
<img src="https://img.shields.io/badge/join%20slack-%23cluster--api-brightgreen"></a>
</p>

# Cluster API Add-on Provider for Helm!

### 👋 Welcome to CAAPH! Here are some links to help you get started:

- [Quick start guide](./docs/quick-start.md)
- [Development guide](./docs/development.md)
- [Project design outline](https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/proposals/20220712-cluster-api-addon-orchestration.md)

## ✨ What is Cluster API Add-on Provider for Helm?

Cluster API Add-on Provider for Helm extends Cluster API by managing the installation, configuration, upgrade, and deletion of cluster add-ons using Helm charts. CAAPH is based on the [Cluster API Add-on Orchestration Proposal](https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/proposals/20220712-cluster-api-addon-orchestration.md), a larger effort to bring orchestration for add-ons to CAPI by using existing package management tools. 

This project is a concrete implementation of a ClusterAddonProvider, a pluggable component to be deployed on the Management Cluster. An add-on provider component acts as a broker between Cluster API and a package management tool. CAAPH is the first concrete implementation of a ClusterAddonProvider and can serve as a reference implementation.

The aims of the ClusterAddonProvider project are as follows:

#### Goals

- Design a solution for orchestrating Cluster add-ons.
- Leverage the capabilities of existing package management tools, such as an add-on package repository, templating, creation, upgrade, and deletion.
- Make add-on management in Cluster API modular and pluggable.
- Make it clear for developers how to build a Cluster API Add-on Provider based on any package management tool.

#### Non-goals

- Implement a new, full-fledged package management tool in Cluster API.
- Provide a mechanism for altering, customizing, or dealing with single Kubernetes resources defining a Cluster add-on, i.e. Deployments, Services, ServiceAccounts. Cluster API should treat add-ons as opaque components and delegate all the operations impacting add-on internals to the package management tool.
- Expect users to use a specific package management tool.
- Implement a solution for installing add-ons on the management cluster itself.

## 🤗 Community, discussion, contribution, and support

Cluster API Add-on Provider for Helm is developed as a part of the [Cluster API project](https://github.com/kubernetes-sigs/cluster-api). As such, it will share the same communication channels and meeting as Cluster API.

This work is made possible due to the efforts of users, contributors, and maintainers. If you have questions or want to get the latest project news, you can connect with us in the following ways:

- Chat with us on the Kubernetes [Slack](http://slack.k8s.io/) in the [#cluster-api](https://kubernetes.slack.com/archives/C8TSNPY4T) channel
- Subscribe to the [SIG Cluster Lifecycle](https://groups.google.com/forum/#!forum/kubernetes-sig-cluster-lifecycle) Google Group for access to documents and calendars

Pull Requests and feedback on issues are very welcome!
See the [issue tracker](https://github.com/kubernetes-sigs/cluster-api-addon-provider-helm/issues) if you're unsure where to start, especially the [Good first issue](https://github.com/kubernetes-sigs/cluster-api-addon-provider-helm/issues?q=is%3Aopen+is%3Aissue+label%3A%22good+first+issue%22) and [Help wanted](https://github.com/kubernetes-sigs/cluster-api-addon-provider-helm/issues?q=is%3Aopen+is%3Aissue+label%3A%22help+wanted%22) tags, and
also feel free to reach out to discuss.

See also our [contributor guide](CONTRIBUTING.md) and the Kubernetes [community page](https://kubernetes.io/community/) for more details on how to get involved.

### Code of conduct

Participation in the Kubernetes community is governed by the [Kubernetes Code of Conduct](code-of-conduct.md).
