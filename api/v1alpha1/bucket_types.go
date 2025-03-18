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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

const Separator = "."

// BucketSpec defines the desired state of Bucket.
type BucketSpec struct {
	SecretName string `json:"secretName"`

	// Specifies which policy is attached to the current bucket.
	// Valid values are:
	// - "private" (default): forbids anonymous user to perform any action on the bucket;
	// - "public": allows any action of upload or download on the bucket for the anonymous user;
	// - "upload": allows all the upload actions to the anonymous user on the bucket;
	// - "download": allows all the download actions to the anonymous user on the bucket;
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=private
	Policy BucketPolicy `json:"policy"`
}

// BucketPolicy describes the policy attached to the bucket for the anonymous user to use.
// Only one of the following policies may be specified.
// If none of the following policies is specified, the default one
// is private.
// +kubebuilder:validation:Enum=private;public;upload;download
type BucketPolicy string

const (
	// PolicyPrivate prevents anonymous users to perform any action on the bucket.
	// The generated policy will be an empty one.
	PolicyPrivate BucketPolicy = "private"

	// PolicyPublic allows anyone to upload or download from the bucket.
	PolicyPublic BucketPolicy = "public"

	// PolicyUpload allows users to upload objects to a bucket without authentication.
	PolicyUpload BucketPolicy = "upload"

	// PolicyDownload allows users to download objects to a bucket without authentication.
	PolicyDownload BucketPolicy = "download"
)

// BucketStatus defines the observed state of Bucket.
type BucketStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Bucket is the Schema for the buckets API.
type Bucket struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BucketSpec   `json:"spec,omitempty"`
	Status BucketStatus `json:"status,omitempty"`
}

func (m Bucket) BucketName() string {
	return m.Namespace + Separator + m.Name
}

// +kubebuilder:object:root=true

// BucketList contains a list of Bucket.
type BucketList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Bucket `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Bucket{}, &BucketList{})
}
