package transfer

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
)

func indexKeyFunc(namespace, name string) string {
	return fmt.Sprintf("%s/%s", namespace, name)
}

func indexAndWatch(mgr manager.Manager, ctrl controller.Controller, obj runtime.Object, field string) error {
	// setup index for the type
	if err := mgr.GetFieldIndexer().IndexField(context.TODO(), &cdiv1.ObjectTransfer{}, field, func(obj runtime.Object) []string {
		var result []string
		ot := obj.(*cdiv1.ObjectTransfer)

		if strings.ToLower(ot.Spec.Source.Kind) == field {
			result = []string{
				indexKeyFunc(ot.Spec.Source.Namespace, ot.Spec.Source.Name),
				indexKeyFunc(getTransferTargetNamespace(ot), getTransferTargetName(ot)),
			}
		}

		return result
	}); err != nil {
		return err
	}

	if err := ctrl.Watch(&source.Kind{Type: obj}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(mapObj handler.MapObject) []reconcile.Request {
			value := indexKeyFunc(mapObj.Meta.GetNamespace(), mapObj.Meta.GetName())
			return indexLookup(mgr.GetClient(), field, value)
		}),
	}); err != nil {
		return err
	}

	return nil
}

func indexLookup(c client.Client, field, value string) []reconcile.Request {
	var result []reconcile.Request
	var objs cdiv1.ObjectTransferList

	if err := c.List(context.TODO(), &objs, &client.ListOptions{
		FieldSelector: fields.OneTermEqualSelector(field, value),
	}); err != nil {
		return nil
	}

	for _, obj := range objs.Items {
		result = append(result, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name: obj.Name,
			},
		})
	}

	return result
}

func watchObjectTransfers(ctrl controller.Controller) error {
	return ctrl.Watch(&source.Kind{Type: &cdiv1.ObjectTransfer{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(mapObj handler.MapObject) []reconcile.Request {
			obj := mapObj.Object.(*cdiv1.ObjectTransfer)
			result := []reconcile.Request{
				{
					NamespacedName: types.NamespacedName{
						Name: obj.Name,
					},
				},
			}

			if obj.Spec.ParentName != nil {
				result = append(result, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name: *obj.Spec.ParentName,
					},
				})
			}

			return result
		}),
	})
}

func watchPVs(mgr manager.Manager, ctrl controller.Controller) error {
	return ctrl.Watch(&source.Kind{Type: &corev1.PersistentVolume{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(mapObj handler.MapObject) []reconcile.Request {
			pv := mapObj.Object.(*corev1.PersistentVolume)
			if pv.Spec.ClaimRef == nil {
				return nil
			}
			value := indexKeyFunc(pv.Spec.ClaimRef.Namespace, pv.Spec.ClaimRef.Name)
			return indexLookup(mgr.GetClient(), "persistentvolumeclaim", value)
		}),
	})
}

func addObjectTransferControllerWatches(mgr manager.Manager, ctrl controller.Controller) error {
	// Add schemes.
	if err := cdiv1.AddToScheme(mgr.GetScheme()); err != nil {
		return err
	}

	if err := watchObjectTransfers(ctrl); err != nil {
		return err
	}

	if err := indexAndWatch(mgr, ctrl, &cdiv1.DataVolume{}, "datavolume"); err != nil {
		return err
	}

	if err := indexAndWatch(mgr, ctrl, &corev1.PersistentVolumeClaim{}, "persistentvolumeclaim"); err != nil {
		return err
	}

	if err := watchPVs(mgr, ctrl); err != nil {
		return err
	}

	return nil
}
