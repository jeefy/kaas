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
	"reflect"

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
	var update bool

	var kaasConfig honkv1.KaasConfig
	if err = r.Get(ctx, types.NamespacedName{Name: "config", Namespace: "kaas-system"}, &kaasConfig); err != nil {
		// We proceed as normal. The config just overrides the defaults.
		log.Info(err.Error())
	}

	var cluster honkv1.Cluster
	if err = r.Get(ctx, req.NamespacedName, &cluster); err != nil {
		// log.Error(err, "unable to fetch Cluster")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		//return ctrl.Result{}, client.IgnoreNotFound(err)
		return ctrl.Result{}, nil
	}

	cluster = cluster.SetConfig(&kaasConfig)

	cm := cluster.ConfigMap(req.Namespace)
	foundCM := &v1.ConfigMap{}
	err = r.Get(context.TODO(), types.NamespacedName{Name: cm.GetName(), Namespace: cm.GetNamespace()}, foundCM)
	if err != nil && errors.IsNotFound(err) {
		log.Info(fmt.Sprintf("Creating ConfigMap %s/%s\n", cm.GetNamespace(), cm.GetName()))
		err = r.Create(context.TODO(), cm)
		if err != nil {
			return ctrl.Result{}, err
		}
	} else if err != nil && errors.IsAlreadyExists(err) {
		return ctrl.Result{}, nil
	} else if err != nil {
		return ctrl.Result{}, err
	} else {
		if len(cm.Data) > 0 && len(foundCM.Data) > 0 && !reflect.DeepEqual(cm.Data, foundCM.Data) {
			log.Info(fmt.Sprintf("Updating ConfigMap object %s/%s", cm.Namespace, cm.Name))
			foundCM.Data = cm.Data
			err = r.Update(context.TODO(), foundCM)
			if err != nil {
				return ctrl.Result{}, err
			}
			update = true
		}
	}

	svc, err := cluster.Service()
	if err != nil {
		return ctrl.Result{}, err
	}
	foundSvc := &v1.Service{}
	err = r.Get(context.TODO(), types.NamespacedName{Name: svc.GetName(), Namespace: svc.GetNamespace()}, foundSvc)
	if err != nil && errors.IsNotFound(err) {
		log.Info(fmt.Sprintf("Creating Service %s/%s\n", cm.GetNamespace(), cm.GetName()))
		err = r.Create(context.TODO(), svc)
		if err != nil {
			return ctrl.Result{}, err
		}
	} else if err != nil && errors.IsAlreadyExists(err) {
		return ctrl.Result{}, nil
	} else if err != nil {
		return ctrl.Result{}, err
	}

	pod := cluster.Pod(req.Namespace)
	foundPod := &v1.Pod{}
	err = r.Get(context.TODO(), types.NamespacedName{Name: pod.GetName(), Namespace: pod.GetNamespace()}, foundPod)

	if err != nil && errors.IsNotFound(err) {
		log.Info(fmt.Sprintf("Creating Pod %s/%s\n", pod.GetNamespace(), pod.GetName()))
		err = r.Create(context.TODO(), pod)
		if err != nil && !errors.IsAlreadyExists(err) {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	} else if err != nil {
		return ctrl.Result{}, err
	} else {
		if !cluster.PodSpecEquals(foundPod) {
			update = true
		}

		if update {
			// Refresh pods
			err = r.Delete(context.TODO(), foundPod)
			if err != nil && errors.IsNotFound(err) {
				return ctrl.Result{}, nil
			}

			return ctrl.Result{}, err
		}
		if foundPod.Status.Phase == v1.PodRunning {
			if foundPod.Status.ContainerStatuses[0].Ready {
				config, err := ctrl.GetConfig()
				if err != nil {
					log.Info("Can't get config from ctrl")
					return ctrl.Result{}, err
				}
				if foundSvc != nil {
					files := make(map[string]string)
					files["root-config"] = "/root/.kube/config"
					files["default-config"] = "/tmp/kube/k8s-kind-user-default-conf"

					kubeconfigs, err := cluster.Kubeconfig(config, foundSvc, files)
					log.Info(fmt.Sprintf("Gathered %d Kubeconfigs", len(kubeconfigs)))
					if err != nil {
						log.Info("Can't get adminkubeconfig from cluster")
						return ctrl.Result{}, err
					}
					if len(kubeconfigs) > 0 {
						secret, err := cluster.Secret("kubeconfig", kubeconfigs)
						if err != nil {
							log.Info("Can't generate kubeconfig secrets")
							return ctrl.Result{}, err
						}
						foundSecret := v1.Secret{}
						createSecret := false
						err = r.Get(context.TODO(), types.NamespacedName{Name: secret.GetName(), Namespace: secret.GetNamespace()}, &foundSecret)
						if err == nil && !reflect.DeepEqual(foundSecret.StringData, secret.StringData) {
							err = r.Delete(context.TODO(), &foundSecret)
							createSecret = true
							if err != nil {
								log.Info("Can't delete old secrets")
								return ctrl.Result{}, err
							}
						}
						if err != nil && errors.IsNotFound(err) {
							createSecret = true
						}
						if createSecret {
							err = r.Create(context.TODO(), secret)
							if err != nil {
								return ctrl.Result{}, err
							}
						}
					}
				}
				if !cluster.Status.Ready {
					cluster.Status.Ready = true

					if len(svc.Status.LoadBalancer.Ingress) > 0 {
						cluster.Status.LoadBalancerIP = svc.Status.LoadBalancer.Ingress[0].IP
					}

					err = r.Update(context.TODO(), &cluster)
					if err != nil {
						return ctrl.Result{}, err
					}
				}
			}
		}
	}

	return ctrl.Result{}, nil
}

var (
	jobOwnerKey = ".metadata.controller"
	apiGVStr    = honkv1.GroupVersion.String()
)

// SetupWithManager sets up the controller manager :tada:
func (r *ClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Index the Cluster-Pods
	if err := mgr.GetFieldIndexer().IndexField(&v1.Pod{}, jobOwnerKey, func(rawObj runtime.Object) []string {
		pod := rawObj.(*v1.Pod)
		owner := metav1.GetControllerOf(pod)
		if owner == nil {
			return nil
		}

		if owner.APIVersion != apiGVStr || owner.Kind != "Cluster" {
			return nil
		}

		return []string{owner.Name}
	}); err != nil {
		return err
	}

	// Index the Cluster-ConfigMaps
	if err := mgr.GetFieldIndexer().IndexField(&v1.ConfigMap{}, jobOwnerKey, func(rawObj runtime.Object) []string {
		cm := rawObj.(*v1.ConfigMap)
		owner := metav1.GetControllerOf(cm)
		if owner == nil {
			return nil
		}

		if owner.APIVersion != apiGVStr || owner.Kind != "Cluster" {
			return nil
		}

		return []string{owner.Name}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&honkv1.Cluster{}).
		Owns(&v1.Service{}).
		Owns(&v1.Pod{}).
		Owns(&v1.ConfigMap{}).
		Complete(r)
}
