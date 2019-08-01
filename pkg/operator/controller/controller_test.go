/*
Copyright 2018 The CDI Authors.

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

package controller

import (
	"context"
	"os"
	"reflect"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	routev1 "github.com/openshift/api/route/v1"
	secv1 "github.com/openshift/api/security/v1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	extv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"

	"k8s.io/client-go/kubernetes/scheme"

	"sigs.k8s.io/controller-runtime/pkg/client"
	realClient "sigs.k8s.io/controller-runtime/pkg/client"
	fakeClient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	cdiviaplha1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"
	clusterResources "kubevirt.io/containerized-data-importer/pkg/operator/resources/cluster"
	namespaceResources "kubevirt.io/containerized-data-importer/pkg/operator/resources/namespaced"
)

const (
	version                   = "v1.5.0"
	cdiNamespace              = "cdi"
	configMapName             = "cdi-config"
	insecureRegistryConfigMap = "cdi-insecure-registries"
)

type args struct {
	cdi        *cdiviaplha1.CDI
	scc        *secv1.SecurityContextConstraints
	client     client.Client
	reconciler *ReconcileCDI
}

var (
	envVars = map[string]string{
		"DOCKER_REPO":              "kubevirt",
		"DOCKER_TAG":               version,
		"DEPLOY_CLUSTER_RESOURCES": "true",
		"CONTROLLER_IMAGE":         "cdi-controller",
		"IMPORTER_IMAGE":           "cdi-importer",
		"CLONER_IMAGE":             "cdi-cloner",
		"UPLOAD_PROXY_IMAGE":       "cdi-uploadproxy",
		"UPLOAD_SERVER_IMAGE":      "cdi-uploadserver",
		"APISERVER_IMAGE":          "cdi-apiserver",
		"VERBOSITY":                "1",
		"PULL_POLICY":              "Always",
	}
)

func init() {
	cdiviaplha1.AddToScheme(scheme.Scheme)
	extv1beta1.AddToScheme(scheme.Scheme)
	secv1.Install(scheme.Scheme)
	routev1.Install(scheme.Scheme)
}

var _ = Describe("Controller", func() {
	Describe("controller runtime bootstrap test", func() {
		Context("Create manager and controller", func() {
			BeforeEach(func() {
				for k, v := range envVars {
					os.Setenv(k, v)
				}
			})

			AfterEach(func() {
				for k := range envVars {
					os.Unsetenv(k)
				}
			})

			It("should succeed", func() {
				mgr, err := manager.New(cfg, manager.Options{})
				Expect(err).ToNot(HaveOccurred())

				err = cdiviaplha1.AddToScheme(mgr.GetScheme())
				Expect(err).ToNot(HaveOccurred())

				err = extv1beta1.AddToScheme(mgr.GetScheme())
				Expect(err).ToNot(HaveOccurred())

				err = secv1.Install(mgr.GetScheme())
				Expect(err).ToNot(HaveOccurred())

				err = Add(mgr)
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})

	DescribeTable("check can create types", func(obj runtime.Object) {
		client := createClient(obj)

		_, err := getObject(client, obj)
		Expect(err).ToNot(HaveOccurred())
	},
		Entry("CDI type", createCDI("cdi", "good uid")),
		Entry("CDR type", &extv1beta1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "crd"}}),
		Entry("SSC type", &secv1.SecurityContextConstraints{ObjectMeta: metav1.ObjectMeta{Name: "scc"}}),
		Entry("Route type", &routev1.Route{ObjectMeta: metav1.ObjectMeta{Name: "route"}}),
	)

	Describe("Deploying CDI", func() {
		Context("CDI lifecycle", func() {
			It("should get deployed", func() {
				args := createArgs()
				doReconcile(args)

				Expect(args.cdi.Status.OperatorVersion).Should(Equal(version))
				Expect(args.cdi.Status.TargetVersion).Should(Equal(version))
				Expect(args.cdi.Status.ObservedVersion).Should(Equal(version))

				Expect(args.cdi.Status.Phase).Should(Equal(cdiviaplha1.CDIPhaseDeployed))
				Expect(args.cdi.Status.Conditions).Should(BeEmpty())

				Expect(args.cdi.Finalizers).Should(HaveLen(1))
			})

			It("should create configmap", func() {
				args := createArgs()
				doReconcile(args)

				cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: cdiNamespace, Name: configMapName}}
				obj, err := getObject(args.client, cm)
				Expect(err).ToNot(HaveOccurred())

				cm = obj.(*corev1.ConfigMap)
				Expect(cm.OwnerReferences[0].UID).Should(Equal(args.cdi.UID))
			})

			It("should be anyuid", func() {
				var err error

				args := createArgs()
				doReconcile(args)

				expectedUsers := namespaceResources.GetPrivilegedAccounts(args.reconciler.namespacedArgs)
				Expect(expectedUsers).To(HaveLen(1))
				Expect(expectedUsers).To(HaveKey("anyuid"))

				args.scc, err = getSCC(args.client, args.scc)
				Expect(err).ToNot(HaveOccurred())

				for _, eu := range expectedUsers["anyuid"] {
					found := false
					for _, au := range args.scc.Users {
						if eu == au {
							found = true
						}
					}
					Expect(found).To(BeTrue())
				}
			})

			It("should create all resources", func() {
				args := createArgs()
				doReconcile(args)

				resources, err := getAllResources(args.reconciler)
				Expect(err).ToNot(HaveOccurred())

				for _, r := range resources {
					_, err := getObject(args.client, r)
					Expect(err).ToNot(HaveOccurred())
				}
			})

			It("should become ready", func() {
				args := createArgs()
				doReconcile(args)

				resources, err := getAllResources(args.reconciler)
				Expect(err).ToNot(HaveOccurred())

				for _, r := range resources {
					d, ok := r.(*appsv1.Deployment)
					if !ok {
						continue
					}

					numReplicas := d.Spec.Replicas
					Expect(numReplicas).ToNot(BeNil())

					d, err := getDeployment(args.client, d)
					Expect(err).ToNot(HaveOccurred())
					d.Status.Replicas = *numReplicas
					err = args.client.Update(context.TODO(), d)
					Expect(err).ToNot(HaveOccurred())

					doReconcile(args)

					Expect(args.cdi.Status.Conditions).Should(BeEmpty())
				}

				resources, err = getAllResources(args.reconciler)
				Expect(err).ToNot(HaveOccurred())
				running := false

				createUploadProxyCACertSecret(args.client)

				for _, r := range resources {
					d, ok := r.(*appsv1.Deployment)
					if !ok {
						continue
					}

					Expect(running).To(BeFalse())

					d, err := getDeployment(args.client, d)
					Expect(err).ToNot(HaveOccurred())
					d.Status.ReadyReplicas = d.Status.Replicas
					err = args.client.Update(context.TODO(), d)
					Expect(err).ToNot(HaveOccurred())

					doReconcile(args)

					if len(args.cdi.Status.Conditions) == 1 &&
						args.cdi.Status.Conditions[0].Type == cdiviaplha1.CDIConditionRunning &&
						args.cdi.Status.Conditions[0].Status == corev1.ConditionTrue {
						running = true
					}
				}

				Expect(running).To(BeTrue())

				route := &routev1.Route{
					ObjectMeta: metav1.ObjectMeta{
						Name:      uploadProxyRouteName,
						Namespace: cdiNamespace,
					},
				}

				obj, err := getObject(args.client, route)
				Expect(err).To(BeNil())
				route = obj.(*routev1.Route)

				Expect(route.Spec.To.Kind).Should(Equal("Service"))
				Expect(route.Spec.To.Name).Should(Equal(uploadProxyServiceName))
				Expect(route.Spec.TLS.DestinationCACertificate).Should(Equal("hello world"))
			})

			It("can become become ready, un-ready, and ready again", func() {
				var deployment *appsv1.Deployment

				args := createArgs()
				doReconcile(args)

				resources, err := getAllResources(args.reconciler)
				Expect(err).ToNot(HaveOccurred())

				createUploadProxyCACertSecret(args.client)

				for _, r := range resources {
					d, ok := r.(*appsv1.Deployment)
					if !ok {
						continue
					}

					d.Status.Replicas = *d.Spec.Replicas
					d.Status.ReadyReplicas = d.Status.Replicas

					err = args.client.Update(context.TODO(), d)
					Expect(err).ToNot(HaveOccurred())
				}

				doReconcile(args)

				Expect(args.cdi.Status.Conditions).Should(HaveLen(1))
				Expect(args.cdi.Status.Conditions[0].Type).Should(Equal(cdiviaplha1.CDIConditionRunning))
				Expect(args.cdi.Status.Conditions[0].Status).Should(Equal(corev1.ConditionTrue))

				for _, r := range resources {
					var ok bool
					deployment, ok = r.(*appsv1.Deployment)
					if ok {
						break
					}
				}

				deployment, err = getDeployment(args.client, deployment)
				Expect(err).ToNot(HaveOccurred())
				deployment.Status.ReadyReplicas = 0
				err = args.client.Update(context.TODO(), deployment)
				Expect(err).ToNot(HaveOccurred())

				doReconcile(args)

				Expect(args.cdi.Status.Conditions).Should(HaveLen(1))
				Expect(args.cdi.Status.Conditions[0].Type).Should(Equal(cdiviaplha1.CDIConditionRunning))
				Expect(args.cdi.Status.Conditions[0].Status).Should(Equal(corev1.ConditionFalse))

				deployment, err = getDeployment(args.client, deployment)
				Expect(err).ToNot(HaveOccurred())
				deployment.Status.ReadyReplicas = deployment.Status.Replicas
				err = args.client.Update(context.TODO(), deployment)
				Expect(err).ToNot(HaveOccurred())

				doReconcile(args)

				Expect(args.cdi.Status.Conditions).Should(HaveLen(1))
				Expect(args.cdi.Status.Conditions[0].Type).Should(Equal(cdiviaplha1.CDIConditionRunning))
				Expect(args.cdi.Status.Conditions[0].Status).Should(Equal(corev1.ConditionTrue))
			})

			It("does not modify insecure registry configmap", func() {
				args := createArgs()
				doReconcile(args)

				cm := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      insecureRegistryConfigMap,
						Namespace: cdiNamespace,
					},
				}

				obj, err := getObject(args.client, cm)
				Expect(err).To(BeNil())
				cm = obj.(*corev1.ConfigMap)

				if cm.Data == nil {
					cm.Data = make(map[string]string)
				}
				data := cm.Data
				data["foo.bar.com"] = ""

				err = args.client.Update(context.TODO(), cm)
				Expect(err).To(BeNil())

				doReconcile(args)

				obj, err = getObject(args.client, cm)
				Expect(err).To(BeNil())
				cm = obj.(*corev1.ConfigMap)

				Expect(cm.Data).Should(Equal(data))
			})

			It("should be an error when creating another CDI instance", func() {
				args := createArgs()
				doReconcile(args)

				newInstance := createCDI("bad", "bad")
				err := args.client.Create(context.TODO(), newInstance)
				Expect(err).ToNot(HaveOccurred())

				result, err := args.reconciler.Reconcile(reconcileRequest(newInstance.Name))
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Requeue).To(BeFalse())

				newInstance, err = getCDI(args.client, newInstance)
				Expect(err).ToNot(HaveOccurred())

				Expect(newInstance.Status.Phase).Should(Equal(cdiviaplha1.CDIPhaseError))
				Expect(newInstance.Status.Conditions).Should(BeEmpty())
			})

			It("should succeed when we delete CDI", func() {
				// create rando pod that should get deleted
				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo",
						Namespace: "default",
						Labels: map[string]string{
							"cdi.kubevirt.io": "",
						},
					},
				}

				args := createArgs()
				doReconcile(args)

				err := args.client.Create(context.TODO(), pod)
				Expect(err).ToNot(HaveOccurred())

				args.cdi.DeletionTimestamp = &metav1.Time{Time: time.Now()}
				err = args.client.Update(context.TODO(), args.cdi)
				Expect(err).ToNot(HaveOccurred())

				doReconcile(args)

				Expect(args.cdi.Finalizers).Should(BeEmpty())
				Expect(args.cdi.Status.Phase).Should(Equal(cdiviaplha1.CDIPhaseDeleted))

				_, err = getObject(args.client, pod)
				Expect(errors.IsNotFound(err)).To(BeTrue())
			})
		})
	})

	DescribeTable("should allow registry override", func(o cdiOverride) {
		args := createArgs()

		o.Set(args.cdi)
		err := args.client.Update(context.TODO(), args.cdi)
		Expect(err).ToNot(HaveOccurred())

		doReconcile(args)

		resources, err := getAllResources(args.reconciler)
		Expect(err).ToNot(HaveOccurred())

		for _, r := range resources {
			d, ok := r.(*appsv1.Deployment)
			if !ok {
				continue
			}

			d, err = getDeployment(args.client, d)
			Expect(err).ToNot(HaveOccurred())

			o.Check(d)
		}
	},
		Entry("Registry override", &registryOverride{"quay.io/kybevirt"}),
		Entry("Tag override", &registryOverride{"v1.100.0"}),
		Entry("Pull override", &pullOverride{corev1.PullNever}),
	)
})

type cdiOverride interface {
	Set(cr *cdiviaplha1.CDI)
	Check(d *appsv1.Deployment)
}

type registryOverride struct {
	value string
}

func (o *registryOverride) Set(cr *cdiviaplha1.CDI) {
	cr.Spec.ImageRegistry = o.value
}

func (o *registryOverride) Check(d *appsv1.Deployment) {
	image := d.Spec.Template.Spec.Containers[0].Image
	Expect(strings.HasPrefix(image, o.value)).To(BeTrue())
}

type tagOverride struct {
	value string
}

func (o *tagOverride) Set(cr *cdiviaplha1.CDI) {
	cr.Spec.ImageTag = o.value
}

func (o *tagOverride) Check(d *appsv1.Deployment) {
	image := d.Spec.Template.Spec.Containers[0].Image
	Expect(strings.HasSuffix(image, ":"+o.value)).To(BeTrue())
}

type pullOverride struct {
	value corev1.PullPolicy
}

func (o *pullOverride) Set(cr *cdiviaplha1.CDI) {
	cr.Spec.ImagePullPolicy = o.value
}

func (o *pullOverride) Check(d *appsv1.Deployment) {
	pp := d.Spec.Template.Spec.Containers[0].ImagePullPolicy
	Expect(pp).Should(Equal(o.value))
}

func getCDI(client realClient.Client, cdi *cdiviaplha1.CDI) (*cdiviaplha1.CDI, error) {
	result, err := getObject(client, cdi)
	if err != nil {
		return nil, err
	}
	return result.(*cdiviaplha1.CDI), nil
}

func getSCC(client realClient.Client, scc *secv1.SecurityContextConstraints) (*secv1.SecurityContextConstraints, error) {
	result, err := getObject(client, scc)
	if err != nil {
		return nil, err
	}
	return result.(*secv1.SecurityContextConstraints), nil
}

func getDeployment(client realClient.Client, deployment *appsv1.Deployment) (*appsv1.Deployment, error) {
	result, err := getObject(client, deployment)
	if err != nil {
		return nil, err
	}
	return result.(*appsv1.Deployment), nil
}

func getObject(client realClient.Client, obj runtime.Object) (runtime.Object, error) {
	metaObj := obj.(metav1.Object)
	key := realClient.ObjectKey{Namespace: metaObj.GetNamespace(), Name: metaObj.GetName()}

	typ := reflect.ValueOf(obj).Elem().Type()
	result := reflect.New(typ).Interface().(runtime.Object)

	if err := client.Get(context.TODO(), key, result); err != nil {
		return nil, err
	}

	return result, nil
}

func getAllResources(reconciler *ReconcileCDI) ([]runtime.Object, error) {
	var result []runtime.Object
	crs, err := clusterResources.CreateAllResources(reconciler.clusterArgs)
	if err != nil {
		return nil, err
	}

	result = append(result, crs...)

	nrs, err := namespaceResources.CreateAllResources(reconciler.namespacedArgs)
	if err != nil {
		return nil, err
	}

	result = append(result, nrs...)

	return result, nil
}

func reconcileRequest(name string) reconcile.Request {
	return reconcile.Request{NamespacedName: types.NamespacedName{Name: name}}
}

func createArgs() *args {
	cdi := createCDI("cdi", "good uid")
	scc := createSCC()
	client := createClient(cdi, scc)
	reconciler := createReconciler(client)

	return &args{
		cdi:        cdi,
		scc:        scc,
		client:     client,
		reconciler: reconciler,
	}
}

func doReconcile(args *args) {
	result, err := args.reconciler.Reconcile(reconcileRequest(args.cdi.Name))
	Expect(err).ToNot(HaveOccurred())
	Expect(result.Requeue).To(BeFalse())

	args.cdi, err = getCDI(args.client, args.cdi)
	Expect(err).ToNot(HaveOccurred())
}

func createClient(objs ...runtime.Object) realClient.Client {
	return fakeClient.NewFakeClientWithScheme(scheme.Scheme, objs...)
}

func createCDI(name, uid string) *cdiviaplha1.CDI {
	return &cdiviaplha1.CDI{ObjectMeta: metav1.ObjectMeta{Name: name, UID: types.UID(uid)}}
}

func createSCC() *secv1.SecurityContextConstraints {
	return &secv1.SecurityContextConstraints{ObjectMeta: metav1.ObjectMeta{Name: "anyuid"}}
}

func createReconciler(client realClient.Client) *ReconcileCDI {
	namespace := "cdi"
	clusterArgs := &clusterResources.FactoryArgs{Namespace: namespace}
	namespacedArgs := &namespaceResources.FactoryArgs{
		DockerRepo:             "kubevirt",
		DockerTag:              version,
		DeployClusterResources: "true",
		ControllerImage:        "cdi-controller",
		ImporterImage:          "cdi-importer",
		ClonerImage:            "cdi-cloner",
		APIServerImage:         "cdi-apiserver",
		UploadProxyImage:       "cdi-uploadproxy",
		UploadServerImage:      "cdi-uploadserver",
		Verbosity:              "1",
		PullPolicy:             "Always",
		Namespace:              namespace,
	}

	return &ReconcileCDI{
		client:         client,
		scheme:         scheme.Scheme,
		namespace:      namespace,
		clusterArgs:    clusterArgs,
		namespacedArgs: namespacedArgs,
	}
}

func createUploadProxyCACertSecret(client realClient.Client) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cdi-upload-proxy-ca-key",
			Namespace: "cdi",
		},
		Data: map[string][]byte{
			"tls.crt": []byte("hello world"),
		},
	}

	err := client.Create(context.TODO(), secret)
	if errors.IsAlreadyExists(err) {
		return
	}
	Expect(err).ToNot(HaveOccurred())
}
