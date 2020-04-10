/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	honkv1 "github.com/jeefy/kaas/api/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClusterReconciler reconciles a Cluster object
type ClusterReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=honk.honk.ci,resources=clusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=honk.honk.ci,resources=clusters/status,verbs=get;update;patch

func (r *ClusterReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("cluster", req.NamespacedName)
	var err error

	var cluster honkv1.Cluster
	if err = r.Get(ctx, req.NamespacedName, &cluster); err != nil {
		log.Error(err, "unable to fetch Cluster")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	cm := generateClusterConfigmap(cluster, req.Namespace)
	foundCM := &v1.ConfigMap{}
	err = r.Get(context.TODO(), types.NamespacedName{Name: cm.GetName(), Namespace: cm.GetNamespace()}, foundCM)
	if err != nil && errors.IsNotFound(err) {
		log.Info(fmt.Sprintf("Creating ConfigMap %s/%s\n", cm.GetNamespace(), cm.GetName()))
		err = r.Create(context.TODO(), cm)
		if err != nil {
			log.Info(fmt.Sprintf("Error creating ConfigMap %s/%s - %v\n", cm.GetNamespace(), cm.GetName(), err.Error()))
			return ctrl.Result{}, err
		}
	} else if err != nil {
		log.Info(fmt.Sprintf("Error getting ConfigMap %s/%s - %v\n", cm.GetNamespace(), cm.GetName(), err.Error()))
		return ctrl.Result{}, err
	}

	pod := generateClusterPod(&cluster, cm)
	foundPod := &v1.Pod{}
	err = r.Get(context.TODO(), types.NamespacedName{Name: pod.GetName(), Namespace: pod.GetNamespace()}, foundPod)
	if err != nil && errors.IsNotFound(err) {
		log.Info(fmt.Sprintf("Creating Pod %s/%s\n", pod.GetNamespace(), pod.GetName()))
		err = r.Create(context.TODO(), pod)
		if err != nil {
			log.Info(fmt.Sprintf("Error creating Pod %s/%s - %v\n", pod.GetNamespace(), pod.GetName(), err.Error()))
			return ctrl.Result{}, err
		}
	} else if err != nil {
		log.Info(fmt.Sprintf("Error getting Pod %s/%s - %v\n", pod.GetNamespace(), pod.GetName(), err.Error()))
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *ClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&honkv1.Cluster{}).
		Complete(r)
}

func generateClusterConfigmap(cluster honkv1.Cluster, namespace string) *v1.ConfigMap {
	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.Name,
			Namespace: namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(&cluster, honkv1.SchemeBuilder.GroupVersion.WithKind("Cluster")),
			},
		},
	}

	cm.Data = make(map[string]string)

	cm.Data["kind-config.yaml"] = string(cluster.Spec.KindSpec)

	for key, data := range cluster.Spec.ClusterYAML {
		cm.Data[fmt.Sprintf("%d.yaml", key)] = data
	}

	return cm
}

func generateClusterPod(cluster *honkv1.Cluster, configMap *v1.ConfigMap) *v1.Pod {
	defaultMode := int32(0777)
	falseValue := false
	resourceList := v1.ResourceList{}
	resourceList[v1.ResourceCPU] = *cluster.Spec.CPU
	resourceList[v1.ResourceMemory] = *cluster.Spec.Memory
	command := "sleep 5 && tail /var/log/docker.log && gsutil cp -P gs://bentheelder-kind-ci-builds/latest/kind-linux-amd64 \"${PATH%%%%:*}/kind\" && apt update && apt install -y jq && kind create cluster --config=/honk/kind-config.yaml && sleep 5"
	for key := range configMap.Data {
		if key != "kind-config.yaml" {
			command += fmt.Sprintf(" && kubectl apply -f /honk/%s && sleep 5", key)
		}
	}
	//  && kubectl apply -f /honk/01.yaml && sleep 5 && kubectl apply -f /honk/02.yaml && sleep 5 && kubectl apply -f /honk/03.yaml"
	command += " && sleep infinity"

	return &v1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind: "pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.Name,
			Namespace: configMap.Namespace,
			Labels:    cluster.GetLabels(),
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(cluster, honkv1.SchemeBuilder.GroupVersion.WithKind("Cluster")),
			},
		},
		Spec: v1.PodSpec{
			//			RuntimeClassName: &runtimeClass,
			AutomountServiceAccountToken: &falseValue,
			EnableServiceLinks:           &falseValue,
			Containers: []v1.Container{
				{
					Name:  "kind",
					Image: "gcr.io/k8s-testimages/krte@sha256:6cae666d578e2ad87f25934efa7b0a907827cf2cd515067c49e6144954b9cb70",
					Command: []string{
						"wrapper.sh",
						"bash",
						"-c",
						command,
					},
					Env: []v1.EnvVar{
						{Name: "DOCKER_IN_DOCKER_ENABLED", Value: "true"},
					},
					VolumeMounts: []v1.VolumeMount{
						{
							Name:      "docker-root",
							MountPath: "/var/lib/docker",
						},
						{
							Name:      "modules",
							MountPath: "/lib/modules",
							ReadOnly:  true,
						},
						{
							Name:      "cgroup",
							MountPath: "/sys/fs/cgroup",
						},
						{
							Name:      "honk",
							MountPath: "/honk",
						},
					},
					Resources: v1.ResourceRequirements{
						Limits:   resourceList,
						Requests: resourceList,
					},
				},
			},
			Volumes: []v1.Volume{
				{
					Name: "docker-root",
					VolumeSource: v1.VolumeSource{
						EmptyDir: &v1.EmptyDirVolumeSource{},
					},
				},
				{
					Name: "modules",
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
							Path: "/lib/modules",
						},
					},
				},
				{
					Name: "cgroup",
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
							Path: "/sys/fs/cgroup",
						},
					},
				},
				{
					Name: "honk",
					VolumeSource: v1.VolumeSource{
						ConfigMap: &v1.ConfigMapVolumeSource{
							LocalObjectReference: v1.LocalObjectReference{
								Name: cluster.Name,
							},
							DefaultMode: &defaultMode,
						},
					},
				},
			},
		},
	}
}
