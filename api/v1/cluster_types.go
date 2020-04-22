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

package v1

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// +kubebuilder:object:root=true

// KaasConfig contains some global config information used by Kaas
type KaasConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	DefaultServiceType v1.ServiceType `json:"defaultServiceType,omitempty"`
	DefaultPort        v1.ServicePort `json:"defaultPort,omitempty"`
}

// ClusterType is a list of the types of local clusters we can provision
type ClusterType string

const (
	// KindCluster is a sigs.k8s.io/kind cluster
	KindCluster ClusterType = "kind"
	// K3sCluster is a k3s.io cluster
	K3sCluster ClusterType = "k3s"
)

// ClusterSpec defines the desired state of Cluster
type ClusterSpec struct {
	ClusterType ClusterType `json:"clusterType"`

	ClusterSpec string `json:"clusterSpec,omitempty"`

	ClusterYAML []string `json:"clusterYAML,omitempty"`

	Image string `json:"image,omitempty"`

	CPU *resource.Quantity `json:"cpu"`

	Memory *resource.Quantity `json:"memory"`
}

// ClusterStatus defines the observed state of Cluster
type ClusterStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Ready          bool   `json:"ready"`
	LoadBalancerIP string `json:"loadBalancerIP"`
}

// +kubebuilder:object:root=true

// Cluster is the Schema for the clusters API
type Cluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterSpec   `json:"spec,omitempty"`
	Status ClusterStatus `json:"status,omitempty"`

	KaasConfig *KaasConfig `json:"kaasConfig,omitempty"`
}

// +kubebuilder:object:root=true

// ClusterList contains a list of Cluster
type ClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Cluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Cluster{}, &ClusterList{}, &KaasConfig{})
}
