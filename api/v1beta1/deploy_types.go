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

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// DeploySpec defines the desired state of Deploy
type DeploySpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of Deploy. Edit deploy_types.go to remove/update
	ServiceAccount string        `json:"serviceAccount"`
	Terraform      TerraformSpec `json:"terraform"`
	Source         SourceSpec    `json:"source"`
	State          StateSpec     `json:"state"`
}

type TerraformSpec struct {
	Image    string   `json:"image"`
	Commands []string `json:"commands"`
	Version  string   `json:"version"`
}
type SourceSpec struct {
	Repo            string `json:"repo"`
	RefreshInterval int    `json:"refresh-interval"`
	Branch          string `json:"branch"`
	EntryPoint      string `json:"entryPoint"`
}

type StateSpec struct {
	Url string `json:"url"`
}

// DeployStatus defines the observed state of Deploy
type DeployStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Deploy is the Schema for the deploys API
type Deploy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DeploySpec   `json:"spec,omitempty"`
	Status DeployStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// DeployList contains a list of Deploy
type DeployList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Deploy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Deploy{}, &DeployList{})
}
