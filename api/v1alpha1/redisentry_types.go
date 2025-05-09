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

// RedisEntrySpec defines the desired state of RedisEntry.
type RedisEntrySpec struct {
	// Key is the Redis key to be set
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Key string `json:"key"`

	// Value is the value to be stored in Redis
	// +kubebuilder:validation:Required
	Value string `json:"value"`

	// TTL is the time-to-live in seconds for the key-value pair
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=0
	TTL *int64 `json:"ttl,omitempty"`
}

// RedisEntryStatus defines the observed state of RedisEntry.
type RedisEntryStatus struct {
	// Conditions represent the latest available observations of the RedisEntry's state
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// LastUpdated is the timestamp of the last successful update to Redis
	// +optional
	LastUpdated *metav1.Time `json:"lastUpdated,omitempty"`

	// CurrentValue represents the current value in Redis for the key
	// +optional
	CurrentValue string `json:"currentValue,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Key",type="string",JSONPath=".spec.key"
// +kubebuilder:printcolumn:name="Value",type="string",JSONPath=".spec.value"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="Last Updated",type="date",JSONPath=".status.lastUpdated"

// RedisEntry is the Schema for the redisentries API.
type RedisEntry struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RedisEntrySpec   `json:"spec,omitempty"`
	Status RedisEntryStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RedisEntryList contains a list of RedisEntry.
type RedisEntryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RedisEntry `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RedisEntry{}, &RedisEntryList{})
}
