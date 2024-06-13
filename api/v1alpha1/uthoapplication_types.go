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

// UthoApplicationSpec defines the desired state of UthoApplication
type UthoApplicationSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of UthoApplication. Edit uthoapplication_types.go to remove/update
	LoadBalancer LoadBalancer  `json:"loadBalancer"`
	TargetGroups []TargetGroup `json:"targetGroups"`
}

type LoadBalancer struct {
	Frontend Frontend `json:"frontend,omitempty"`
	// +kubebuilder:default:=application
	Type   string    `json:"type,omitempty"`
	Dcslug string    `json:"dcslug"`
	Name   string    `json:"name"`
	ACL    []ACLRule `json:"aclRule,omitempty"`
}

type Frontend struct {
	Name            string `json:"name"`
	Algorithm       string `json:"algorithm"`
	Protocol        string `json:"protocol"`
	Port            int64  `json:"port"`
	CertificateName string `json:"certificateName,omitempty"`
	RedirectHttps   bool   `json:"redirectHttps,omitempty"`
	Cookie          bool   `json:"cookie,omitempty"`
}

type ACLRule struct {
	Name          string  `json:"name"`
	ConditionType string  `json:"conditionType"`
	Value         ACLData `json:"value"`
}

type ACLData struct {
	FrontendID string   `json:"frontend_id,omitempty"`
	Type       string   `json:"type"`
	Data       []string `json:"data"`
}

type TargetGroup struct {
	Name                string `json:"name"`
	Protocol            string `json:"protocol"`
	HealthCheckPath     string `json:"health_check_path"`
	HealthCheckProtocol string `json:"health_check_protocol"`
	HealthCheckInterval int64  `json:"health_check_interval"`
	HealthCheckTimeout  int64  `json:"health_check_timeout"`
	HealthyThreshold    int64  `json:"healthy_threshold"`
	UnhealthyThreshold  int64  `json:"unhealthy_threshold"`
	Port                int64  `json:"port"`
}

type StatusPhase string

const (
	RunningPhase             StatusPhase = "RUNNING"
	LBPendingPhase           StatusPhase = "LB_PENDING"
	LBCreatedPhase           StatusPhase = "LB_CREATED"
	LBErrorPhase             StatusPhase = "LB_ERROR"
	LBDeletedPhase           StatusPhase = "LB_DELETED"
	LBDeletionErrorPhase     StatusPhase = "LB_DELETION_ERROR"
	TGPendingPhase           StatusPhase = "TG_PENDING"
	TGCreatedPhase           StatusPhase = "TG_CREATED"
	TGErrorPhase             StatusPhase = "TG_ERROR"
	TGDeletedPhase           StatusPhase = "TG_DELETED"
	TGDeletionErrorPhase     StatusPhase = "TG_DELETION_ERROR"
	LBAttachmentPendingPhase StatusPhase = "LB_ATTACHMENT_PENDING"
	LBAttachmentCreatedPhase StatusPhase = "LB__ATTACHMENT_CREATED"
	LBAttachmentErrorPhase   StatusPhase = "LB_ATTACHMENT_ERROR"
	FrontendPendingPhase     StatusPhase = "FRONTEND_PENDING"
	FrontendCreatedPhase     StatusPhase = "FRONTEND_CREATED"
	FrontendErrorPhase       StatusPhase = "FRONTEND_ERROR"
	TGAttachmentPendingPhase StatusPhase = "TG_ATTACHMENT_PENDING"
	TGAttachmentCreatedPhase StatusPhase = "TG_ATTACHMENT_CREATED"
	TGAttachmentErrorPhase   StatusPhase = "TG_ATTACHMENT_PHASE"
	ACLPendingPhase          StatusPhase = "ACL_PENDING"
	ACLCreatedPhase          StatusPhase = "ACL_CREATED"
	ACLErrorPhase            StatusPhase = "ACL_ERROR"
)

// UthoApplicationStatus defines the observed state of UthoApplication
type UthoApplicationStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	LoadBalancerID string      `json:"load_balancer_id"`
	LoadBalancerIP string      `json:"load_balancer_ip"`
	FrontendID     string      `json:"frontend_id"`
	TargetGroupsID []string    `json:"target_group_ids,omitempty"`
	ACLRuleIDs     []string    `json:"acl_rule_ids,omitempty"`
	Phase          StatusPhase `json:"phase"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName={"utho-app"}
// +kubebuilder:printcolumn:name="Load-Balancer-IP",type=string,JSONPath=`.status.load_balancer_ip`
// +kubebuilder:printcolumn:name="Load-Balancer-Type",type=string,JSONPath=`.spec.loadBalancer.type`
// +kubebuilder:printcolumn:name="Frontend-Port",type=integer,JSONPath=`.spec.loadBalancer.frontend.port`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// UthoApplication is the Schema for the uthoapplications API
type UthoApplication struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   UthoApplicationSpec   `json:"spec,omitempty"`
	Status UthoApplicationStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// UthoApplicationList contains a list of UthoApplication
type UthoApplicationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []UthoApplication `json:"items"`
}

func init() {
	SchemeBuilder.Register(&UthoApplication{}, &UthoApplicationList{})
}
