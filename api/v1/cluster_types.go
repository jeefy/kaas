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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ClusterSpec defines the desired state of Cluster
type ClusterSpec struct {
	KindSpec string `json:"kindSpec,omitempty"`

	ClusterYAML []string `json:"clusterYAML,omitempty"`

	Source *ClusterSource `json:"clusterSource,omitempty"`

	CPU *resource.Quantity `json:"cpu"`

	Memory *resource.Quantity `json:"memory"`
}

// +kubebuilder:object:root=true

// ClusterSource defines a cluster config source to attempt to sync to
type ClusterSource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	GitHubSource *GitHubClusterSource `json:"githubSource,omitempty"`
	FileSource   *FileClusterSource   `json:"fileSource,omitempty"`
	Enabled      bool                 `json:"enabled,omitempty"`

	Status ClusterSourceStatus `json:"status,omitempty"`
}

// GitHubClusterSource is a URL to a GitHub repo containing a kind/cluster config
type GitHubClusterSource struct {
	URL string `json:"url"`
}

// FileClusterSource is a URL to a single YAML file containing a kind/cluster config
type FileClusterSource struct {
	URL string `json:"url"`
}

// ClusterStatus defines the observed state of Cluster
type ClusterStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	LoadBalancerIP     string `json:"loadBalancerIP"`
	ClusterAdminConfig string `json:"clusterAdminConfig"`
	DefaultUserConfig  string `json:"defaultUserConfig"`
}

// ClusterSourceStatus defines the observed state of Cluster
type ClusterSourceStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true

// Cluster is the Schema for the clusters API
type Cluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterSpec   `json:"spec,omitempty"`
	Status ClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ClusterList contains a list of Cluster
type ClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Cluster `json:"items"`
}

// +kubebuilder:object:root=true

// ClusterSourceList contains a list of Cluster
type ClusterSourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterSource `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterSource{}, &ClusterSourceList{}, &Cluster{}, &ClusterList{})
}
