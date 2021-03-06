package v1

import (
	"bytes"
	"fmt"
	"log"
	"math/rand"
	"reflect"
	"strings"

	yaml "gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"

	"sigs.k8s.io/kind/pkg/apis/config/v1alpha4"
)

// SetConfig stores the KaasConfig object internally as part of the cluster
func (c Cluster) SetConfig(config *KaasConfig) Cluster {
	c.KaasConfig = config
	return c
}

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

	if c.Spec.ClusterType == KindCluster {
		kindConfig, err := c.KindConfig()
		if err != nil {
			log.Printf("Error getting KindConfig: %s", err.Error())
		}

		cm.Data["kind-config.yaml"] = kindConfig
	}

	for key, data := range c.Spec.ClusterYAML {
		cm.Data[fmt.Sprintf("%d.yaml", key)] = data
	}

	return cm
}

// KindConfig generates a valid KindConfig, makes updates, then returns a string
func (c Cluster) KindConfig() (string, error) {
	kindConfig := &v1alpha4.Cluster{}

	err := yaml.Unmarshal([]byte(c.Spec.ClusterSpec), kindConfig)
	if err != nil {
		return "", fmt.Errorf("error unmarshalling kindconfig: %s", err.Error())
	}

	kindConfig.Networking.APIServerPort = 6443
	kindConfig.Networking.APIServerAddress = "0.0.0.0"

	data, err := yaml.Marshal(kindConfig)
	if err != nil {
		return "", fmt.Errorf("error marshalling kindconfig: %s", err.Error())
	}

	return string(data), nil
}

// Pod generates a Pod based on the Cluster Spec
func (c Cluster) Pod(namespace string) *corev1.Pod {
	command := "sleep 5 && curl -sSLo /root/add_sa.sh https://gist.githubusercontent.com/jeefy/81fb5bc9b95898c1492d796a8a27ab10/raw/374f0cf09a6a6eceb5ae982bbd5df39dab7804e5/kubernetes_add_service_account_kubeconfig.sh && chmod +x /root/add_sa.sh && apt update && apt install -y jq && mkdir -p /root/.kube/ && "
	defaultMode := int32(0777)
	falseValue := false
	resourceList := v1.ResourceList{}
	resourceList[v1.ResourceCPU] = *c.Spec.CPU
	resourceList[v1.ResourceMemory] = *c.Spec.Memory
	image := c.Spec.Image
	switch c.Spec.ClusterType {
	case KindCluster:
		if c.Spec.Image == "" {
			image = "kindest/node:v1.18.0"
		}
		command += fmt.Sprintf("curl -sSLo \"${PATH%%%%:*}/kind\" https://storage.googleapis.com/bentheelder-kind-ci-builds/latest/kind-linux-amd64 && chmod +x \"${PATH%%%%:*}/kind\" && kind create cluster --image %s --config=/honk/kind-config.yaml && ", image)
	case K3sCluster:
		if c.Spec.Image == "" {
			image = "rancher/k3s:v1.18.2-rc1-k3s1"
		}
		command += fmt.Sprintf("curl -s https://raw.githubusercontent.com/rancher/k3d/master/install.sh | bash && k3d create --image %s --api-port 6443 && sleep 30 && cp -ruf $(k3d get-kubeconfig) /root/.kube/config && ", image)
	}

	command += "sleep 5 && /root/add_sa.sh kind-user default && sleep 5 && "

	for key := range c.Spec.ClusterYAML {
		command += fmt.Sprintf("kubectl apply -f /honk/%d.yaml && sleep 5 && ", key)
	}
	//  && kubectl apply -f /honk/01.yaml && sleep 5 && kubectl apply -f /honk/02.yaml && sleep 5 && kubectl apply -f /honk/03.yaml"
	command += " kubectl create ns honk && sleep infinity"

	trueValue := true
	securityContext := v1.SecurityContext{
		Privileged: &trueValue,
	}

	labels := c.GetLabels()
	if len(labels) == 0 {
		labels = make(map[string]string)
	}
	labels["cluster"] = c.Name

	return &v1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind: "pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      c.Name,
			Namespace: namespace,
			Labels:    labels,
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
					Name:            "kind",
					Image:           "gcr.io/k8s-testimages/krte@sha256:6cae666d578e2ad87f25934efa7b0a907827cf2cd515067c49e6144954b9cb70",
					SecurityContext: &securityContext,
					Command: []string{
						"wrapper.sh",
						"bash",
						"-c",
						command,
					},
					ReadinessProbe: &v1.Probe{
						InitialDelaySeconds: 120,
						TimeoutSeconds:      5,
						Handler: v1.Handler{
							Exec: &v1.ExecAction{
								Command: []string{
									"kubectl",
									"get",
									"ns",
									"honk",
								},
							},
						},
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

// Service generates a Service to point to the Kubernetes Pod
func (c Cluster) Service() (*v1.Service, error) {
	selector := make(map[string]string)
	selector["cluster"] = c.Name

	// Set up the defaults
	loadBalancerType := v1.ServiceTypeNodePort
	log.Printf("Default LB Type: %s", c.KaasConfig.DefaultServiceType)
	if c.KaasConfig.DefaultServiceType != "" {
		loadBalancerType = c.KaasConfig.DefaultServiceType
	}

	servicePort := v1.ServicePort{
		Name: "kube-apiserver",
		Port: 6443,
		TargetPort: intstr.IntOrString{
			IntVal: 6443,
		},
	}
	log.Printf("Default service port: %v", c.KaasConfig.DefaultPort)
	if c.KaasConfig.DefaultPort.Port != 0 {
		servicePort = c.KaasConfig.DefaultPort
	}
	return &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      c.Name,
			Namespace: c.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(&c, SchemeBuilder.GroupVersion.WithKind("Cluster")),
			},
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				servicePort,
			},
			Selector: selector,
			Type:     loadBalancerType,
		},
	}, nil
}

// Secret stores a Secret owned by the Cluster
func (c Cluster) Secret(name string, data map[string]string) (*v1.Secret, error) {
	selector := make(map[string]string)
	selector["cluster"] = c.Name
	return &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", c.Name, name),
			Namespace: c.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(&c, SchemeBuilder.GroupVersion.WithKind("Cluster")),
			},
		},
		Type:       v1.SecretTypeOpaque,
		StringData: data,
	}, nil
}

// Kubeconfig gets the cluster kubeconfigs
// It makes several assumptions depending on the ServiceType
// If those assumptions are incorrect.... WELP. It'll just die.
func (c Cluster) Kubeconfig(config *rest.Config, svc *v1.Service, configs map[string]string) (kubeconfigs map[string]string, err error) {
	kubeconfigs = make(map[string]string)
	var port int32
	ip := "0.0.0.0"
	rand.Seed(112358)

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Printf("Unable to create clientset: %s", err.Error())
		return kubeconfigs, err
	}

	// Load Balancers are SO EASY
	if svc.Spec.Type == v1.ServiceTypeLoadBalancer && len(svc.Status.LoadBalancer.Ingress) > 0 {
		log.Printf("Swapping out IP for loadBalancer IP: %s", svc.Status.LoadBalancer.Ingress[0].IP)
		ip = svc.Status.LoadBalancer.Ingress[0].IP
		port = svc.Spec.Ports[0].Port
	}

	// Otherwise easiest is a NodePort
	if svc.Spec.Type == v1.ServiceTypeNodePort {
		// Nodes can have multiple IPs, let's handle that
		internalAddress := ""
		externalAddress := ""
		nodes, err := clientset.CoreV1().Nodes().List(metav1.ListOptions{})
		if err != nil {
			return nil, err
		}

		// Let's select a random node since it'll all go to the same place anyway
		node := nodes.Items[rand.Intn(len(nodes.Items))]

		for _, address := range node.Status.Addresses {
			if address.Type == v1.NodeExternalIP {
				externalAddress = address.Address
			}
			if address.Type == v1.NodeInternalIP {
				internalAddress = address.Address
			}
		}
		// Set the default to the internal address first
		ip = internalAddress

		// If a node has an external address, that takes precedence
		if externalAddress != "" {
			ip = externalAddress
		}
		port = svc.Spec.Ports[0].NodePort
		log.Printf("Swapping out IP/Port for NodePort IP/Port: %s:%d", ip, port)
	}

	for k, v := range configs {
		log.Printf("Catting file %s", v)
		data, err := c.catFile(config, v)
		if err != nil {
			return nil, err
		}
		if port > 0 {
			kubeconfigs[k] = strings.Replace(data, "0.0.0.0:6443", fmt.Sprintf("%s:%d", ip, port), -1)
		} else {
			kubeconfigs[k] = strings.Replace(data, "0.0.0.0", ip, -1)
		}
	}

	return kubeconfigs, nil
}

func (c Cluster) catFile(config *rest.Config, filename string) (data string, err error) {
	return c.execCommand(config, []string{"cat", filename})
}

func (c Cluster) execCommand(config *rest.Config, command []string) (data string, err error) {
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Printf("Unable to create clientset: %s", err.Error())
		return "", err
	}

	req := clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(c.Name).
		Namespace(c.Namespace).
		SubResource("exec")

	log.Printf("Exec command for %s/%s", c.Namespace, c.Name)
	var stdout, stderr bytes.Buffer

	req.VersionedParams(&v1.PodExecOptions{
		Command:   command,
		Container: "",
		Stdin:     false,
		Stdout:    true,
		Stderr:    true,
		TTY:       false,
	}, scheme.ParameterCodec)

	fmt.Println("Request URL:", req.URL().String())

	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		log.Printf("spdy error: %v", err)
	}

	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  nil,
		Stdout: &stdout,
		Stderr: &stderr,
		Tty:    false,
	})

	if err != nil {
		log.Printf("stderr: `%s`", stderr.String())
		log.Printf("stdout: `%s`", stdout.String())
		log.Printf("stream error: `%v`", err)
		return "", err
	}

	return stdout.String(), nil
}
