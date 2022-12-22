/*
Copyright 2022.

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// IndexSpec defines the desired state of Index
type IndexSpec struct {

	// Index Name, use this to override defaults
	// +optional
	Name string `json:"name,omitempty"`
	// Application Name
	Application string `json:"application"`

	// Config Map name to be used contained the create Index Payload including settings and mappings
	// +optional
	ConfigMap string `json:"configMap,omitempty"`

	// +optional
	NumberOfShards int `json:"numberOfShards,omitempty"`
	// +optional
	NumberOfReplicas int `json:"numberOfReplicas,omitempty"`
	// +optional
	RefreshInterval string `json:"refreshInterval,omitempty"`
	// +optional
	Analyzers string `json:"analyzers,omitempty"`
	// +optional
	SourceEnabled bool `json:"sourceEnabled,omitempty"`
	// +optional
	Properties string `json:"properties,omitempty"`
}

type IndexStatusEnum string

const (
	Creating IndexStatusEnum = "Creating"

	Created IndexStatusEnum = "Created"

	Ready IndexStatusEnum = "Ready"

	Error IndexStatusEnum = "Error"
)

// IndexStatus defines the observed state of Index
type IndexStatus struct {
	IndexStatus IndexStatusEnum `json:"indexStatus,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Index is the Schema for the indices API
type Index struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   IndexSpec   `json:"spec,omitempty"`
	Status IndexStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// IndexList contains a list of Index
type IndexList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Index `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Index{}, &IndexList{})
}
