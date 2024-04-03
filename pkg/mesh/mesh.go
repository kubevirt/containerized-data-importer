package mesh

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ApplyPreStopHook applies the appropriate mesh pre stop hook if the service mesh annotation is present
func ApplyPreStopHook(objMeta metav1.ObjectMeta, ctr *v1.Container) {
	if isL5dMeshed(objMeta.Annotations) {
		ctr.Lifecycle = &v1.Lifecycle{
			PreStop: L5dPreStopHook(),
		}
		return
	}
	if isIstioMeshed(objMeta.Annotations) {
		ctr.Lifecycle = &v1.Lifecycle{
			PreStop: IstioPreStopHook(),
		}
	}
}
