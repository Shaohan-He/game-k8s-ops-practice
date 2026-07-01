package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

const (
	PhasePending     = "Pending"
	PhaseProgressing = "Progressing"
	PhaseReady       = "Ready"
	PhaseDegraded    = "Degraded"
)

type GamePlatformSpec struct {
	ImageRegistry string         `json:"imageRegistry,omitempty"`
	ImageTag      string         `json:"imageTag,omitempty"`
	Replicas      int32          `json:"replicas,omitempty"`
	Ingress       IngressSpec    `json:"ingress,omitempty"`
	Monitoring    MonitoringSpec `json:"monitoring,omitempty"`
	Storage       StorageSpec    `json:"storage,omitempty"`
}

type IngressSpec struct {
	Host             string `json:"host,omitempty"`
	IngressClassName string `json:"ingressClassName,omitempty"`
}

type MonitoringSpec struct {
	Enabled *bool `json:"enabled,omitempty"`
}

type StorageSpec struct {
	MySQL MySQLStorageSpec `json:"mysql,omitempty"`
}

type MySQLStorageSpec struct {
	Size string `json:"size,omitempty"`
}

type GamePlatformStatus struct {
	Phase              string             `json:"phase,omitempty"`
	Conditions         []metav1.Condition `json:"conditions,omitempty"`
	ServiceStatuses    []ServiceStatus    `json:"serviceStatuses,omitempty"`
	ObservedGeneration int64              `json:"observedGeneration,omitempty"`
}

type ServiceStatus struct {
	Name         string `json:"name"`
	Desired      int32  `json:"desired"`
	Ready        int32  `json:"ready"`
	Image        string `json:"image"`
	RolloutState string `json:"rolloutState"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=gp
type GamePlatform struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GamePlatformSpec   `json:"spec,omitempty"`
	Status GamePlatformStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type GamePlatformList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GamePlatform `json:"items"`
}
