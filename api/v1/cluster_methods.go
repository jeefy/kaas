package v1

import (
	"fmt"
	"log"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PodSpecEquals accepts a pod and returns a bool whether the
// podSpec is the same as a generated podSpec.
// Since we have no way to DeepEquals podSpecs, we have to
// handle this ourselves. God dammit.
func (c Cluster) PodSpecEquals(foundPod *corev1.Pod) bool {
	pod := c.Pod(foundPod.Namespace)

	// Check that commands haven't changed (due to clusterYAML changing)
	if !reflect.DeepEqual(pod.Spec.Containers[0].Command, foundPod.Spec.Containers[0].Command) {
		log.Print("Container commands not equal")
		log.Printf("%v", pod.Spec.Containers[0].Command)
		log.Printf("%v", foundPod.Spec.Containers[0].Command)
		return false
	}

	// Check that the CPU and Memory haven't changed
	if !pod.Spec.Containers[0].Resources.Limits.Cpu().Equal(*foundPod.Spec.Containers[0].Resources.Limits.Cpu()) {
		log.Printf("CPU not equal: `%v` != `%v`", pod.Spec.Containers[0].Resources.Limits.Cpu(), foundPod.Spec.Containers[0].Resources.Limits.Cpu())
		return false
	}
	if !pod.Spec.Containers[0].Resources.Limits.Memory().Equal(*foundPod.Spec.Containers[0].Resources.Limits.Memory()) {
		log.Printf("Memory not equal: `%v` != `%v`", pod.Spec.Containers[0].Resources.Limits.Memory(), foundPod.Spec.Containers[0].Resources.Limits.Memory())
		return false
	}

	return true
}

// ConfigMap generates a ConfigMap based on the Cluster's Spec
func (c Cluster) ConfigMap(namespace string) *corev1.ConfigMap {
	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      c.Name,
			Namespace: namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(&c, SchemeBuilder.GroupVersion.WithKind("Cluster")),
			},
		},
	}

	cm.Data = make(map[string]string)

	cm.Data["kind-config.yaml"] = string(c.Spec.KindSpec)

	for key, data := range c.Spec.ClusterYAML {
		cm.Data[fmt.Sprintf("%d.yaml", key)] = data
	}

	return cm
}

// Pod generates a Pod based on the Cluster Spec
func (c Cluster) Pod(namespace string) *corev1.Pod {
	defaultMode := int32(0777)
	falseValue := false
	resourceList := v1.ResourceList{}
	resourceList[v1.ResourceCPU] = *c.Spec.CPU
	resourceList[v1.ResourceMemory] = *c.Spec.Memory
	command := "sleep 5 && tail /var/log/docker.log && gsutil cp -P gs://bentheelder-kind-ci-builds/latest/kind-linux-amd64 \"${PATH%%%%:*}/kind\" && apt update && apt install -y jq && kind create cluster --config=/honk/kind-config.yaml && sleep 5"
	for key := range c.Spec.ClusterYAML {
		command += fmt.Sprintf(" && kubectl apply -f /honk/%d.yaml && sleep 5", key)
	}
	//  && kubectl apply -f /honk/01.yaml && sleep 5 && kubectl apply -f /honk/02.yaml && sleep 5 && kubectl apply -f /honk/03.yaml"
	command += " && sleep infinity"

	return &v1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind: "pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      c.Name,
			Namespace: namespace,
			Labels:    c.GetLabels(),
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(&c, SchemeBuilder.GroupVersion.WithKind("Cluster")),
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
								Name: c.Name,
							},
							DefaultMode: &defaultMode,
						},
					},
				},
			},
		},
	}
}
