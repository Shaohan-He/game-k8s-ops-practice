package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func (in *GamePlatform) DeepCopyInto(out *GamePlatform) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

func (in *GamePlatform) DeepCopy() *GamePlatform {
	if in == nil {
		return nil
	}
	out := new(GamePlatform)
	in.DeepCopyInto(out)
	return out
}

func (in *GamePlatform) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

func (in *GamePlatformList) DeepCopyInto(out *GamePlatformList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		out.Items = make([]GamePlatform, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&out.Items[i])
		}
	}
}

func (in *GamePlatformList) DeepCopy() *GamePlatformList {
	if in == nil {
		return nil
	}
	out := new(GamePlatformList)
	in.DeepCopyInto(out)
	return out
}

func (in *GamePlatformList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

func (in *GamePlatformSpec) DeepCopyInto(out *GamePlatformSpec) {
	*out = *in
	in.Monitoring.DeepCopyInto(&out.Monitoring)
}

func (in *MonitoringSpec) DeepCopyInto(out *MonitoringSpec) {
	*out = *in
	if in.Enabled != nil {
		out.Enabled = new(bool)
		*out.Enabled = *in.Enabled
	}
}

func (in *GamePlatformStatus) DeepCopyInto(out *GamePlatformStatus) {
	*out = *in
	if in.Conditions != nil {
		out.Conditions = make([]metav1.Condition, len(in.Conditions))
		copy(out.Conditions, in.Conditions)
	}
	if in.ServiceStatuses != nil {
		out.ServiceStatuses = make([]ServiceStatus, len(in.ServiceStatuses))
		copy(out.ServiceStatuses, in.ServiceStatuses)
	}
}

func (in *GamePlatformStatus) DeepCopy() *GamePlatformStatus {
	if in == nil {
		return nil
	}
	out := new(GamePlatformStatus)
	in.DeepCopyInto(out)
	return out
}
