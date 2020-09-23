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
	"os"
	"reflect"
	"time"

	"kubevirt.io/controller-lifecycle-operator-sdk/pkg/sdk/callbacks"

	sdkapi "kubevirt.io/controller-lifecycle-operator-sdk/pkg/sdk/api"
	sdkr "kubevirt.io/controller-lifecycle-operator-sdk/pkg/sdk/reconciler"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	routev1 "github.com/openshift/api/route/v1"
	secv1 "github.com/openshift/api/security/v1"
	conditions "github.com/openshift/custom-resource-status/conditions/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	apiregistrationv1beta1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	realClient "sigs.k8s.io/controller-runtime/pkg/client"
	fakeClient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	clusterResources "kubevirt.io/containerized-data-importer/pkg/operator/resources/cluster"
	namespaceResources "kubevirt.io/containerized-data-importer/pkg/operator/resources/namespaced"
	utils "kubevirt.io/containerized-data-importer/pkg/operator/resources/utils"
)

const (
	version                   = "v1.5.0"
	cdiNamespace              = "cdi"
	configMapName             = "cdi-config"
	insecureRegistryConfigMap = "cdi-insecure-registries"

	normalCreateSuccess              = "Normal CreateResourceSuccess Successfully created resource"
	normalCreateEnsured              = "Normal CreateResourceSuccess Successfully ensured"
	normalDeleteResourceSuccess      = "Normal DeleteResourceSuccess Deleted deployment cdi-deployment successfully"
	normalDeleteResourceSuccesWorker = "Normal DeleteResourceSuccess Deleted worker resources successfully"
)

type args struct {
	cdi        *cdiv1.CDI
	client     client.Client
	reconciler *ReconcileCDI
}

var (
	envVars = map[string]string{
		"OPERATOR_VERSION":         version,
		"DEPLOY_CLUSTER_RESOURCES": "true",
		"CONTROLLER_IMAGE":         "kubevirt/cdi-controller",
		"IMPORTER_IMAGE":           "kubevirt/cdi-importer",
		"CLONER_IMAGE":             "kubevirt/cdi-cloner",
		"UPLOAD_PROXY_IMAGE":       "ckubevirt/di-uploadproxy",
		"UPLOAD_SERVER_IMAGE":      "kubevirt/cdi-uploadserver",
		"APISERVER_IMAGE":          "kubevirt/cdi-apiserver",
		"VERBOSITY":                "1",
		"PULL_POLICY":              "Always",
	}
)

func init() {
	cdiv1.AddToScheme(scheme.Scheme)
	extv1.AddToScheme(scheme.Scheme)
	apiregistrationv1beta1.AddToScheme(scheme.Scheme)
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

				err = cdiv1.AddToScheme(mgr.GetScheme())
				Expect(err).ToNot(HaveOccurred())

				err = extv1.AddToScheme(mgr.GetScheme())
				Expect(err).ToNot(HaveOccurred())

				err = secv1.Install(mgr.GetScheme())
				Expect(err).ToNot(HaveOccurred())

				mgr.GetClient().Create(context.TODO(), createCDI("cdi", "good uid"), &client.CreateOptions{})

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
		Entry("CDR type", &extv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "crd"}}),
		Entry("SSC type", &secv1.SecurityContextConstraints{ObjectMeta: metav1.ObjectMeta{Name: "scc"}}),
		Entry("Route type", &routev1.Route{ObjectMeta: metav1.ObjectMeta{Name: "route"}}),
	)

	Describe("Deploying CDI", func() {
		Context("CDI lifecycle", func() {
			It("should get deployed", func() {
				args := createArgs()
				doReconcile(args)
				setDeploymentsReady(args)

				Expect(args.cdi.Status.OperatorVersion).Should(Equal(version))
				Expect(args.cdi.Status.TargetVersion).Should(Equal(version))
				Expect(args.cdi.Status.ObservedVersion).Should(Equal(version))

				Expect(args.cdi.Status.Conditions).Should(HaveLen(3))
				Expect(conditions.IsStatusConditionTrue(args.cdi.Status.Conditions, conditions.ConditionAvailable)).To(BeTrue())
				Expect(conditions.IsStatusConditionFalse(args.cdi.Status.Conditions, conditions.ConditionProgressing)).To(BeTrue())
				Expect(conditions.IsStatusConditionFalse(args.cdi.Status.Conditions, conditions.ConditionDegraded)).To(BeTrue())

				Expect(args.cdi.Finalizers).Should(HaveLen(1))

				validateEvents(args.reconciler, createReadyEventValidationMap())
			})

			It("should create configmap", func() {
				args := createArgs()
				doReconcile(args)

				cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: cdiNamespace, Name: configMapName}}
				obj, err := getObject(args.client, cm)
				Expect(err).ToNot(HaveOccurred())

				cm = obj.(*corev1.ConfigMap)
				Expect(cm.OwnerReferences[0].UID).Should(Equal(args.cdi.UID))
				validateEvents(args.reconciler, createNotReadyEventValidationMap())
			})

			It("should create prometheus service", func() {
				args := createArgs()
				doReconcile(args)

				svc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: cdiNamespace, Name: common.PrometheusServiceName}}
				obj, err := getObject(args.client, svc)
				Expect(err).ToNot(HaveOccurred())

				svc = obj.(*corev1.Service)
				Expect(svc.OwnerReferences[0].UID).Should(Equal(args.cdi.UID))
			})

			It("should create requeue when configmap exists with another owner", func() {
				cm := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: cdiNamespace,
						Name:      configMapName,
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: cdiv1.SchemeGroupVersion.String(),
								Kind:       "CDI",
								Name:       "cdi",
								UID:        "badUID",
							},
						},
					},
				}

				args := createArgs()

				err := args.client.Create(context.TODO(), cm)
				Expect(err).ToNot(HaveOccurred())

				doReconcileRequeue(args)
			})

			It("should create requeue when configmap has deletion timestamp", func() {
				t := metav1.Now()
				cm := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Namespace:         cdiNamespace,
						Name:              configMapName,
						DeletionTimestamp: &t,
					},
				}

				args := createArgs()

				err := args.client.Create(context.TODO(), cm)
				Expect(err).ToNot(HaveOccurred())

				doReconcileRequeue(args)
			})

			It("should create requeue when a resource exists", func() {
				args := createArgs()
				resources, err := getAllResources(args.reconciler)
				Expect(err).ToNot(HaveOccurred())

				err = args.client.Create(context.TODO(), resources[0])
				Expect(err).ToNot(HaveOccurred())

				doReconcileRequeue(args)
			})

			It("should be in securitycontextconstraint", func() {
				args := createArgs()
				doReconcile(args)
				Expect(setDeploymentsReady(args)).To(BeTrue())

				scc := &secv1.SecurityContextConstraints{
					ObjectMeta: metav1.ObjectMeta{
						Name: "containerized-data-importer",
					},
				}

				scc, err := getSCC(args.client, scc)
				Expect(err).ToNot(HaveOccurred())

				for _, eu := range []string{"system:serviceaccount:cdi:cdi-sa"} {
					found := false
					for _, au := range scc.Users {
						if eu == au {
							found = true
						}
					}
					Expect(found).To(BeTrue())
				}
				validateEvents(args.reconciler, createReadyEventValidationMap())
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
				validateEvents(args.reconciler, createNotReadyEventValidationMap())
			})

			It("should become ready", func() {
				args := createArgs()
				doReconcile(args)
				Expect(setDeploymentsReady(args)).To(BeTrue())

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
				Expect(route.Spec.TLS.DestinationCACertificate).Should(Equal(testCertData))
				validateEvents(args.reconciler, createReadyEventValidationMap())
			})

			It("can become become ready, un-ready, and ready again", func() {
				var deployment *appsv1.Deployment

				args := createArgs()
				doReconcile(args)

				resources, err := getAllResources(args.reconciler)
				Expect(err).ToNot(HaveOccurred())

				for _, r := range resources {
					d, ok := r.(*appsv1.Deployment)
					if !ok {
						continue
					}

					dd, err := getDeployment(args.client, d)
					Expect(err).ToNot(HaveOccurred())

					dd.Status.Replicas = *dd.Spec.Replicas
					dd.Status.ReadyReplicas = dd.Status.Replicas

					err = args.client.Update(context.TODO(), dd)
					Expect(err).ToNot(HaveOccurred())
				}

				doReconcile(args)

				Expect(args.cdi.Status.Conditions).Should(HaveLen(3))
				Expect(conditions.IsStatusConditionTrue(args.cdi.Status.Conditions, conditions.ConditionAvailable)).To(BeTrue())
				Expect(conditions.IsStatusConditionFalse(args.cdi.Status.Conditions, conditions.ConditionProgressing)).To(BeTrue())
				Expect(conditions.IsStatusConditionFalse(args.cdi.Status.Conditions, conditions.ConditionDegraded)).To(BeTrue())

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

				Expect(args.cdi.Status.Conditions).Should(HaveLen(3))
				Expect(conditions.IsStatusConditionTrue(args.cdi.Status.Conditions, conditions.ConditionAvailable)).To(BeTrue())
				Expect(conditions.IsStatusConditionFalse(args.cdi.Status.Conditions, conditions.ConditionProgressing)).To(BeTrue())
				// Application should be degraded due to missing deployment pods (set to 0)
				Expect(conditions.IsStatusConditionTrue(args.cdi.Status.Conditions, conditions.ConditionDegraded)).To(BeTrue())

				deployment, err = getDeployment(args.client, deployment)
				Expect(err).ToNot(HaveOccurred())
				deployment.Status.ReadyReplicas = deployment.Status.Replicas
				err = args.client.Update(context.TODO(), deployment)
				Expect(err).ToNot(HaveOccurred())

				doReconcile(args)

				Expect(args.cdi.Status.Conditions).Should(HaveLen(3))
				Expect(conditions.IsStatusConditionTrue(args.cdi.Status.Conditions, conditions.ConditionAvailable)).To(BeTrue())
				Expect(conditions.IsStatusConditionFalse(args.cdi.Status.Conditions, conditions.ConditionProgressing)).To(BeTrue())
				Expect(conditions.IsStatusConditionFalse(args.cdi.Status.Conditions, conditions.ConditionDegraded)).To(BeTrue())
				validateEvents(args.reconciler, createReadyEventValidationMap())
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
				validateEvents(args.reconciler, createNotReadyEventValidationMap())
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

				Expect(newInstance.Status.Phase).Should(Equal(sdkapi.PhaseError))
				Expect(newInstance.Status.Conditions).Should(HaveLen(3))
				Expect(conditions.IsStatusConditionFalse(newInstance.Status.Conditions, conditions.ConditionAvailable)).To(BeTrue())
				Expect(conditions.IsStatusConditionFalse(newInstance.Status.Conditions, conditions.ConditionProgressing)).To(BeTrue())
				Expect(conditions.IsStatusConditionTrue(newInstance.Status.Conditions, conditions.ConditionDegraded)).To(BeTrue())
				validateEvents(args.reconciler, createErrorCDIEventValidationMap())
			})

			It("should succeed when we delete CDI", func() {
				// create rando pod that should get deleted
				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo",
						Namespace: "default",
						Labels: map[string]string{
							"cdi.kubevirt.io": "cdi-upload-server",
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
				Expect(args.cdi.Status.Phase).Should(Equal(sdkapi.PhaseDeleted))

				_, err = getObject(args.client, pod)
				Expect(errors.IsNotFound(err)).To(BeTrue())
				validateEvents(args.reconciler, createDeleteCDIEventValidationMap())
			})
		})
	})

	DescribeTable("should allow override", func(o cdiOverride) {
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
		validateEvents(args.reconciler, createNotReadyEventValidationMap())
	},
		Entry("Pull override", &pullOverride{corev1.PullNever}),
	)

	Describe("Upgrading CDI", func() {

		DescribeTable("check detects upgrade correctly", func(prevVersion, newVersion string, shouldUpgrade, shouldError bool) {
			//verify on int version is set
			args := createFromArgs(newVersion)
			doReconcile(args)
			setDeploymentsReady(args)

			Expect(args.cdi.Status.ObservedVersion).Should(Equal(newVersion))
			Expect(args.cdi.Status.OperatorVersion).Should(Equal(newVersion))
			Expect(args.cdi.Status.TargetVersion).Should(Equal(newVersion))
			Expect(args.cdi.Status.Phase).Should(Equal(sdkapi.PhaseDeployed))

			//Modify CRD to be of previousVersion
			err := crSetVersion(args.reconciler.reconciler, args.cdi, prevVersion)
			Expect(err).ToNot(HaveOccurred())

			if shouldError {
				doReconcileError(args)
				return
			}

			setDeploymentsDegraded(args)
			doReconcile(args)

			if shouldUpgrade {
				//verify upgraded has started
				Expect(args.cdi.Status.OperatorVersion).Should(Equal(newVersion))
				Expect(args.cdi.Status.ObservedVersion).Should(Equal(prevVersion))
				Expect(args.cdi.Status.TargetVersion).Should(Equal(newVersion))
				Expect(args.cdi.Status.Phase).Should(Equal(sdkapi.PhaseUpgrading))
			} else {
				//verify upgraded hasn't started
				Expect(args.cdi.Status.OperatorVersion).Should(Equal(prevVersion))
				Expect(args.cdi.Status.ObservedVersion).Should(Equal(prevVersion))
				Expect(args.cdi.Status.TargetVersion).Should(Equal(prevVersion))
				Expect(args.cdi.Status.Phase).Should(Equal(sdkapi.PhaseDeployed))
			}

			//change deployment to ready
			isReady := setDeploymentsReady(args)
			Expect(isReady).Should(Equal(true))

			//now should be upgraded
			if shouldUpgrade {
				//verify versions were updated
				Expect(args.cdi.Status.Phase).Should(Equal(sdkapi.PhaseDeployed))
				Expect(args.cdi.Status.OperatorVersion).Should(Equal(newVersion))
				Expect(args.cdi.Status.TargetVersion).Should(Equal(newVersion))
				Expect(args.cdi.Status.ObservedVersion).Should(Equal(newVersion))
			} else {
				//verify versions remained unchaged
				Expect(args.cdi.Status.Phase).Should(Equal(sdkapi.PhaseDeployed))
				Expect(args.cdi.Status.OperatorVersion).Should(Equal(prevVersion))
				Expect(args.cdi.Status.TargetVersion).Should(Equal(prevVersion))
				Expect(args.cdi.Status.ObservedVersion).Should(Equal(prevVersion))
			}
		},
			Entry("increasing semver ", "v1.9.5", "v1.10.0", true, false),
			Entry("decreasing semver", "v1.10.0", "v1.9.5", false, true),
			Entry("identical semver", "v1.10.0", "v1.10.0", false, false),
			Entry("invalid semver", "devel", "v1.9.5", true, false),
			Entry("increasing  semver no prefix", "1.9.5", "1.10.0", true, false),
			Entry("decreasing  semver no prefix", "1.10.0", "1.9.5", false, true),
			Entry("identical  semver no prefix", "1.10.0", "1.10.0", false, false),
			Entry("invalid  semver with prefix", "devel1.9.5", "devel1.9.5", false, false),
			Entry("invalid  semver no prefix", "devel", "1.9.5", true, false),
			/* having trouble making sense of this test "" should not be valid previous version
			Entry("no current no prefix", "", "invalid", false, false),
			*/
		)

		It("check detects upgrade w/o prev version", func() {
			prevVersion := ""
			newVersion := "v1.2.3"

			args := createFromArgs(prevVersion)
			doReconcile(args)
			setDeploymentsReady(args)

			Expect(args.cdi.Status.ObservedVersion).To(BeEmpty())
			Expect(args.cdi.Status.OperatorVersion).To(BeEmpty())
			Expect(args.cdi.Status.TargetVersion).To(BeEmpty())
			Expect(args.cdi.Status.Phase).Should(Equal(sdkapi.PhaseDeployed))

			args.reconciler.namespacedArgs.OperatorVersion = newVersion
			setDeploymentsDegraded(args)
			doReconcile(args)
			Expect(args.cdi.Status.ObservedVersion).To(BeEmpty())
			Expect(args.cdi.Status.OperatorVersion).Should(Equal(newVersion))
			Expect(args.cdi.Status.TargetVersion).Should(Equal(newVersion))
			Expect(args.cdi.Status.Phase).Should(Equal(sdkapi.PhaseUpgrading))

			//change deployment to ready
			isReady := setDeploymentsReady(args)
			Expect(isReady).To(BeTrue())
			Expect(args.cdi.Status.ObservedVersion).Should(Equal(newVersion))
			Expect(args.cdi.Status.OperatorVersion).Should(Equal(newVersion))
			Expect(args.cdi.Status.TargetVersion).Should(Equal(newVersion))
			Expect(args.cdi.Status.Phase).Should(Equal(sdkapi.PhaseDeployed))
		})

		Describe("CDI CR deletion during upgrade", func() {
			Context("cr deletion during upgrade", func() {
				It("should delete CR if it is marked for deletion and not begin upgrade flow", func() {
					newVersion := "1.10.0"
					prevVersion := "1.9.5"

					args := createFromArgs(newVersion)
					doReconcile(args)

					//set deployment to ready
					isReady := setDeploymentsReady(args)
					Expect(isReady).Should(Equal(true))

					//verify on int version is set
					Expect(args.cdi.Status.Phase).Should(Equal(sdkapi.PhaseDeployed))

					//Modify CRD to be of previousVersion
					crSetVersion(args.reconciler.reconciler, args.cdi, prevVersion)
					//marc CDI CR for deltetion
					args.cdi.SetDeletionTimestamp(&metav1.Time{Time: time.Now()})
					err := args.client.Update(context.TODO(), args.cdi)
					Expect(err).ToNot(HaveOccurred())

					doReconcile(args)

					//verify the version cr is deleted and upgrade hasn't started
					Expect(args.cdi.Status.OperatorVersion).Should(Equal(prevVersion))
					Expect(args.cdi.Status.ObservedVersion).Should(Equal(prevVersion))
					Expect(args.cdi.Status.TargetVersion).Should(Equal(prevVersion))
					Expect(args.cdi.Status.Phase).Should(Equal(sdkapi.PhaseDeleted))
				})

				It("should delete CR if it is marked for deletion during upgrade flow", func() {
					newVersion := "1.10.0"
					prevVersion := "1.9.5"

					args := createFromArgs(newVersion)
					doReconcile(args)
					setDeploymentsReady(args)

					//verify on int version is set
					Expect(args.cdi.Status.Phase).Should(Equal(sdkapi.PhaseDeployed))

					//Modify CRD to be of previousVersion
					crSetVersion(args.reconciler.reconciler, args.cdi, prevVersion)
					err := args.client.Update(context.TODO(), args.cdi)
					Expect(err).ToNot(HaveOccurred())
					setDeploymentsDegraded(args)

					//begin upgrade
					doReconcile(args)

					//mark CDI CR for deltetion
					args.cdi.SetDeletionTimestamp(&metav1.Time{Time: time.Now()})
					err = args.client.Update(context.TODO(), args.cdi)
					Expect(err).ToNot(HaveOccurred())

					doReconcile(args)
					//verify the version cr is marked as deleted
					Expect(args.cdi.Status.Phase).Should(Equal(sdkapi.PhaseDeleted))

					//verify events, this should include an upgrade event
					match := createDeleteCDIAfterReadyEventValidationMap()
					match["Normal UpgradeStarted Started upgrade to version 1.10.0"] = false
					validateEvents(args.reconciler, match)
				})
			})
		})

		DescribeTable("Updates objects on upgrade", func(
			modify modifyResource,
			tomodify isModifySubject,
			upgraded isUpgraded) {

			newVersion := "1.10.0"
			prevVersion := "1.9.5"

			args := createFromArgs(newVersion)
			doReconcile(args)
			setDeploymentsReady(args)

			//verify on int version is set
			Expect(args.cdi.Status.Phase).Should(Equal(sdkapi.PhaseDeployed))

			//Modify CRD to be of previousVersion
			crSetVersion(args.reconciler.reconciler, args.cdi, prevVersion)
			err := args.client.Update(context.TODO(), args.cdi)
			Expect(err).ToNot(HaveOccurred())

			setDeploymentsDegraded(args)

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
			Expect(args.cdi.Status.Phase).Should(Equal(sdkapi.PhaseUpgrading))

			//change deployment to ready
			isReady := setDeploymentsReady(args)
			Expect(isReady).Should(Equal(true))

			doReconcile(args)
			Expect(args.cdi.Status.Phase).Should(Equal(sdkapi.PhaseDeployed))

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
					deployment.Annotations["fake.anno.1"] = "fakeannotation1"
					deployment.Annotations["fake.anno.2"] = "fakeannotation2"
					deployment.Annotations["fake.anno.3"] = "fakeannotation3"
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

					delete(desiredDep.Annotations, lastAppliedConfigAnnotation)

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
					deployment.Labels["fake.label.1"] = "fakelabel1"
					deployment.Labels["fake.label.2"] = "fakelabel2"
					deployment.Labels["fake.label.3"] = "fakelabel3"
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
					deployment.Spec.Template.Spec.Containers = append(containers, container)

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
					service.Annotations["fake.anno.1"] = "fakeannotation1"
					service.Annotations["fake.anno.2"] = "fakeannotation2"
					service.Annotations["fake.anno.3"] = "fakeannotation3"
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
					service.Labels["fake.label.1"] = "fakelabel1"
					service.Labels["fake.label.2"] = "fakelabel2"
					service.Labels["fake.label.3"] = "fakelabel3"
					return toModify, service, nil
				},
				func(resource runtime.Object) bool { //find resource for test
					//return true if object is the one we want to test
					_, ok := resource.(*corev1.Service)
					return ok
				},
				func(postUpgradeObj runtime.Object, deisredObj runtime.Object) bool { //check resource was upgraded
					//return true if postUpgrade has the same fields as desired
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

			newVersion := "1.10.0"
			prevVersion := "1.9.5"

			args := createFromArgs(newVersion)
			doReconcile(args)

			setDeploymentsReady(args)

			//verify on int version is set
			Expect(args.cdi.Status.Phase).Should(Equal(sdkapi.PhaseDeployed))

			//Modify CRD to be of previousVersion
			crSetVersion(args.reconciler.reconciler, args.cdi, prevVersion)
			err := args.client.Update(context.TODO(), args.cdi)
			Expect(err).ToNot(HaveOccurred())

			setDeploymentsDegraded(args)
			unusedObj, err := createObj()
			Expect(err).ToNot(HaveOccurred())
			unusedMetaObj := unusedObj.(metav1.Object)
			unusedMetaObj.GetLabels()["operator.cdi.kubevirt.io/createVersion"] = prevVersion
			err = controllerutil.SetControllerReference(args.cdi, unusedMetaObj, scheme.Scheme)
			Expect(err).ToNot(HaveOccurred())

			//add unused object via client, with curObject
			err = args.client.Create(context.TODO(), unusedObj)
			Expect(err).ToNot(HaveOccurred())

			doReconcile(args)

			//verify upgraded has started
			Expect(args.cdi.Status.Phase).Should(Equal(sdkapi.PhaseUpgrading))

			//verify unused exists before upgrade is done
			_, err = getObject(args.client, unusedObj)
			Expect(err).ToNot(HaveOccurred())

			//change deployment to ready
			isReady := setDeploymentsReady(args)
			Expect(isReady).Should(Equal(true))

			doReconcile(args)
			Expect(args.cdi.Status.Phase).Should(Equal(sdkapi.PhaseDeployed))

			//verify that object no longer exists after upgrade
			_, err = getObject(args.client, unusedObj)
			Expect(errors.IsNotFound(err)).Should(Equal(true))

		},

			Entry("verify - unused deployment deleted",
				func() (runtime.Object, error) {
					deployment := utils.CreateDeployment("fake-cdi-deployment", "app", "containerized-data-importer", "fake-sa", int32(1), &sdkapi.NodePlacement{})
					return deployment, nil
				}),
			Entry("verify - unused service deleted",
				func() (runtime.Object, error) {
					service := utils.ResourcesBuiler.CreateService("fake-cdi-service", "fake-service", "fake", nil)
					return service, nil
				}),
			Entry("verify - unused sa deleted",
				func() (runtime.Object, error) {
					sa := utils.ResourcesBuiler.CreateServiceAccount("fake-cdi-sa")
					return sa, nil
				}),

			Entry("verify - unused crd deleted",
				func() (runtime.Object, error) {
					crd := &extv1.CustomResourceDefinition{
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
						Spec: extv1.CustomResourceDefinitionSpec{
							Group: "cdi.kubevirt.io",
							Scope: "Cluster",

							Versions: []extv1.CustomResourceDefinitionVersion{
								{
									Name:    "v1beta1",
									Served:  true,
									Storage: true,
									AdditionalPrinterColumns: []extv1.CustomResourceColumnDefinition{
										{Name: "Age", Type: "date", JSONPath: ".metadata.creationTimestamp"},
										{Name: "Phase", Type: "string", JSONPath: ".status.phase"},
									},
								},
								{
									Name:    "v1alpha1",
									Served:  true,
									Storage: false,
									AdditionalPrinterColumns: []extv1.CustomResourceColumnDefinition{
										{Name: "Age", Type: "date", JSONPath: ".metadata.creationTimestamp"},
										{Name: "Phase", Type: "string", JSONPath: ".status.phase"},
									},
								},
							},
							Names: extv1.CustomResourceDefinitionNames{
								Kind:     "FakeCDI",
								ListKind: "FakeCDIList",
								Plural:   "fakecdis",
								Singular: "fakecdi",
								Categories: []string{
									"all",
								},
								ShortNames: []string{"fakecdi", "fakecdis"},
							},
						},
					}
					return crd, nil
				}),

			Entry("verify - unused role deleted",
				func() (runtime.Object, error) {
					role := utils.ResourcesBuiler.CreateRole("fake-role", nil)
					return role, nil
				}),

			Entry("verify - unused role binding deleted",
				func() (runtime.Object, error) {
					role := utils.ResourcesBuiler.CreateRoleBinding("fake-role", "fake-role", "fake-role", "fake-role")
					return role, nil
				}),
			Entry("verify - unused cluster role deleted",
				func() (runtime.Object, error) {
					role := utils.ResourcesBuiler.CreateClusterRole("fake-cluster-role", nil)
					return role, nil
				}),
			Entry("verify - unused cluster role binding deleted",
				func() (runtime.Object, error) {
					role := utils.ResourcesBuiler.CreateClusterRoleBinding("fake-cluster-role", "fake-cluster-role", "fake-cluster-role", "fake-cluster-role")
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
		r, err := getObject(reconciler.client, resource)
		Expect(err).ToNot(HaveOccurred())
		if tomodify(r) {
			orig = r
			break
		}
	}
	//apply modify function on resource and return modified one
	return modify(orig)
}

type cdiOverride interface {
	Set(cr *cdiv1.CDI)
	Check(d *appsv1.Deployment)
}

type pullOverride struct {
	value corev1.PullPolicy
}

func (o *pullOverride) Set(cr *cdiv1.CDI) {
	cr.Spec.ImagePullPolicy = o.value
}

func (o *pullOverride) Check(d *appsv1.Deployment) {
	pp := d.Spec.Template.Spec.Containers[0].ImagePullPolicy
	Expect(pp).Should(Equal(o.value))
}

func getCDI(client realClient.Client, cdi *cdiv1.CDI) (*cdiv1.CDI, error) {
	result, err := getObject(client, cdi)
	if err != nil {
		return nil, err
	}
	return result.(*cdiv1.CDI), nil
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

		if len(args.cdi.Status.Conditions) == 3 &&
			conditions.IsStatusConditionTrue(args.cdi.Status.Conditions, conditions.ConditionAvailable) &&
			conditions.IsStatusConditionFalse(args.cdi.Status.Conditions, conditions.ConditionProgressing) &&
			conditions.IsStatusConditionFalse(args.cdi.Status.Conditions, conditions.ConditionDegraded) {
			running = true
		}
	}

	return running
}

func setDeploymentsDegraded(args *args) {
	resources, err := getAllResources(args.reconciler)
	Expect(err).ToNot(HaveOccurred())

	for _, r := range resources {
		d, ok := r.(*appsv1.Deployment)
		if !ok {
			continue
		}

		d, err := getDeployment(args.client, d)
		Expect(err).ToNot(HaveOccurred())
		if d.Spec.Replicas != nil {
			d.Status.Replicas = int32(0)
			d.Status.ReadyReplicas = d.Status.Replicas
			err = args.client.Update(context.TODO(), d)
			Expect(err).ToNot(HaveOccurred())
		}

	}
	doReconcile(args)
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
	crs, err := clusterResources.CreateAllStaticResources(reconciler.clusterArgs)
	if err != nil {
		return nil, err
	}

	result = append(result, crs...)

	nrs, err := namespaceResources.CreateAllResources(reconciler.namespacedArgs)
	if err != nil {
		return nil, err
	}

	result = append(result, nrs...)

	drs, err := clusterResources.CreateAllDynamicResources(reconciler.clusterArgs)
	if err != nil {
		return nil, err
	}

	result = append(result, drs...)

	return result, nil
}

func reconcileRequest(name string) reconcile.Request {
	return reconcile.Request{NamespacedName: types.NamespacedName{Name: name}}
}

func createFromArgs(version string) *args {
	cdi := createCDI("cdi", "good uid")
	client := createClient(cdi)
	reconciler := createReconcilerWithVersion(client, version)

	return &args{
		cdi:        cdi,
		client:     client,
		reconciler: reconciler,
	}
}

func createArgs() *args {
	cdi := createCDI("cdi", "good uid")
	client := createClient(cdi)
	reconciler := createReconciler(client)

	return &args{
		cdi:        cdi,
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

func doReconcileError(args *args) {
	result, err := args.reconciler.Reconcile(reconcileRequest(args.cdi.Name))
	Expect(err).To(HaveOccurred())
	Expect(result.Requeue).To(BeFalse())

	args.cdi, err = getCDI(args.client, args.cdi)
	Expect(err).ToNot(HaveOccurred())
}

func doReconcileRequeue(args *args) {
	result, err := args.reconciler.Reconcile(reconcileRequest(args.cdi.Name))
	Expect(err).ToNot(HaveOccurred())
	Expect(result.Requeue || result.RequeueAfter > 0).To(BeTrue())

	args.cdi, err = getCDI(args.client, args.cdi)
	Expect(err).ToNot(HaveOccurred())
}

func createClient(objs ...runtime.Object) realClient.Client {
	return fakeClient.NewFakeClientWithScheme(scheme.Scheme, objs...)
}

func createCDI(name, uid string) *cdiv1.CDI {
	return &cdiv1.CDI{ObjectMeta: metav1.ObjectMeta{Name: name, UID: types.UID(uid)}}
}

func createReconcilerWithVersion(client realClient.Client, version string) *ReconcileCDI {
	r := createReconciler(client)
	r.namespacedArgs.OperatorVersion = version
	return r
}

func createReconciler(client realClient.Client) *ReconcileCDI {
	namespace := "cdi"
	clusterArgs := &clusterResources.FactoryArgs{
		Namespace: namespace,
		Client:    client,
		Logger:    log,
	}
	namespacedArgs := &namespaceResources.FactoryArgs{
		OperatorVersion:        version,
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

	recorder := record.NewFakeRecorder(250)
	r := &ReconcileCDI{
		client:         client,
		uncachedClient: client,
		scheme:         scheme.Scheme,
		recorder:       recorder,
		namespace:      namespace,
		clusterArgs:    clusterArgs,
		namespacedArgs: namespacedArgs,
		certManager:    newFakeCertManager(client, namespace),
	}
	callbackDispatcher := callbacks.NewCallbackDispatcher(log, client, client, scheme.Scheme, namespace)
	r.reconciler = sdkr.NewReconciler(r, log, client, callbackDispatcher, scheme.Scheme, createVersionLabel, updateVersionLabel, lastAppliedConfigAnnotation, certPollInterval, finalizerName, recorder).
		WithWatching(true)

	r.registerHooks()
	addReconcileCallbacks(r)

	return r
}

func crSetVersion(r *sdkr.Reconciler, cr *cdiv1.CDI, version string) error {
	return r.CrSetVersion(cr, version)
}

func validateEvents(reconciler *ReconcileCDI, match map[string]bool) {
	events := reconciler.recorder.(*record.FakeRecorder).Events
	// Closing the channel allows me to do non blocking reads of the channel, once the channel runs out of items the loop exits.
	close(events)
	for event := range events {
		val, ok := match[event]
		Expect(ok).To(BeTrue(), "Event [%s] was not expected", event)
		if !val {
			match[event] = true
		}
	}
	for k, v := range match {
		Expect(v).To(BeTrue(), "Event [%s] not observed", k)
	}
}

func createDeleteCDIAfterReadyEventValidationMap() map[string]bool {
	match := createReadyEventValidationMap()
	match[normalDeleteResourceSuccess] = false
	match[normalDeleteResourceSuccesWorker] = false
	return match
}

func createDeleteCDIEventValidationMap() map[string]bool {
	match := createNotReadyEventValidationMap()
	match[normalDeleteResourceSuccess] = false
	match[normalDeleteResourceSuccesWorker] = false
	return match
}

func createErrorCDIEventValidationMap() map[string]bool {
	match := createNotReadyEventValidationMap()
	match["Warning ConfigError Reconciling to error state, unwanted CDI object"] = false
	return match
}

func createReadyEventValidationMap() map[string]bool {
	match := createNotReadyEventValidationMap()
	match[normalCreateEnsured+" upload proxy route exists"] = false
	match["Normal DeployCompleted Deployment Completed"] = false
	return match
}

func createNotReadyEventValidationMap() map[string]bool {
	// match is map of strings and if we observed the event.
	// We are not interested in the order of the events, just that the events happen at least once.
	match := make(map[string]bool)
	match["Normal DeployStarted Started Deployment"] = false
	match[normalCreateSuccess+" *v1.ClusterRole cdi-apiserver"] = false
	match[normalCreateSuccess+" *v1.ClusterRoleBinding cdi-apiserver"] = false
	match[normalCreateSuccess+" *v1.ClusterRole cdi"] = false
	match[normalCreateSuccess+" *v1.ClusterRoleBinding cdi-sa"] = false
	match[normalCreateSuccess+" *v1.CustomResourceDefinition datavolumes.cdi.kubevirt.io"] = false
	match[normalCreateSuccess+" *v1.CustomResourceDefinition cdiconfigs.cdi.kubevirt.io"] = false
	match[normalCreateSuccess+" *v1.ClusterRole cdi-uploadproxy"] = false
	match[normalCreateSuccess+" *v1.ClusterRoleBinding cdi-uploadproxy"] = false
	match[normalCreateSuccess+" *v1.ClusterRole cdi.kubevirt.io:admin"] = false
	match[normalCreateSuccess+" *v1.ClusterRole cdi.kubevirt.io:edit"] = false
	match[normalCreateSuccess+" *v1.ClusterRole cdi.kubevirt.io:view"] = false
	match[normalCreateSuccess+" *v1.ClusterRole cdi.kubevirt.io:config-reader"] = false
	match[normalCreateSuccess+" *v1.ClusterRoleBinding cdi.kubevirt.io:config-reader"] = false
	match[normalCreateSuccess+" *v1.ServiceAccount cdi-apiserver"] = false
	match[normalCreateSuccess+" *v1.RoleBinding cdi-apiserver"] = false
	match[normalCreateSuccess+" *v1.Role cdi-apiserver"] = false
	match[normalCreateSuccess+" *v1.Service cdi-api"] = false
	match[normalCreateSuccess+" *v1.Deployment cdi-apiserver"] = false
	match[normalCreateSuccess+" *v1.ServiceAccount cdi-sa"] = false
	match[normalCreateSuccess+" *v1.RoleBinding cdi-deployment"] = false
	match[normalCreateSuccess+" *v1.Role cdi-deployment"] = false
	match[normalCreateSuccess+" *v1.Deployment cdi-deployment"] = false
	match[normalCreateSuccess+" *v1.ConfigMap cdi-insecure-registries"] = false
	match[normalCreateSuccess+" *v1.ServiceAccount cdi-uploadproxy"] = false
	match[normalCreateSuccess+" *v1.Service cdi-uploadproxy"] = false
	match[normalCreateSuccess+" *v1.RoleBinding cdi-uploadproxy"] = false
	match[normalCreateSuccess+" *v1.Role cdi-uploadproxy"] = false
	match[normalCreateSuccess+" *v1.Deployment cdi-uploadproxy"] = false
	match[normalCreateSuccess+" *v1beta1.APIService v1beta1.upload.cdi.kubevirt.io"] = false
	match[normalCreateSuccess+" *v1beta1.APIService v1alpha1.upload.cdi.kubevirt.io"] = false
	match[normalCreateSuccess+" *v1beta1.ValidatingWebhookConfiguration cdi-api-datavolume-validate"] = false
	match[normalCreateSuccess+" *v1beta1.MutatingWebhookConfiguration cdi-api-datavolume-mutate"] = false
	match[normalCreateSuccess+" *v1beta1.ValidatingWebhookConfiguration cdi-api-validate"] = false
	match[normalCreateSuccess+" *v1.Secret cdi-apiserver-signer"] = false
	match[normalCreateSuccess+" *v1.ConfigMap cdi-apiserver-signer-bundle"] = false
	match[normalCreateSuccess+" *v1.Secret cdi-apiserver-server-cert"] = false
	match[normalCreateSuccess+" *v1.Secret cdi-uploadproxy-signer"] = false
	match[normalCreateSuccess+" *v1.ConfigMap cdi-uploadproxy-signer-bundle"] = false
	match[normalCreateSuccess+" *v1.Secret cdi-uploadproxy-server-cert"] = false
	match[normalCreateSuccess+" *v1.Secret cdi-uploadserver-signer"] = false
	match[normalCreateSuccess+" *v1.ConfigMap cdi-uploadserver-signer-bundle"] = false
	match[normalCreateSuccess+" *v1.Secret cdi-uploadserver-client-signer"] = false
	match[normalCreateSuccess+" *v1.ConfigMap cdi-uploadserver-client-signer-bundle"] = false
	match[normalCreateSuccess+" *v1.Secret cdi-uploadserver-client-cert"] = false
	match[normalCreateSuccess+" *v1.Service cdi-prometheus-metrics"] = false
	match[normalCreateEnsured+" SecurityContextConstraint exists"] = false
	return match
}
