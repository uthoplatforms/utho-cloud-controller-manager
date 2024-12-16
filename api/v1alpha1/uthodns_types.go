/*
Copyright 2024 Animesh.

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

// UthoDNSSpec defines the desired state of UthoDNS
type UthoDNSSpec struct {
	Domain  string   `json:"domain"`
	Records []Record `json:"records"`
}

type Record struct {
	Hostname string `json:"hostname"`
	Type     string `json:"type"`
	TTL      int    `json:"ttl"`
	Value    string `json:"value"`
	Priority int    `json:"priority,omitempty"`
	Port     int    `json:"port,omitempty"`
	PortType string `json:"portType,omitempty"`
	Weight   int    `json:"weight,omitempty"`
}

// UthoDNSStatus defines the observed state of UthoDNS
type UthoDNSStatus struct {
	Phase       StatusPhase `json:"phase"`
	DNSRecordID []string    `json:"dnsRecordId,omitempty"`
	RecordCount int         `json:"recordCount"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName={"uthodns"}
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Domain",type=string,JSONPath=`.spec.domain`
// +kubebuilder:printcolumn:name="Records",type=integer,JSONPath=".status.recordCount"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// UthoDNS is the Schema for the uthodns API
type UthoDNS struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   UthoDNSSpec   `json:"spec,omitempty"`
	Status UthoDNSStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// UthoDNSList contains a list of UthoDNS
type UthoDNSList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []UthoDNS `json:"items"`
}

func init() {
	SchemeBuilder.Register(&UthoDNS{}, &UthoDNSList{})
}
