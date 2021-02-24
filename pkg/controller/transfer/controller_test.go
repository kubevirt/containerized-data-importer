package transfer_test

import (
	"context"
	"reflect"

	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/client/clientset/versioned/scheme"
	"kubevirt.io/containerized-data-importer/pkg/controller/transfer"
)

func rr(name string) reconcile.Request {
	return reconcile.Request{NamespacedName: types.NamespacedName{Name: name}}
}

func getResource(c client.Client, ns, name string, obj runtime.Object) error {
	p := reflect.ValueOf(obj).Elem()
	p.Set(reflect.Zero(p.Type()))
	return c.Get(context.TODO(), types.NamespacedName{Namespace: ns, Name: name}, obj)
}

func checkCompleteFalse(ot *cdiv1.ObjectTransfer, m, r string) {
	Expect(ot.Status.Conditions).To(HaveLen(1))
	cond := ot.Status.Conditions[0]
	Expect(cond.Type).To(Equal(cdiv1.ObjectTransferConditionComplete))
	Expect(cond.Status).To(Equal(corev1.ConditionFalse))
	Expect(cond.Message).To(Equal(m))
	Expect(cond.Reason).To(Equal(r))
	Expect(cond.LastHeartbeatTime.Unix()).ToNot(BeZero())
	Expect(cond.LastTransitionTime.Unix()).ToNot(BeZero())
}

func checkCompleteTrue(ot *cdiv1.ObjectTransfer) {
	Expect(ot.Status.Conditions).To(HaveLen(1))
	cond := ot.Status.Conditions[0]
	Expect(cond.Type).To(Equal(cdiv1.ObjectTransferConditionComplete))
	Expect(cond.Status).To(Equal(corev1.ConditionTrue))
	Expect(cond.Message).To(Equal("Transfer complete"))
	Expect(cond.Reason).To(Equal(""))
	Expect(cond.LastHeartbeatTime.Unix()).ToNot(BeZero())
	Expect(cond.LastTransitionTime.Unix()).ToNot(BeZero())
}

func createReconciler(objects ...runtime.Object) *transfer.ObjectTransferReconciler {
	objs := []runtime.Object{}
	objs = append(objs, objects...)

	s := scheme.Scheme
	corev1.AddToScheme(s)
	cdiv1.AddToScheme(s)

	cl := fake.NewFakeClientWithScheme(s, objs...)

	return &transfer.ObjectTransferReconciler{
		Client:   cl,
		Scheme:   s,
		Log:      logf.Log.WithName("transfer-controller-test"),
		Recorder: record.NewFakeRecorder(10),
	}
}
