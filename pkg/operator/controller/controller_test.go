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
	generrors "errors"
	"fmt"

	utils "kubevirt.io/containerized-data-importer/pkg/operator/resources/utils"

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
	cluster "kubevirt.io/containerized-data-importer/pkg/operator/resources/cluster"
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

type modifyResource func(toModify runtime.Object) (runtime.Object, runtime.Object, error)
type isModifySubject func(resource runtime.Object) bool
type isUpgraded func(postUpgradeObj runtime.Object, deisredObj runtime.Object) bool

type createUnusedObject func() (runtime.Object, error)

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

				args.scc, err = getSCC(args.client, args.scc)
				Expect(err).ToNot(HaveOccurred())

				for _, eu := range []string{"system:serviceaccount:cdi:cdi-sa"} {
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

	Describe("Upgrading CDI", func() {

		DescribeTable("check detects upgrade correctly", func(prevVersion, newVersion string, shouldUpgrade bool) {
			registry := "kubevirt"

			//verify on int version is set
			args := createFromArgs("cdi", newVersion, registry)
			doReconcile(args)

			Expect(args.cdi.Status.ObservedVersion).Should(Equal(newVersion))
			Expect(args.cdi.Status.OperatorVersion).Should(Equal(newVersion))
			Expect(args.cdi.Status.TargetVersion).Should(Equal(newVersion))
			Expect(args.cdi.Status.Phase).Should(Equal(cdiviaplha1.CDIPhaseDeployed))

			//Modify CRD to be of previousVersion
			err := args.reconciler.crSetVersion(args.cdi, prevVersion, registry)
			Expect(err).ToNot(HaveOccurred())

			doReconcile(args)

			if shouldUpgrade {
				//verify upgraded has started
				Expect(args.cdi.Status.OperatorVersion).Should(Equal(prevVersion))
				Expect(args.cdi.Status.ObservedVersion).Should(Equal(prevVersion))
				Expect(args.cdi.Status.TargetVersion).Should(Equal(newVersion))
				Expect(args.cdi.Status.Phase).Should(Equal(cdiviaplha1.CDIPhaseDeploying))
			} else {
				//verify upgraded hasn't started
				Expect(args.cdi.Status.OperatorVersion).Should(Equal(prevVersion))
				Expect(args.cdi.Status.ObservedVersion).Should(Equal(prevVersion))
				Expect(args.cdi.Status.TargetVersion).Should(Equal(prevVersion))
				Expect(args.cdi.Status.Phase).Should(Equal(cdiviaplha1.CDIPhaseDeployed))
			}

			//change deployment to ready
			isReady := setDeploymentsReady(args)
			Expect(isReady).Should(Equal(true))

			//now should be upgraded
			if shouldUpgrade {
				//verify versions were updated
				Expect(args.cdi.Status.Phase).Should(Equal(cdiviaplha1.CDIPhaseDeployed))
				Expect(args.cdi.Status.OperatorVersion).Should(Equal(newVersion))
				Expect(args.cdi.Status.TargetVersion).Should(Equal(newVersion))
				Expect(args.cdi.Status.ObservedVersion).Should(Equal(newVersion))
			} else {
				//verify versions remained unchaged
				Expect(args.cdi.Status.Phase).Should(Equal(cdiviaplha1.CDIPhaseDeployed))
				Expect(args.cdi.Status.OperatorVersion).Should(Equal(prevVersion))
				Expect(args.cdi.Status.TargetVersion).Should(Equal(prevVersion))
				Expect(args.cdi.Status.ObservedVersion).Should(Equal(prevVersion))
			}
		},
			Entry("increasing semver ", "v1.9.5", "v1.10.0", true),
			Entry("decreasing semver", "v1.10.0", "v1.9.5", false),
			Entry("identical semver", "v1.10.0", "v1.10.0", false),
			Entry("invalid semver", "devel", "v1.9.5", true),
			Entry("increasing  semver no prefix", "1.9.5", "1.10.0", true),
			Entry("decreasing  semver no prefix", "1.10.0", "1.9.5", false),
			Entry("identical  semver no prefix", "1.10.0", "1.10.0", false),
			Entry("invalid  semver with prefix", "devel1.9.5", "devel1.9.5", false),
			Entry("invalid  semver no prefix", "devel", "1.9.5", true),
			Entry("no current no prefix", "", "invalid", false),
		)

		Describe("CDI CR deletion during upgrade", func() {
			Context("cr deletion during upgrade", func() {
				It("should delete CR if it is marked for deletion and not begin upgrade flow", func() {
					var args *args
					registry := "kubevirt"
					newVersion := "1.10.0"
					prevVersion := "1.9.5"

					args = createFromArgs("cdi", newVersion, registry)
					doReconcile(args)

					//set deployment to ready
					isReady := setDeploymentsReady(args)
					Expect(isReady).Should(Equal(true))

					//verify on int version is set
					Expect(args.cdi.Status.Phase).Should(Equal(cdiviaplha1.CDIPhaseDeployed))

					//Modify CRD to be of previousVersion
					args.reconciler.crSetVersion(args.cdi, prevVersion, registry)
					//marc CDI CR for deltetion
					args.cdi.SetDeletionTimestamp(&metav1.Time{time.Now()})
					err := args.client.Update(context.TODO(), args.cdi)
					Expect(err).ToNot(HaveOccurred())

					doReconcile(args)

					//verify the version cr is deleted and upgrade hasn't started
					Expect(args.cdi.Status.OperatorVersion).Should(Equal(prevVersion))
					Expect(args.cdi.Status.ObservedVersion).Should(Equal(prevVersion))
					Expect(args.cdi.Status.TargetVersion).Should(Equal(prevVersion))
					Expect(args.cdi.Status.Phase).Should(Equal(cdiviaplha1.CDIPhaseDeleted))
				})

				It("should delete CR if it is marked for deletion during upgrade flow", func() {
					var args *args
					registry := "kubevirt"
					newVersion := "1.10.0"
					prevVersion := "1.9.5"

					args = createFromArgs("cdi", newVersion, registry)
					doReconcile(args)

					//verify on int version is set
					Expect(args.cdi.Status.Phase).Should(Equal(cdiviaplha1.CDIPhaseDeployed))

					//Modify CRD to be of previousVersion
					args.reconciler.crSetVersion(args.cdi, prevVersion, registry)
					err := args.client.Update(context.TODO(), args.cdi)
					Expect(err).ToNot(HaveOccurred())

					//begin upgrade
					doReconcile(args)

					//mark CDI CR for deltetion
					args.cdi.SetDeletionTimestamp(&metav1.Time{time.Now()})
					err = args.client.Update(context.TODO(), args.cdi)
					Expect(err).ToNot(HaveOccurred())

					doReconcile(args)

					//set deployment to ready
					isReady := setDeploymentsReady(args)
					Expect(isReady).Should(Equal(false))

					doReconcile(args)
					//verify the version cr is marked as deleted
					Expect(args.cdi.Status.Phase).Should(Equal(cdiviaplha1.CDIPhaseDeleted))
				})
				It("should cleanup unused resources from previos version if CDI cr if it is marked for deletion during upgrade flow", func() {
					var args *args
					registry := "kubevirt"
					newVersion := "1.10.0"
					prevVersion := "1.9.5"

					args = createFromArgs("cdi", newVersion, registry)
					doReconcile(args)

					//verify on int version is set
					Expect(args.cdi.Status.Phase).Should(Equal(cdiviaplha1.CDIPhaseDeployed))

					//Modify CRD to be of previousVersion
					args.reconciler.crSetVersion(args.cdi, prevVersion, registry)
					err := args.client.Update(context.TODO(), args.cdi)
					Expect(err).ToNot(HaveOccurred())

					//begin upgrade
					doReconcile(args)

					//mark CDI CR for deletetion
					args.cdi.SetDeletionTimestamp(&metav1.Time{time.Now()})
					err = args.client.Update(context.TODO(), args.cdi)
					Expect(err).ToNot(HaveOccurred())

					//add object unused by new version
					unusedObj := utils.CreateDeployment("fake-cdi-deployment", "app", "containerized-data-importer", "fake-sa", int32(1))
					err = args.client.Create(context.TODO(), unusedObj)
					Expect(err).ToNot(HaveOccurred())

					doReconcile(args)

					//set deployment to ready
					isReady := setDeploymentsReady(args)
					Expect(isReady).Should(Equal(false))

					doReconcile(args)

					key := client.ObjectKey{
						Namespace: unusedObj.GetNamespace(),
						Name:      unusedObj.GetName(),
					}
					err = args.client.Get(context.TODO(), key, unusedObj)
					Expect(err).To(HaveOccurred())
					Expect(errors.IsNotFound(err)).Should(Equal(true))

					//verify the version cr is marked as deleted
					Expect(args.cdi.Status.Phase).Should(Equal(cdiviaplha1.CDIPhaseDeleted))
				})

			})
		})

		DescribeTable("Updates objects on upgrade", func(
			modify modifyResource,
			tomodify isModifySubject,
			upgraded isUpgraded) {

			var args *args
			registry := "kubevirt"
			newVersion := "1.10.0"
			prevVersion := "1.9.5"

			args = createFromArgs("cdi", newVersion, registry)
			doReconcile(args)

			//verify on int version is set
			Expect(args.cdi.Status.Phase).Should(Equal(cdiviaplha1.CDIPhaseDeployed))

			//Modify CRD to be of previousVersion
			args.reconciler.crSetVersion(args.cdi, prevVersion, registry)
			err := args.client.Update(context.TODO(), args.cdi)
			Expect(err).ToNot(HaveOccurred())

			//find the resource to modify
			oOriginal, oModified, err := getModifiedResource(args.reconciler, modify, tomodify)
			Expect(err).ToNot(HaveOccurred())

			//update object via client, with curObject
			err = args.client.Update(context.TODO(), oModified)
			Expect(err).ToNot(HaveOccurred())

			//verify object is modified
			storedObj, err := getObject(args.client, oModified)
			Expect(err).ToNot(HaveOccurred())

			Expect(reflect.DeepEqual(storedObj, oModified)).Should(Equal(true))

			doReconcile(args)

			//verify upgraded has started
			Expect(args.cdi.Status.Phase).Should(Equal(cdiviaplha1.CDIPhaseDeploying))

			//change deployment to ready
			isReady := setDeploymentsReady(args)
			Expect(isReady).Should(Equal(true))

			doReconcile(args)
			Expect(args.cdi.Status.Phase).Should(Equal(cdiviaplha1.CDIPhaseDeployed))

			//verify that stored object equals to object in getResources
			storedObj, err = getObject(args.client, oModified)
			Expect(err).ToNot(HaveOccurred())

			Expect(upgraded(storedObj, oOriginal)).Should(Equal(true))

		},
			//Deployment update
			Entry("verify - deployment updated on upgrade - annotation changed",
				func(toModify runtime.Object) (runtime.Object, runtime.Object, error) { //Modify
					deploymentOrig, ok := toModify.(*appsv1.Deployment)
					if !ok {
						return toModify, toModify, generrors.New(fmt.Sprint("wrong type"))
					}
					deployment := deploymentOrig.DeepCopy()
					deployment.Annotations = map[string]string{
						"fake.anno.1": "fakeannotation1",
						"fake.anno.2": "fakeannotation2",
						"fake.anno.3": "fakeannotation3",
					}
					return toModify, deployment, nil
				},
				func(resource runtime.Object) bool { //find resource for test
					//return true if object is the one we want to test
					_, ok := resource.(*appsv1.Deployment)
					return ok
				},
				func(postUpgradeObj runtime.Object, deisredObj runtime.Object) bool { //check resource was upgraded
					//return true if postUpgrade has teh same fields as desired
					postDep, ok := postUpgradeObj.(*appsv1.Deployment)
					if !ok {
						return false
					}

					desiredDep, ok := deisredObj.(*appsv1.Deployment)
					if !ok {
						return false
					}

					for key, ann := range desiredDep.Annotations {
						if postDep.Annotations[key] != ann {
							return false
						}
					}

					if len(desiredDep.Annotations) > len(postDep.Annotations) {
						return false
					}

					return true
				}),
			Entry("verify - deployment updated on upgrade - labels changed",
				func(toModify runtime.Object) (runtime.Object, runtime.Object, error) { //Modify
					deploymentOrig, ok := toModify.(*appsv1.Deployment)
					if !ok {
						return toModify, toModify, generrors.New(fmt.Sprint("wrong type"))
					}
					deployment := deploymentOrig.DeepCopy()
					deployment.Labels = map[string]string{
						"fake.label.1": "fakelabel1",
						"fake.label.2": "fakelabel2",
						"fake.label.3": "fakelabel3",
					}
					return toModify, deployment, nil
				},
				func(resource runtime.Object) bool { //find resource for test
					//return true if object is the one we want to test
					_, ok := resource.(*appsv1.Deployment)
					return ok
				},
				func(postUpgradeObj runtime.Object, deisredObj runtime.Object) bool { //check resource was upgraded
					//return true if postUpgrade has teh same fields as desired
					postDep, ok := postUpgradeObj.(*appsv1.Deployment)
					if !ok {
						return false
					}

					desiredDep, ok := deisredObj.(*appsv1.Deployment)
					if !ok {
						return false
					}

					for key, label := range desiredDep.Labels {
						if postDep.Labels[key] != label {
							return false
						}
					}

					if len(desiredDep.Labels) > len(postDep.Labels) {
						return false
					}

					return true
				}),
			Entry("verify - deployment updated on upgrade - deployment spec changed - modify container",
				func(toModify runtime.Object) (runtime.Object, runtime.Object, error) { //Modify
					deploymentOrig, ok := toModify.(*appsv1.Deployment)
					if !ok {
						return toModify, toModify, generrors.New(fmt.Sprint("wrong type"))
					}
					deployment := deploymentOrig.DeepCopy()

					containers := deployment.Spec.Template.Spec.Containers
					containers[0].Env = []corev1.EnvVar{
						{
							Name:  "FAKE_ENVVAR",
							Value: fmt.Sprintf("%s/%s:%s", "fake_repo", "importerImage", "tag"),
						},
					}

					return toModify, deployment, nil
				},
				func(resource runtime.Object) bool { //find resource for test
					//search for cdi-deployment - to test ENV virables change
					deployment, ok := resource.(*appsv1.Deployment)
					if !ok {
						return false
					}
					if deployment.Name == "cdi-deployment" {
						return true
					}
					return false
				},
				func(postUpgradeObj runtime.Object, deisredObj runtime.Object) bool { //check resource was upgraded
					//return true if postUpgrade has teh same fields as desired
					postDep, ok := postUpgradeObj.(*appsv1.Deployment)
					if !ok {
						return false
					}

					desiredDep, ok := deisredObj.(*appsv1.Deployment)
					if !ok {
						return false
					}

					for key, envVar := range desiredDep.Spec.Template.Spec.Containers[0].Env {
						if postDep.Spec.Template.Spec.Containers[0].Env[key].Name != envVar.Name {
							return false
						}
					}

					if len(desiredDep.Spec.Template.Spec.Containers[0].Env) != len(postDep.Spec.Template.Spec.Containers[0].Env) {
						return false
					}

					return true
				}),
			Entry("verify - deployment updated on upgrade - deployment spec changed - add new container",
				func(toModify runtime.Object) (runtime.Object, runtime.Object, error) { //Modify
					deploymentOrig, ok := toModify.(*appsv1.Deployment)
					if !ok {
						return toModify, toModify, generrors.New(fmt.Sprint("wrong type"))
					}
					deployment := deploymentOrig.DeepCopy()

					containers := deployment.Spec.Template.Spec.Containers
					container := corev1.Container{
						Name:            "FAKE_CONTAINER",
						Image:           fmt.Sprintf("%s/%s:%s", "fake-repo", "fake-image", "fake-tag"),
						ImagePullPolicy: "FakePullPolicy",
						Args:            []string{"-v=10"},
					}
					containers = append(containers, container)

					return toModify, deployment, nil
				},
				func(resource runtime.Object) bool { //find resource for test
					//search for cdi-deployment - to test container change
					deployment, ok := resource.(*appsv1.Deployment)
					if !ok {
						return false
					}
					if deployment.Name == "cdi-deployment" {
						return true
					}
					return false
				},
				func(postUpgradeObj runtime.Object, deisredObj runtime.Object) bool { //check resource was upgraded
					//return true if postUpgrade has teh same fields as desired
					postDep, ok := postUpgradeObj.(*appsv1.Deployment)
					if !ok {
						return false
					}

					desiredDep, ok := deisredObj.(*appsv1.Deployment)
					if !ok {
						return false
					}

					for key, container := range desiredDep.Spec.Template.Spec.Containers {
						if postDep.Spec.Template.Spec.Containers[key].Name != container.Name {
							return false
						}
					}

					if len(desiredDep.Spec.Template.Spec.Containers) > len(postDep.Spec.Template.Spec.Containers) {
						return false
					}

					return true
				}),
			Entry("verify - deployment updated on upgrade - deployment spec changed - remove existing container",
				func(toModify runtime.Object) (runtime.Object, runtime.Object, error) { //Modify
					deploymentOrig, ok := toModify.(*appsv1.Deployment)
					if !ok {
						return toModify, toModify, generrors.New(fmt.Sprint("wrong type"))
					}
					deployment := deploymentOrig.DeepCopy()

					deployment.Spec.Template.Spec.Containers = nil

					return toModify, deployment, nil
				},
				func(resource runtime.Object) bool { //find resource for test
					//search for cdi-deployment - to test container change
					deployment, ok := resource.(*appsv1.Deployment)
					if !ok {
						return false
					}
					if deployment.Name == "cdi-deployment" {
						return true
					}
					return false
				},
				func(postUpgradeObj runtime.Object, deisredObj runtime.Object) bool { //check resource was upgraded
					//return true if postUpgrade has teh same fields as desired
					postDep, ok := postUpgradeObj.(*appsv1.Deployment)
					if !ok {
						return false
					}

					desiredDep, ok := deisredObj.(*appsv1.Deployment)
					if !ok {
						return false
					}

					return (len(postDep.Spec.Template.Spec.Containers) == len(desiredDep.Spec.Template.Spec.Containers))
				}),

			//Services update
			Entry("verify - services updated on upgrade - annotation changed",
				func(toModify runtime.Object) (runtime.Object, runtime.Object, error) { //Modify
					serviceOrig, ok := toModify.(*corev1.Service)
					if !ok {
						return toModify, toModify, generrors.New(fmt.Sprint("wrong type"))
					}
					service := serviceOrig.DeepCopy()
					service.Annotations = map[string]string{
						"fake.anno.1": "fakeannotation1",
						"fake.anno.2": "fakeannotation2",
						"fake.anno.3": "fakeannotation3",
					}
					return toModify, service, nil
				},
				func(resource runtime.Object) bool { //find resource for test
					//return true if object is the one we want to test
					_, ok := resource.(*corev1.Service)
					return ok
				},
				func(postUpgradeObj runtime.Object, deisredObj runtime.Object) bool { //check resource was upgraded
					//return true if postUpgrade has teh same fields as desired
					post, ok := postUpgradeObj.(*corev1.Service)
					if !ok {
						return false
					}

					desired, ok := deisredObj.(*corev1.Service)
					if !ok {
						return false
					}

					for key, ann := range desired.Annotations {
						if post.Annotations[key] != ann {
							return false
						}
					}

					if len(desired.Annotations) > len(post.Annotations) {
						return false
					}
					return true
				}),

			Entry("verify - services updated on upgrade - label changed",
				func(toModify runtime.Object) (runtime.Object, runtime.Object, error) { //Modify
					serviceOrig, ok := toModify.(*corev1.Service)
					if !ok {
						return toModify, toModify, generrors.New(fmt.Sprint("wrong type"))
					}
					service := serviceOrig.DeepCopy()
					service.Labels = map[string]string{
						"fake.label.1": "fakelabel1",
						"fake.label.2": "fakelabel2",
						"fake.label.3": "fakelabel3",
					}
					return toModify, service, nil
				},
				func(resource runtime.Object) bool { //find resource for test
					//return true if object is the one we want to test
					_, ok := resource.(*corev1.Service)
					return ok
				},
				func(postUpgradeObj runtime.Object, deisredObj runtime.Object) bool { //check resource was upgraded
					//return true if postUpgrade has teh same fields as desired
					post, ok := postUpgradeObj.(*corev1.Service)
					if !ok {
						return false
					}

					desired, ok := deisredObj.(*corev1.Service)
					if !ok {
						return false
					}

					for key, label := range desired.Labels {
						if post.Labels[key] != label {
							return false
						}
					}

					if len(desired.Labels) > len(post.Labels) {
						return false
					}

					return true
				}),

			Entry("verify - services updated on upgrade - service port changed",
				func(toModify runtime.Object) (runtime.Object, runtime.Object, error) { //Modify
					serviceOrig, ok := toModify.(*corev1.Service)
					if !ok {
						return toModify, toModify, generrors.New(fmt.Sprint("wrong type"))
					}
					service := serviceOrig.DeepCopy()
					service.Spec.Ports = []corev1.ServicePort{
						{
							Port:     999999,
							Protocol: corev1.ProtocolUDP,
						},
					}
					return toModify, service, nil
				},
				func(resource runtime.Object) bool { //find resource for test
					//return true if object is the one we want to test
					_, ok := resource.(*corev1.Service)
					return ok
				},
				func(postUpgradeObj runtime.Object, deisredObj runtime.Object) bool { //check resource was upgraded
					//return true if postUpgrade has teh same fields as desired
					post, ok := postUpgradeObj.(*corev1.Service)
					if !ok {
						return false
					}

					desired, ok := deisredObj.(*corev1.Service)
					if !ok {
						return false
					}

					for key, port := range desired.Spec.Ports {
						if post.Spec.Ports[key].Port != port.Port {
							return false
						}
					}

					if len(desired.Spec.Ports) != len(post.Spec.Ports) {
						return false
					}

					return true
				}),

			//CRD update
			// - update CRD label
			// - update CRD annotation
			// - update CRD version
			// - update CRD spec
			// - update CRD status
			// - add new CRD
			// -

			//RBAC update
			// - update RoleBinding/ClusterRoleBinding
			// - Update Role/ClusterRole

			//ServiceAccount upgrade
			// - update ServiceAccount SCC
			// - update ServiceAccount Labels/Annotations

		) //updates objects on upgrade

		DescribeTable("Removes unused objects on upgrade", func(
			createObj createUnusedObject) {

			var args *args
			registry := "kubevirt"
			newVersion := "1.10.0"
			prevVersion := "1.9.5"

			args = createFromArgs("cdi", newVersion, registry)
			doReconcile(args)

			//verify on int version is set
			Expect(args.cdi.Status.Phase).Should(Equal(cdiviaplha1.CDIPhaseDeployed))

			//Modify CRD to be of previousVersion
			args.reconciler.crSetVersion(args.cdi, prevVersion, registry)
			err := args.client.Update(context.TODO(), args.cdi)
			Expect(err).ToNot(HaveOccurred())

			unusedObj, err := createObj()
			Expect(err).ToNot(HaveOccurred())

			//add unused object via client, with curObject
			err = args.client.Create(context.TODO(), unusedObj)
			Expect(err).ToNot(HaveOccurred())

			doReconcile(args)

			//verify upgraded has started
			Expect(args.cdi.Status.Phase).Should(Equal(cdiviaplha1.CDIPhaseDeploying))

			//verify unused exists before upgrade is done
			_, err = getObject(args.client, unusedObj)
			Expect(err).ToNot(HaveOccurred())

			//change deployment to ready
			isReady := setDeploymentsReady(args)
			Expect(isReady).Should(Equal(true))

			doReconcile(args)
			Expect(args.cdi.Status.Phase).Should(Equal(cdiviaplha1.CDIPhaseDeployed))

			//verify that object no longer exists after upgrade
			_, err = getObject(args.client, unusedObj)
			Expect(errors.IsNotFound(err)).Should(Equal(true))

		},

			Entry("verify - unused deployment deleted",
				func() (runtime.Object, error) {
					deployment := utils.CreateDeployment("fake-cdi-deployment", "app", "containerized-data-importer", "fake-sa", int32(1))
					return deployment, nil
				}),
			Entry("verify - unused service deleted",
				func() (runtime.Object, error) {
					service := utils.CreateService("fake-cdi-service", "fake-service", "fake")
					return service, nil
				}),
			Entry("verify - unused sa deleted",
				func() (runtime.Object, error) {
					sa := utils.CreateServiceAccount("fake-cdi-sa")
					return sa, nil
				}),

			Entry("verify - unused crd deleted",
				func() (runtime.Object, error) {
					crd := &extv1beta1.CustomResourceDefinition{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "apiextensions.k8s.io/v1beta1",
							Kind:       "CustomResourceDefinition",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "fake.cdis.cdi.kubevirt.io",
							Labels: map[string]string{
								"operator.cdi.kubevirt.io": "",
							},
						},
						Spec: extv1beta1.CustomResourceDefinitionSpec{
							Group:   "cdi.kubevirt.io",
							Version: "v1alpha1",
							Scope:   "Cluster",

							Versions: []extv1beta1.CustomResourceDefinitionVersion{
								{
									Name:    "v1alpha1",
									Served:  true,
									Storage: true,
								},
							},
							Names: extv1beta1.CustomResourceDefinitionNames{
								Kind:     "FakeCDI",
								ListKind: "FakeCDIList",
								Plural:   "fakecdis",
								Singular: "fakecdi",
								Categories: []string{
									"all",
								},
								ShortNames: []string{"fakecdi", "fakecdis"},
							},

							AdditionalPrinterColumns: []extv1beta1.CustomResourceColumnDefinition{
								{Name: "Age", Type: "date", JSONPath: ".metadata.creationTimestamp"},
								{Name: "Phase", Type: "string", JSONPath: ".status.phase"},
							},
						},
					}
					return crd, nil
				}),

			Entry("verify - unused role deleted",
				func() (runtime.Object, error) {
					role := utils.CreateRole("fake-role")
					return role, nil
				}),

			Entry("verify - unused role binding deleted",
				func() (runtime.Object, error) {
					role := utils.CreateRoleBinding("fake-role", "fake-role", "fake-role", "fake-role")
					return role, nil
				}),
			Entry("verify - unused cluster role deleted",
				func() (runtime.Object, error) {
					role := cluster.CreateClusterRole("fake-cluster-role")
					return role, nil
				}),
			Entry("verify - unused cluster role binding deleted",
				func() (runtime.Object, error) {
					role := cluster.CreateClusterRoleBinding("fake-cluster-role", "fake-cluster-role", "fake-cluster-role", "fake-cluster-role")
					return role, nil
				}),
		)

	})
})

func getModifiedResource(reconciler *ReconcileCDI, modify modifyResource, tomodify isModifySubject) (runtime.Object, runtime.Object, error) {
	resources, err := getAllResources(reconciler)
	if err != nil {
		return nil, nil, err
	}

	//find the resource to modify
	var orig runtime.Object
	for _, resource := range resources {
		if tomodify(resource) {
			orig = resource
			break
		}
	}
	//apply modify function on resource and return modified one
	return modify(orig)
}

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

func setDeploymentsReady(args *args) bool {
	resources, err := getAllResources(args.reconciler)
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
		if d.Spec.Replicas != nil {
			d.Status.Replicas = *d.Spec.Replicas
			d.Status.ReadyReplicas = d.Status.Replicas
			err = args.client.Update(context.TODO(), d)
			Expect(err).ToNot(HaveOccurred())
		}

		doReconcile(args)

		if len(args.cdi.Status.Conditions) == 1 &&
			args.cdi.Status.Conditions[0].Type == cdiviaplha1.CDIConditionRunning &&
			args.cdi.Status.Conditions[0].Status == corev1.ConditionTrue {
			running = true
		}
	}

	return running
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

func createFromArgs(namespace, version, repo string) *args {
	cdi := createCDI("cdi", "good uid")
	scc := createSCC()
	client := createClient(cdi, scc)
	reconciler := createReconcilerWithVersion(client, version, repo, namespace)

	return &args{
		cdi:        cdi,
		scc:        scc,
		client:     client,
		reconciler: reconciler,
	}
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
	return &secv1.SecurityContextConstraints{
		ObjectMeta: metav1.ObjectMeta{
			Name: "anyuid",
		},
		Users: []string{},
	}
}

func createReconcilerWithVersion(client realClient.Client, version, repo, namespace string) *ReconcileCDI {
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

	r := &ReconcileCDI{
		client:         client,
		scheme:         scheme.Scheme,
		namespace:      namespace,
		clusterArgs:    clusterArgs,
		namespacedArgs: namespacedArgs,
		callbacks:      make(map[reflect.Type][]ReconcileCallback),
	}

	addReconcileCallbacks(r)

	return r
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
