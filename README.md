# kaas
Kind (Kubernetes) as a Service

## What
Kubebuilder-based controller that takes a kind cluster [defined in YAML](./manifests/test-cluster.yaml) and spins it up within an existing Kubernetes cluster.

## Why
Honk (Expand on this later)

## How

Requirements:

A K8s cluster that supports service type `LoadBalancer`

For local development, refer to https://mauilion.dev/posts/kind-metallb/

*Fair warning*: Local development with kind on OSX is kinda busted. See: https://www.thehumblelab.com/kind-and-metallb-on-mac/ 

```
# Use two terminal windows

## Terminal 1:
kind create cluster --config=manifests/kind-config.yaml
kubectl apply -f https://raw.githubusercontent.com/metallb/metallb/v0.9.3/manifests/namespace.yaml
kubectl apply -f https://raw.githubusercontent.com/metallb/metallb/v0.9.3/manifests/metallb.yaml
kubectl create secret generic -n metallb-system memberlist --from-literal=secretkey="$(openssl rand -base64 128)"
kubectl apply -f manifests/metallb.yaml

## Terminal 2:
make install && make run

## Terminal 1:
kubectl apply -f manifests/cluster-test.yaml
kubectl get pods -w

# Once the pod is ready

kubectl get secrets

# The cluster secret contains both an admin Kubeconfig (root-config) as well as a Kubeconfig for system:serviceaccount:default:default (default-config)
# Note: The default account does not have any RBAC

# Let's get access to the test cluster!
kubectl get secret test-cluster-kubeconfig -o json | jq '.["data"]["root-config"]' | tr -d '"' | base64 -d > /tmp/test-cluster-kubeconfig
export KUBECONFIG=/tmp/test-cluster-kubeconfig

# From here you should have access to your cluster within a cluster
```
