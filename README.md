# Reconciler.io ducks <!-- omit in toc -->

![CI](https://github.com/reconcilerio/ducks/workflows/CI/badge.svg?branch=main)
[![GoDoc](https://godoc.org/reconciler.io/ducks?status.svg)](https://godoc.org/reconciler.io/ducks)
[![Go Report Card](https://goreportcard.com/badge/reconciler.io/ducks)](https://goreportcard.com/report/reconciler.io/ducks)
[![codecov](https://codecov.io/gh/reconcilerio/ducks/main/graph/badge.svg)](https://codecov.io/gh/reconcilerio/ducks)

> If it looks like a duck, swims like a duck, and quacks like a duck, then it probably is a duck.

`ducks` is an experimental library for interacting with arbitrary resources in a Kubernetes cluster that share a common shape within the reconciler.io ecosystem. The [`reonciler.io/runtime`](https://reconciler.io/runtime) project has strong support for interacting with Duck typed resources as if they were traditional typed resources. The `ducks` project extends that support to facilitate discovery of resources compatible with a duck type and dynamically creates informers to track changes to referenced types.

- [Waddling and Quacking](#waddling-and-quacking)
  - [Define a new DuckType](#define-a-new-ducktype)
  - [Mark a resource as implementing the DuckType](#mark-a-resource-as-implementing-the-ducktype)
  - [Granting role based access](#granting-role-based-access)
  - [Consuming a DuckType](#consuming-a-ducktype)
- [Getting Started](#getting-started)
  - [Deploy a released build](#deploy-a-released-build)
  - [Build from source](#build-from-source)
    - [Undeploy controller](#undeploy-controller)
- [Community](#community)
  - [Code of Conduct](#code-of-conduct)
  - [Communication](#communication)
  - [Contributing](#contributing)
- [Acknowledgements](#acknowledgements)
- [License](#license)

## Waddling and Quacking

### Define a new DuckType

Duck types are defined in `ducks` by the `DuckType` resource. For example, the Service Binding for Kubernetes project defines a [Provisioned Serivce duck type](https://servicebinding.io/spec/core/1.1.0/#provisioned-service).

```yaml
apiVersion: duck.reconciler.io/v1
kind: DuckType
metadata:
  name: provisionedservices.duck.servicebinding.io
spec:
  group: duck.servicebinding.io
  plural: provisionedservices
  kind: ProvisionedService
```

### Mark a resource as implementing the DuckType

Resources implementing the duck type are marked. For example, the `ExternalSecret` resource from the [External Secrets Operator](https://external-secrets.io/) project implements the provisioned service duck type:

```yaml
apiVersion: duck.servicebinding.io/v1
kind: ProvisionedService
metadata:
  name: externalsecrets.external-secrets.io
spec:
  group: external-secrets.io
  version: v1beta1
  kind: ExternalSecret
```

### Granting role based access

The `ducks` manager will validate the marked API exists and creates `ClusterRole`s for clients to be able to view or edit marked resources.

```sh
kubectl get clusterrole --selector ducks.reconciler.io/type=provisionedservices.duck.servicebinding.io
```

```txt
NAME
reconcilerio-ducks-provisionedservices.duck.servicebinding.io-edit
reconcilerio-ducks-provisionedservices.duck.servicebinding.io-view
<snip>
```

Controllers can use a `ClusterRoleBinding` to grant access to all current and future known resources implementing the duck type.

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: my-ducks-provisionedservices-manager-rolebinding
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: reconcilerio-ducks-provisionedservices.duck.servicebinding.io-view
subjects:
- kind: ServiceAccount
  name: my-controller-manager
  namespace: my-system
```

### Consuming a DuckType

Inside the controller manager updates to duck typed resources can be tracked by subscribing to a broker watching all resource for the duck type.

```go
// typically in main.go
provisionedServiceDuckBroker, err := duckclient.NewBroker(mgr, schema.GroupKind{Group: "duck.servicebinding.io/v1", Kind: "ProvisionedService"})
if err != nil {
	setupLog.Error(err, "unable to create ProvisionedServiceBroker")
	os.Exit(1)
}
```

Inside a reconciler, the broker can be combined with a [tracker](https://github.com/reconcilerio/runtime?tab=readme-ov-file#tracker) to cause the reconciled resource to be reprocessed when a tracked duck is updated.

```go
&reconcilers.SyncReconciler[componentsv1alpha1.GenericComponent]{
    Setup: func(ctx context.Context, mgr manager.Manager, bldr *builder.TypedBuilder[reconcile.Request]) error {
        // watching all resources backing the ProvisionedService duck type
        bldr.WatchesRawSource(source.Channel(provisionedServiceDuckBroker.Subscribe(ctx), reconcilers.EnqueueTracked(ctx)))

        return nil
    },
    Sync: func() error {
        // strongly typed representation of the duck type's shape
        provisionedService := &componentsv1alpha1.ProvisionedService{
            // type metadata for resource implementing the duck type
            TypeMeta: metav1.TypeMeta{
                APIVersion: "external-secrets.io/v1beta1",
                Kind:       "ExternalSecret",
            },
            ObjectMeta: metav1.ObjectMeta{
                Namespace: "default",
                Name:      "my-externalservice"
            }
        }
        // the DuckType resource
        duckType := schema.GroupKind{
            Group: "duck.servicebinding.io",
            Kind:  "ProvisionedService",
        }

        // get the provisioned service and track it for changes
        if err := duckclient.TrackAndGet(ctx, client.ObjectKeyFromObject(provisionedService), provisionedService, duckType); err != nil {
            return err
		}

        //  do something with the provisionedService

		return nil
	}
    },
}
```

## Getting Started

You’ll need a Kubernetes cluster to run against. You can use [kind](https://kind.sigs.k8s.io) to get a local cluster for testing, or run against a remote cluster.


### Deploy a released build

The easiest way to get started is by deploying the [latest release](https://github.com/reconcilerio/ducks/releases). Alternatively, you can [build from source](#build-from-source).

### Build from source

1. Define where to publish images:

   ```sh
   export KO_DOCKER_REPO=<a-repository-you-can-write-to>
   ```

   For kind, a registry is not required (or run `make kind-deploy`):

   ```sh
   export KO_DOCKER_REPO=kind.local
   ```
	
1. Build and deploy the controller to the cluster:

   Note: The cluster must have the [cert-manager](https://cert-manager.io) deployed.  There is a `make deploy-cert-manager` target to deploy the cert-manager.

   ```sh
   make deploy
   ```

#### Undeploy controller
Undeploy the controller to the cluster:

```sh
make undeploy
```


## Community

### Code of Conduct

The reconciler.io projects follow the [Contributor Covenant Code of Conduct](./CODE_OF_CONDUCT.md). In short, be kind and treat others with respect.

### Communication

General discussion and questions about the project can occur either on the Kubernetes Slack [#reconcilerio](https://kubernetes.slack.com/archives/C07J5G9NDHR) channel, or in the project's [GitHub discussions](https://github.com/orgs/reconcilerio/discussions). Use the channel you find most comfortable.

### Contributing

The reconciler.io ducks project team welcomes contributions from the community. A contributor license agreement (CLA) is not required. You own full rights to your contribution and agree to license the work to the community under the Apache License v2.0, via a [Developer Certificate of Origin (DCO)](https://developercertificate.org). For more detailed information, refer to [CONTRIBUTING.md](CONTRIBUTING.md).

## Acknowledgements

This project was inspired by experiences working with duck typed resources in the [Service Binding for Kubernetes](https://servicebinding.io) project, and the [Knative](https://knative.dev) project.

## License

Apache License v2.0: see [LICENSE](./LICENSE) for details.
