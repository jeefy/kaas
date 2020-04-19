# kaas
Kind as a Service

## What
Kubebuilder-based controller that takes a kind cluster [defined in YAML](./kind-cluster.yaml) and spins it up within an existing Kubernetes cluster.

## Why
Honk

## How
```
kubectl apply -f https://raw.githubusercontent.com/metallb/metallb/v0.9.3/manifests/namespace.yaml
kubectl apply -f https://raw.githubusercontent.com/metallb/metallb/v0.9.3/manifests/metallb.yaml
kubectl create secret generic -n metallb-system memberlist --from-literal=secretkey="$(openssl rand -base64 128)"

make install && make run

kubectl apply -f manifests/metallb.yaml
kubectl apply -f manifests/kind-config.yaml
```
