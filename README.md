# kaas
Kind (Kubernetes) as a Service

## What
Kubebuilder-based controller that takes a kind cluster [defined in YAML](./manifests/test-cluster.yaml) and spins it up within an existing Kubernetes cluster.

This is purposefully made simple, and is not currently aimed at providing a secure or robust solution.

It takes a cluster and lets you spin up tiny clusters inside of it. Fin.

## Why
Honk (Expand on this later)

## How

```
# This assumes you have a cluster already set in your `Kubeconfig`
# If not, `kind create cluster` should get you going
make install && make run

# In a new window
kubectl apply -f manifests/cluster-test.yaml
kubectl get pods -w

# Once the pod is ready, the cluster is up! Let's get access to the test cluster!
# Note: This currently only works (seamlessly) if you have metallb or something up that supports ServiceType: LoadBalancer

# The cluster secret contains both an admin Kubeconfig (root-config) as well as a Kubeconfig for system:serviceaccount:default:default (default-config)
# Note: The default account does not have any RBAC

kubectl get secret test-cluster-kubeconfig -o json | jq '.["data"]["root-config"]' | tr -d '"' | base64 -d > /tmp/test-cluster-kubeconfig
export KUBECONFIG=/tmp/test-cluster-kubeconfig

# From here you should have access to your cluster within a cluster
```

## Config

You can specify a global [config](/manifests/kaas-config.yaml) for kaas. 

The global config options are slim, but can be found in the KaasConfig object [here](/api/v1/cluster_types.go)

For individual clusters, see the [manifests/kind-cluster.yaml](/manifests/kind-cluster.yaml) and [manifests/k3s-cluster.yaml](/manifests/k3s-cluster.yaml) for basic examples. For detailed config options, see the ClusterSpec object [here](/api/v1/cluster_types.go)


## Future State
- CLI interface for better UX (WIP)
- Kubeconfig generation for more than ServiceType LoadBalancer
- Additional cluster types
  - OpenShift (likely crc)
  - MiniKube
- Actual release process
- Format result of `kubectl get clusters` better