/*
Copyright 2025.

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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PolicySpec defines the desired state of Policy.
type PolicySpec struct {
	// +kubebuilder:validation:Required
	BucketName string `json:"bucketName"`

	// +kubebuilder:validation:Optional
	SecretName string `json:"secretName"`

	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:Required
	Statements []Statement `json:"statements"`
}

type Statement struct {
	// +kubebuilder:validation:Optional
	// +listType=set
	SubPaths []string `json:"subPaths"`
	// +kubebuilder:validation:Enum=Allow;Deny
	// +kubebuilder:validation:Required
	Effect string `json:"effect"`
	// +kubebuilder:validation:Required
	// +listType=set
	Actions []string `json:"actions"`
}

// PolicyStatus defines the observed state of Policy.
type PolicyStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Policy is the Schema for the policies API.
type Policy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:Required
	Spec   PolicySpec   `json:"spec"`
	Status PolicyStatus `json:"status,omitempty"`
}

func (p Policy) PolicyName() string {
	return p.Namespace + Separator + p.Spec.BucketName + Separator + p.Name
}

// +kubebuilder:object:root=true

// PolicyList contains a list of Policy.
type PolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Policy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Policy{}, &PolicyList{})
}
