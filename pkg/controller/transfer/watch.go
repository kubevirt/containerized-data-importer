package transfer

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

func indexKeyFunc(namespace, name string) string {
	return fmt.Sprintf("%s/%s", namespace, name)
}

func indexAndWatch(mgr manager.Manager, ctrl controller.Controller, obj client.Object, field string) error {
	// setup index for the type
	if err := mgr.GetFieldIndexer().IndexField(context.TODO(), &cdiv1.ObjectTransfer{}, field, func(obj client.Object) []string {
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

	if err := ctrl.Watch(source.Kind(mgr.GetCache(), obj), handler.EnqueueRequestsFromMapFunc(
		func(ctx context.Context, obj client.Object) []reconcile.Request {
			value := indexKeyFunc(obj.GetNamespace(), obj.GetName())
			return indexLookup(ctx, mgr.GetClient(), field, value)
		},
	)); err != nil {
		return err
	}

	return nil
}

func indexLookup(ctx context.Context, c client.Client, field, value string) []reconcile.Request {
	var result []reconcile.Request
	var objs cdiv1.ObjectTransferList

	if err := c.List(ctx, &objs, &client.ListOptions{
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

func watchObjectTransfers(mgr manager.Manager, ctrl controller.Controller) error {
	return ctrl.Watch(source.Kind(mgr.GetCache(), &cdiv1.ObjectTransfer{}), handler.EnqueueRequestsFromMapFunc(
		func(_ context.Context, obj client.Object) []reconcile.Request {
			ot := obj.(*cdiv1.ObjectTransfer)
			result := []reconcile.Request{
				{
					NamespacedName: types.NamespacedName{
						Name: ot.Name,
					},
				},
			}

			if ot.Spec.ParentName != nil {
				result = append(result, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name: *ot.Spec.ParentName,
					},
				})
			}

			return result
		},
	))
}

func watchPVs(mgr manager.Manager, ctrl controller.Controller) error {
	return ctrl.Watch(source.Kind(mgr.GetCache(), &corev1.PersistentVolume{}), handler.EnqueueRequestsFromMapFunc(
		func(ctx context.Context, obj client.Object) []reconcile.Request {
			pv := obj.(*corev1.PersistentVolume)
			if pv.Spec.ClaimRef == nil {
				return nil
			}
			value := indexKeyFunc(pv.Spec.ClaimRef.Namespace, pv.Spec.ClaimRef.Name)
			return indexLookup(ctx, mgr.GetClient(), "persistentvolumeclaim", value)
		},
	))
}

func addObjectTransferControllerWatches(mgr manager.Manager, ctrl controller.Controller) error {
	if err := watchObjectTransfers(mgr, ctrl); err != nil {
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
