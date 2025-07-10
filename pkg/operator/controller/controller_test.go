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
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	routev1 "github.com/openshift/api/route/v1"
	secv1 "github.com/openshift/api/security/v1"
	conditions "github.com/openshift/custom-resource-status/conditions/v1"
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"

	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeClient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/monitoring/rules"
	"kubevirt.io/containerized-data-importer/pkg/monitoring/rules/alerts"
	clusterResources "kubevirt.io/containerized-data-importer/pkg/operator/resources/cluster"
	namespaceResources "kubevirt.io/containerized-data-importer/pkg/operator/resources/namespaced"
	utils "kubevirt.io/containerized-data-importer/pkg/operator/resources/utils"
	sdkapi "kubevirt.io/controller-lifecycle-operator-sdk/api"
	"kubevirt.io/controller-lifecycle-operator-sdk/pkg/sdk/callbacks"
	sdkr "kubevirt.io/controller-lifecycle-operator-sdk/pkg/sdk/reconciler"
)

const (
	version       = "v1.5.0"
	cdiNamespace  = "cdi"
	configMapName = "cdi-config"

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

func init() {
	schemeInitFuncs := []func(*runtime.Scheme) error{
		cdiv1.AddToScheme,
		extv1.AddToScheme,
		apiregistrationv1.AddToScheme,
		promv1.AddToScheme,
		secv1.Install,
		routev1.Install,
	}

	for _, f := range schemeInitFuncs {
		if err := f(scheme.Scheme); err != nil {
			panic(fmt.Errorf("failed to initiate the scheme %w", err))
		}
	}
}

type modifyResource func(toModify client.Object) (client.Object, client.Object, error)
type isModifySubject func(resource client.Object) bool
type isUpgraded func(postUpgradeObj client.Object, deisredObj client.Object) bool

type createUnusedObject func() (client.Object, error)

var _ = Describe("Controller", func() {
	DescribeTable("check can create types", func(obj client.Object) {
		client := createClient(obj)

		_, err := getObject(client, obj)
		Expect(err).ToNot(HaveOccurred())
	},
		Entry("CDI type", createCDI("cdi", "good uid")),
		Entry("CDR type", &extv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "crd"}}),
		Entry("SSC type", &secv1.SecurityContextConstraints{ObjectMeta: metav1.ObjectMeta{Name: "scc"}}),
		Entry("Route type", &routev1.Route{ObjectMeta: metav1.ObjectMeta{Name: "route"}}),
		Entry("PromRule type", &promv1.PrometheusRule{ObjectMeta: metav1.ObjectMeta{Name: "rule"}}),
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

			It("should apply securitycontextconstraints and related changes", func() {
				args := createArgs()
				doReconcile(args)
				Expect(setDeploymentsReady(args)).To(BeTrue())

				deploymentList := &appsv1.DeploymentList{}
				err := args.client.List(context.TODO(), deploymentList)
				Expect(err).ToNot(HaveOccurred())
				Expect(deploymentList.Items).To(HaveLen(3))

				for _, d := range deploymentList.Items {
					Expect(d.Spec.Template.GetAnnotations()[secv1.RequiredSCCAnnotation]).To(Equal(common.RestrictedSCCName))
				}

				scc := &secv1.SecurityContextConstraints{
					ObjectMeta: metav1.ObjectMeta{
						Name: "containerized-data-importer",
					},
				}

				scc, err = getSCC(args.client, scc)
				Expect(err).ToNot(HaveOccurred())
				Expect(scc.Labels[common.AppKubernetesPartOfLabel]).To(Equal("testing"))
				Expect(scc.Priority).To(BeNil())

				Expect(scc.Users).To(ContainElement("system:serviceaccount:cdi:cdi-sa"))

				Expect(scc.Volumes).To(ConsistOf(
					secv1.FSTypeConfigMap,
					secv1.FSTypeDownwardAPI,
					secv1.FSTypeEmptyDir,
					secv1.FSTypePersistentVolumeClaim,
					secv1.FSProjected,
					secv1.FSTypeSecret,
					secv1.FSTypeCSI,
					secv1.FSTypeEphemeral,
				))
				Expect(scc.AllowPrivilegeEscalation).To(HaveValue(BeFalse()))
				validateEvents(args.reconciler, createReadyEventValidationMap())
			})

			It("should not fire event over order differences in scc volumes stanza", func() {
				scc := &secv1.SecurityContextConstraints{
					ObjectMeta: metav1.ObjectMeta{
						Name: sccName,
						Labels: map[string]string{
							"cdi.kubevirt.io": "",
						},
					},
					Users: []string{
						"system:serviceaccount:cdi:cdi-sa",
						"system:serviceaccount:cdi:cdi-cronjob",
					},
				}
				setSCC(scc)
				sccCpy := scc.DeepCopy()
				// Shuffle order
				scc.Volumes[0], scc.Volumes[1] = scc.Volumes[1], scc.Volumes[0]
				Expect(apiequality.Semantic.DeepEqual(sccCpy.Volumes, scc.Volumes)).To(BeFalse())
				args := createArgs(scc)
				doReconcile(args)
				Expect(setDeploymentsReady(args)).To(BeTrue())

				events := args.reconciler.recorder.(*record.FakeRecorder).Events
				close(events)
				for event := range events {
					Expect(event).ToNot(ContainSubstring("SecurityContextConstraint"))
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
				Expect(err).ToNot(HaveOccurred())
				route = obj.(*routev1.Route)

				Expect(route.Spec.To.Kind).Should(Equal("Service"))
				Expect(route.Spec.To.Name).Should(Equal(uploadProxyServiceName))
				Expect(route.Spec.TLS.DestinationCACertificate).Should(Equal(testCertData))
				Expect(route.Labels[common.AppKubernetesPartOfLabel]).To(Equal("testing"))
				validateEvents(args.reconciler, createReadyEventValidationMap())
			})

			It("should update existing route", func() {
				route := &routev1.Route{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "user-route",
						Namespace: cdiNamespace,
						Annotations: map[string]string{
							"operator.cdi.kubevirt.io/injectUploadProxyCert": "true",
						},
					},
					Spec: routev1.RouteSpec{
						TLS: &routev1.TLSConfig{},
					},
				}

				args := createArgs(route)
				doReconcile(args)
				Expect(setDeploymentsReady(args)).To(BeTrue())

				obj, err := getObject(args.client, route)
				Expect(err).ToNot(HaveOccurred())
				route = obj.(*routev1.Route)

				Expect(route.Spec.TLS.DestinationCACertificate).Should(Equal(testCertData))

				eventMap := createReadyEventValidationMap()
				eventMap["Normal UploadProxyRouteInjectSuccess Successfully updated Route destination CA certificate"] = false
				validateEvents(args.reconciler, eventMap)
			})

			It("should have CDIOperatorDown", func() {
				args := createArgs()
				doReconcile(args)
				Expect(setDeploymentsReady(args)).To(BeTrue())

				runbookURLTemplate := alerts.GetRunbookURLTemplate()

				rule := &promv1.PrometheusRule{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "prometheus-cdi-rules",
						Namespace: cdiNamespace,
					},
				}
				obj, err := getObject(args.client, rule)
				Expect(err).ToNot(HaveOccurred())
				rule = obj.(*promv1.PrometheusRule)
				duration := promv1.Duration("5m")
				cdiDownAlert := promv1.Rule{
					Alert: "CDIOperatorDown",
					Expr:  intstr.FromString("kubevirt_cdi_operator_up == 0"),
					For:   &duration,
					Annotations: map[string]string{
						"summary":     "CDI operator is down",
						"runbook_url": fmt.Sprintf(runbookURLTemplate, "CDIOperatorDown"),
					},
					Labels: map[string]string{
						"severity":                      "warning",
						"operator_health_impact":        "critical",
						"kubernetes_operator_part_of":   "kubevirt",
						"kubernetes_operator_component": "containerized-data-importer",
					},
				}

				Expect(rule.Spec.Groups[1].Rules).To(ContainElement(cdiDownAlert))
				Expect(rule.Labels[common.AppKubernetesPartOfLabel]).To(Equal("testing"))
				validateEvents(args.reconciler, createReadyEventValidationMap())
			})

			It("should use the default runbook URL template when no ENV Variable is set", func() {
				args := createArgs()
				doReconcile(args)
				Expect(setDeploymentsReady(args)).To(BeTrue())

				rule := &promv1.PrometheusRule{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "prometheus-cdi-rules",
						Namespace: cdiNamespace,
					},
				}
				obj, err := getObject(args.client, rule)
				Expect(err).ToNot(HaveOccurred())
				rule = obj.(*promv1.PrometheusRule)

				for _, group := range rule.Spec.Groups {
					for _, rule := range group.Rules {
						if rule.Alert != "" {
							if rule.Annotations["runbook_url"] != "" {
								Expect(rule.Annotations["runbook_url"]).To(Equal(fmt.Sprintf(defaultRunbookURLTemplate, rule.Alert)))
							}
						}
					}
				}
			})

			It("should use the desired runbook URL template when its ENV Variable is set", func() {
				desiredRunbookURLTemplate := "desired/runbookURL/template/%s"
				os.Setenv(runbookURLTemplateEnv, desiredRunbookURLTemplate)

				args := createArgs()
				doReconcile(args)
				Expect(setDeploymentsReady(args)).To(BeTrue())

				rule := &promv1.PrometheusRule{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "prometheus-cdi-rules",
						Namespace: cdiNamespace,
					},
				}
				obj, err := getObject(args.client, rule)
				Expect(err).ToNot(HaveOccurred())
				rule = obj.(*promv1.PrometheusRule)

				for _, group := range rule.Spec.Groups {
					for _, rule := range group.Rules {
						if rule.Alert != "" {
							if rule.Annotations["runbook_url"] != "" {
								Expect(rule.Annotations["runbook_url"]).To(Equal(fmt.Sprintf(desiredRunbookURLTemplate, rule.Alert)))
							}
						}
					}
				}

				os.Unsetenv(runbookURLTemplateEnv)
			})

			It("should create prometheus service monitor", func() {
				args := createArgs()
				doReconcile(args)
				Expect(setDeploymentsReady(args)).To(BeTrue())

				monitor := &promv1.ServiceMonitor{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "service-monitor-cdi",
						Namespace: cdiNamespace,
					},
				}
				obj, err := getObject(args.client, monitor)
				Expect(err).ToNot(HaveOccurred())
				monitor = obj.(*promv1.ServiceMonitor)

				Expect(monitor.Spec.NamespaceSelector.MatchNames).To(ContainElement(cdiNamespace))
				Expect(monitor.Labels[common.AppKubernetesPartOfLabel]).To(Equal("testing"))
				validateEvents(args.reconciler, createReadyEventValidationMap())
			})

			It("should create prometheus rbac", func() {
				args := createArgs()
				doReconcile(args)
				Expect(setDeploymentsReady(args)).To(BeTrue())

				role := &rbacv1.Role{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cdi-monitoring",
						Namespace: cdiNamespace,
					},
				}
				obj, err := getObject(args.client, role)
				Expect(err).ToNot(HaveOccurred())
				role = obj.(*rbacv1.Role)
				roleBinding := &rbacv1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cdi-monitoring",
						Namespace: cdiNamespace,
					},
				}
				obj, err = getObject(args.client, roleBinding)
				Expect(err).ToNot(HaveOccurred())
				roleBinding = obj.(*rbacv1.RoleBinding)

				Expect(role.Rules[0].Resources).To(ContainElement("endpoints"))
				Expect(roleBinding.Subjects[0].Name).To(Equal("prometheus-k8s"))
				Expect(role.Labels[common.AppKubernetesPartOfLabel]).To(Equal("testing"))
				Expect(roleBinding.Labels[common.AppKubernetesPartOfLabel]).To(Equal("testing"))
				validateEvents(args.reconciler, createReadyEventValidationMap())
			})

			Context("RBAC testing", func() {
				// https://kubernetes.io/docs/concepts/security/rbac-good-practices/#persistent-volume-creation
				checkPersistentVolumeCreateViolation := func(rsc string, rule *rbacv1.PolicyRule) {
					if rsc == "persistentvolumes" {
						Expect(rule.Verbs).ToNot(ContainElement("create"))
					}
				}

				// https://kubernetes.io/docs/concepts/security/rbac-good-practices/#control-admission-webhooks
				checkWebhookViolation := func(rsc string, rule *rbacv1.PolicyRule) {
					if rsc == "validatingwebhookconfigurations" || rsc == "mutatingwebhookconfigurations" {
						for _, verb := range rule.Verbs {
							if verb == "update" || verb == "patch" || verb == "delete" || verb == "get" {
								Expect(rule.ResourceNames).ToNot(BeEmpty())
							}
						}
					}
				}

				allGroupsExcepted := func(groups []string) bool {
					for _, group := range groups {
						if !strings.HasSuffix(group, "cdi.kubevirt.io") {
							return false
						}
					}
					return true
				}

				verifyRule := func(rule *rbacv1.PolicyRule) {
					Expect(rule.Verbs).ToNot(ContainElement("escalate"))
					Expect(rule.Verbs).ToNot(ContainElement("bind"))
					Expect(rule.Verbs).ToNot(ContainElement("impersonate"))
					Expect(rule.APIGroups).ToNot(ContainElement("*"))
					if len(rule.APIGroups) > 0 && allGroupsExcepted(rule.APIGroups) {
						return
					}

					Expect(rule.Resources).ToNot(ContainElement("*"))
					Expect(rule.Verbs).ToNot(ContainElement("*"))

					for _, rsc := range rule.Resources {
						checkPersistentVolumeCreateViolation(rsc, rule)
						checkWebhookViolation(rsc, rule)
					}
				}

				It("should not have global rbac", func() {
					args := createArgs()
					doReconcile(args)
					Expect(setDeploymentsReady(args)).To(BeTrue())

					roles := &rbacv1.RoleList{}
					err := args.client.List(context.TODO(), roles)
					Expect(err).ToNot(HaveOccurred())
					for _, role := range roles.Items {
						for _, rule := range role.Rules {
							verifyRule(&rule)
						}
					}

					croles := &rbacv1.ClusterRoleList{}
					err = args.client.List(context.TODO(), croles)
					Expect(err).ToNot(HaveOccurred())
					for _, crole := range croles.Items {
						for _, rule := range crole.Rules {
							verifyRule(&rule)
						}
					}
				})
			})

			It("should reconcile configmap labels on update", func() {
				args := createArgs()
				doReconcile(args)
				Expect(setDeploymentsReady(args)).To(BeTrue())

				cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: cdiNamespace, Name: configMapName}}
				obj, err := getObject(args.client, cm)
				cm = obj.(*corev1.ConfigMap)
				Expect(err).ToNot(HaveOccurred())
				Expect(cm.OwnerReferences[0].UID).Should(Equal(args.cdi.UID))
				Expect(cm.Labels[common.AppKubernetesPartOfLabel]).To(Equal("testing"))

				args.cdi.Labels[common.AppKubernetesPartOfLabel] = "newtesting"
				err = args.client.Update(context.TODO(), args.cdi)
				Expect(err).ToNot(HaveOccurred())

				doReconcile(args)

				cm = &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: cdiNamespace, Name: configMapName}}
				obj, err = getObject(args.client, cm)
				cm = obj.(*corev1.ConfigMap)
				Expect(err).ToNot(HaveOccurred())
				Expect(cm.Labels[common.AppKubernetesPartOfLabel]).To(Equal("newtesting"))
			})

			It("should get rid of deprecated insecure registries config map", func() {
				args := createArgs()
				doReconcile(args)
				Expect(setDeploymentsReady(args)).To(BeTrue())

				cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: cdiNamespace, Name: "cdi-insecure-registries"}}
				err := args.client.Create(context.TODO(), cm)
				Expect(err).ToNot(HaveOccurred())

				doReconcile(args)

				_, err = getObject(args.client, cm)
				Expect(errors.IsNotFound(err)).To(BeTrue())
			})

			It("should set config authority", func() {
				args := createArgs()
				doReconcile(args)

				cfg := &cdiv1.CDIConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name: "config",
					},
				}

				err := args.client.Create(context.TODO(), cfg)
				Expect(err).ToNot(HaveOccurred())

				Expect(setDeploymentsReady(args)).To(BeTrue())

				cdi, err := getCDI(args.client, args.cdi)
				Expect(err).ToNot(HaveOccurred())
				_, ok := cdi.Annotations["cdi.kubevirt.io/configAuthority"]
				Expect(ok).To(BeTrue())
				Expect(cdi.Spec.Config).To(BeNil())
			})

			It("should set config authority (existing values)", func() {
				args := createArgs()
				doReconcile(args)

				cfg := &cdiv1.CDIConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name: "config",
					},
					Spec: cdiv1.CDIConfigSpec{
						FeatureGates: []string{"foobar"},
					},
				}

				err := args.client.Create(context.TODO(), cfg)
				Expect(err).ToNot(HaveOccurred())

				Expect(setDeploymentsReady(args)).To(BeTrue())

				cdi, err := getCDI(args.client, args.cdi)
				Expect(err).ToNot(HaveOccurred())
				_, ok := cdi.Annotations["cdi.kubevirt.io/configAuthority"]
				Expect(ok).To(BeTrue())
				Expect(cdi.Spec.Config).To(Equal(&cfg.Spec))
			})

			It("should set verbosity level into namespacedArguments", func() {
				args := createArgs()
				doReconcile(args)
				Expect(args.reconciler.namespacedArgs.Verbosity).To(Equal("1"))
				expectedValue := int32(5)
				cfg := &cdiv1.CDIConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name: "config",
					},
					Spec: cdiv1.CDIConfigSpec{
						LogVerbosity: &expectedValue,
					},
				}

				err := args.client.Create(context.TODO(), cfg)
				Expect(err).ToNot(HaveOccurred())
				doReconcile(args)
				Expect(*args.cdi.Spec.Config.LogVerbosity).To(Equal(expectedValue))
				args.reconciler.namespacedArgs = args.reconciler.getNamespacedArgs(args.cdi)
				Expect(args.reconciler.namespacedArgs.Verbosity).To(Equal(strconv.Itoa(int(expectedValue))))
			})

			It("can become ready, un-ready, and ready again", func() {
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

					err = args.client.Status().Update(context.TODO(), dd)
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
				err = args.client.Status().Update(context.TODO(), deployment)
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
				err = args.client.Status().Update(context.TODO(), deployment)
				Expect(err).ToNot(HaveOccurred())

				doReconcile(args)

				Expect(args.cdi.Status.Conditions).Should(HaveLen(3))
				Expect(conditions.IsStatusConditionTrue(args.cdi.Status.Conditions, conditions.ConditionAvailable)).To(BeTrue())
				Expect(conditions.IsStatusConditionFalse(args.cdi.Status.Conditions, conditions.ConditionProgressing)).To(BeTrue())
				Expect(conditions.IsStatusConditionFalse(args.cdi.Status.Conditions, conditions.ConditionDegraded)).To(BeTrue())
				validateEvents(args.reconciler, createReadyEventValidationMap())
			})

			It("should be an error when creating another CDI instance", func() {
				args := createArgs()
				doReconcile(args)

				newInstance := createCDI("bad", "bad")
				err := args.client.Create(context.TODO(), newInstance)
				Expect(err).ToNot(HaveOccurred())

				result, err := args.reconciler.Reconcile(context.TODO(), reconcileRequest(newInstance.Name))
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

				err = args.client.Delete(context.TODO(), args.cdi)
				Expect(err).ToNot(HaveOccurred())

				doReconcileExpectDelete(args)

				_, err = getObject(args.client, pod)
				Expect(errors.IsNotFound(err)).To(BeTrue())
				validateEvents(args.reconciler, createDeleteCDIEventValidationMap())
			})

			It("should recreate existing controller deployment with wrong .spec.selector labels", func() {
				args := createArgs()
				doReconcile(args)
				setDeploymentsReady(args)

				deploy := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: cdiNamespace, Name: "cdi-deployment"}}
				deploy, err := getDeployment(args.client, deploy)
				Expect(err).ToNot(HaveOccurred())

				deploy.Spec.Selector.MatchLabels = map[string]string{"test": "test"}
				Expect(args.client.Update(context.TODO(), deploy)).To(Succeed())

				// immutable field and mismatch detected, resource should be deleted
				doReconcileError(args)

				_, err = getDeployment(args.client, deploy)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("deployments.apps \"cdi-deployment\" not found"))

				// requeue (recreate cdi-deployment)
				doReconcileRequeue(args)

				deploy, err = getDeployment(args.client, deploy)
				Expect(err).ToNot(HaveOccurred())

				Expect(deploy.Spec.Selector.MatchLabels).To(Equal(map[string]string{common.CDIComponentLabel: common.CDIControllerResourceName}))
			})

			It("should create all deployment containers with terminationMessagePolicy FallbackToLogsOnError", func() {
				args := createArgs()
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
					for _, c := range d.Spec.Template.Spec.Containers {
						Expect(c.TerminationMessagePolicy).To(Equal(corev1.TerminationMessageFallbackToLogsOnError))
					}
				}
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
			Expect(isReady).Should(BeTrue())

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
					Expect(isReady).Should(BeTrue())

					//verify on int version is set
					Expect(args.cdi.Status.Phase).Should(Equal(sdkapi.PhaseDeployed))

					//Modify CRD to be of previousVersion
					Expect(crSetVersion(args.reconciler.reconciler, args.cdi, prevVersion)).To(Succeed())
					//mark CDI CR for deletion
					args.cdi.Finalizers = append(args.cdi.Finalizers, "keepmearound")
					Expect(args.client.Update(context.TODO(), args.cdi)).To(Succeed())
					Expect(args.client.Delete(context.TODO(), args.cdi)).To(Succeed())

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
					Expect(crSetVersion(args.reconciler.reconciler, args.cdi, prevVersion)).To(Succeed())
					Expect(args.client.Update(context.TODO(), args.cdi)).To(Succeed())
					setDeploymentsDegraded(args)

					//begin upgrade
					doReconcile(args)

					//mark CDI CR for deletion
					Expect(args.client.Delete(context.TODO(), args.cdi)).To(Succeed())

					doReconcileExpectDelete(args)

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
			Expect(crSetVersion(args.reconciler.reconciler, args.cdi, prevVersion)).To(Succeed())
			Expect(args.client.Update(context.TODO(), args.cdi)).To(Succeed())

			setDeploymentsDegraded(args)

			//find the resource to modify
			oOriginal, oModified, err := getModifiedResource(args.reconciler, modify, tomodify)
			Expect(err).ToNot(HaveOccurred())

			//update object via client, with curObject
			Expect(args.client.Update(context.TODO(), oModified)).To(Succeed())

			//verify object is modified
			storedObj, err := getObject(args.client, oModified)
			Expect(err).ToNot(HaveOccurred())

			Expect(reflect.DeepEqual(storedObj, oModified)).Should(BeTrue())

			doReconcile(args)

			//verify upgraded has started
			Expect(args.cdi.Status.Phase).Should(Equal(sdkapi.PhaseUpgrading))

			//change deployment to ready
			Expect(setDeploymentsReady(args)).Should(BeTrue())

			doReconcile(args)
			Expect(args.cdi.Status.Phase).Should(Equal(sdkapi.PhaseDeployed))

			//verify that stored object equals to object in getResources
			storedObj, err = getObject(args.client, oModified)
			Expect(err).ToNot(HaveOccurred())

			Expect(upgraded(storedObj, oOriginal)).Should(BeTrue())

		},
			//Deployment update
			Entry("verify - deployment updated on upgrade - annotation changed",
				func(toModify client.Object) (client.Object, client.Object, error) { //Modify
					deploymentOrig, ok := toModify.(*appsv1.Deployment)
					if !ok {
						return toModify, toModify, generrors.New("wrong type")
					}
					deployment := deploymentOrig.DeepCopy()
					deployment.Annotations["fake.anno.1"] = "fakeannotation1"
					deployment.Annotations["fake.anno.2"] = "fakeannotation2"
					deployment.Annotations["fake.anno.3"] = "fakeannotation3"
					return toModify, deployment, nil
				},
				func(resource client.Object) bool { //find resource for test
					//return true if object is the one we want to test
					_, ok := resource.(*appsv1.Deployment)
					return ok
				},
				func(postUpgradeObj client.Object, deisredObj client.Object) bool { //check resource was upgraded
					//return true if postUpgrade has the same fields as desired
					postDep, ok := postUpgradeObj.(*appsv1.Deployment)
					if !ok {
						return false
					}

					desiredDep, ok := deisredObj.(*appsv1.Deployment)
					if !ok {
						return false
					}

					delete(desiredDep.Annotations, LastAppliedConfigAnnotation)

					for key, ann := range desiredDep.Annotations {
						if postDep.Annotations[key] != ann {
							return false
						}
					}

					return len(desiredDep.Annotations) <= len(postDep.Annotations)
				}),
			Entry("verify - deployment updated on upgrade - labels changed",
				func(toModify client.Object) (client.Object, client.Object, error) { //Modify
					deploymentOrig, ok := toModify.(*appsv1.Deployment)
					if !ok {
						return toModify, toModify, generrors.New("wrong type")
					}
					deployment := deploymentOrig.DeepCopy()
					deployment.Labels["fake.label.1"] = "fakelabel1"
					deployment.Labels["fake.label.2"] = "fakelabel2"
					deployment.Labels["fake.label.3"] = "fakelabel3"
					return toModify, deployment, nil
				},
				func(resource client.Object) bool { //find resource for test
					//return true if object is the one we want to test
					_, ok := resource.(*appsv1.Deployment)
					return ok
				},
				func(postUpgradeObj client.Object, deisredObj client.Object) bool { //check resource was upgraded
					//return true if postUpgrade has the same fields as desired
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

					return len(desiredDep.Labels) <= len(postDep.Labels)
				}),
			Entry("verify - deployment updated on upgrade - deployment spec changed - modify container",
				func(toModify client.Object) (client.Object, client.Object, error) { //Modify
					deploymentOrig, ok := toModify.(*appsv1.Deployment)
					if !ok {
						return toModify, toModify, generrors.New("wrong type")
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
				func(resource client.Object) bool { //find resource for test
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
				func(postUpgradeObj client.Object, deisredObj client.Object) bool { //check resource was upgraded
					//return true if postUpgrade has the same fields as desired
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

					return len(desiredDep.Spec.Template.Spec.Containers[0].Env) == len(postDep.Spec.Template.Spec.Containers[0].Env)
				}),
			Entry("verify - deployment updated on upgrade - deployment spec changed - add new container",
				func(toModify client.Object) (client.Object, client.Object, error) { //Modify
					deploymentOrig, ok := toModify.(*appsv1.Deployment)
					if !ok {
						return toModify, toModify, generrors.New("wrong type")
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
				func(resource client.Object) bool { //find resource for test
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
				func(postUpgradeObj client.Object, deisredObj client.Object) bool { //check resource was upgraded
					//return true if postUpgrade has the same fields as desired
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

					return len(desiredDep.Spec.Template.Spec.Containers) <= len(postDep.Spec.Template.Spec.Containers)
				}),
			Entry("verify - deployment updated on upgrade - deployment spec changed - remove existing container",
				func(toModify client.Object) (client.Object, client.Object, error) { //Modify
					deploymentOrig, ok := toModify.(*appsv1.Deployment)
					if !ok {
						return toModify, toModify, generrors.New("wrong type")
					}
					deployment := deploymentOrig.DeepCopy()

					deployment.Spec.Template.Spec.Containers = nil

					return toModify, deployment, nil
				},
				func(resource client.Object) bool { //find resource for test
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
				func(postUpgradeObj client.Object, deisredObj client.Object) bool { //check resource was upgraded
					//return true if postUpgrade has the same fields as desired
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
				func(toModify client.Object) (client.Object, client.Object, error) { //Modify
					serviceOrig, ok := toModify.(*corev1.Service)
					if !ok {
						return toModify, toModify, generrors.New("wrong type")
					}
					service := serviceOrig.DeepCopy()
					service.Annotations["fake.anno.1"] = "fakeannotation1"
					service.Annotations["fake.anno.2"] = "fakeannotation2"
					service.Annotations["fake.anno.3"] = "fakeannotation3"
					return toModify, service, nil
				},
				func(resource client.Object) bool { //find resource for test
					//return true if object is the one we want to test
					_, ok := resource.(*corev1.Service)
					return ok
				},
				func(postUpgradeObj client.Object, deisredObj client.Object) bool { //check resource was upgraded
					//return true if postUpgrade has the same fields as desired
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

					return len(desired.Annotations) <= len(post.Annotations)
				}),

			Entry("verify - services updated on upgrade - label changed",
				func(toModify client.Object) (client.Object, client.Object, error) { //Modify
					serviceOrig, ok := toModify.(*corev1.Service)
					if !ok {
						return toModify, toModify, generrors.New("wrong type")
					}
					service := serviceOrig.DeepCopy()
					service.Labels["fake.label.1"] = "fakelabel1"
					service.Labels["fake.label.2"] = "fakelabel2"
					service.Labels["fake.label.3"] = "fakelabel3"
					return toModify, service, nil
				},
				func(resource client.Object) bool { //find resource for test
					//return true if object is the one we want to test
					_, ok := resource.(*corev1.Service)
					return ok
				},
				func(postUpgradeObj client.Object, deisredObj client.Object) bool { //check resource was upgraded
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

					return len(desired.Labels) <= len(post.Labels)
				}),

			Entry("verify - services updated on upgrade - service port changed",
				func(toModify client.Object) (client.Object, client.Object, error) { //Modify
					serviceOrig, ok := toModify.(*corev1.Service)
					if !ok {
						return toModify, toModify, generrors.New("wrong type")
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
				func(resource client.Object) bool { //find resource for test
					//return true if object is the one we want to test
					_, ok := resource.(*corev1.Service)
					return ok
				},
				func(postUpgradeObj client.Object, deisredObj client.Object) bool { //check resource was upgraded
					//return true if postUpgrade has the same fields as desired
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

					return len(desired.Spec.Ports) == len(post.Spec.Ports)
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
			Expect(crSetVersion(args.reconciler.reconciler, args.cdi, prevVersion)).To(Succeed())
			Expect(args.client.Update(context.TODO(), args.cdi)).To(Succeed())

			setDeploymentsDegraded(args)
			unusedObj, err := createObj()
			Expect(err).ToNot(HaveOccurred())
			unusedMetaObj := unusedObj.(metav1.Object)
			unusedMetaObj.GetLabels()["operator.cdi.kubevirt.io/createVersion"] = prevVersion
			err = controllerutil.SetControllerReference(args.cdi, unusedMetaObj, scheme.Scheme)
			Expect(err).ToNot(HaveOccurred())

			//add unused object via client, with curObject
			Expect(args.client.Create(context.TODO(), unusedObj)).To(Succeed())

			doReconcile(args)

			//verify upgraded has started
			Expect(args.cdi.Status.Phase).Should(Equal(sdkapi.PhaseUpgrading))

			//verify unused exists before upgrade is done
			_, err = getObject(args.client, unusedObj)
			Expect(err).ToNot(HaveOccurred())

			//change deployment to ready
			Expect(setDeploymentsReady(args)).Should(BeTrue())

			doReconcile(args)
			Expect(args.cdi.Status.Phase).Should(Equal(sdkapi.PhaseDeployed))

			//verify that object no longer exists after upgrade
			_, err = getObject(args.client, unusedObj)
			Expect(errors.IsNotFound(err)).Should(BeTrue())

		},

			Entry("verify - unused deployment deleted",
				func() (client.Object, error) {
					const imagePullSecretName = "fake-registry-key"
					var imagePullSecrets = []corev1.LocalObjectReference{{Name: imagePullSecretName}}
					deployment := utils.CreateDeployment("fake-cdi-deployment", "app", "containerized-data-importer", "fake-sa", imagePullSecrets, int32(1), &sdkapi.NodePlacement{})
					return deployment, nil
				}),
			Entry("verify - unused service deleted",
				func() (client.Object, error) {
					service := utils.ResourceBuilder.CreateService("fake-cdi-service", "fake-service", "fake", nil)
					return service, nil
				}),
			Entry("verify - unused sa deleted",
				func() (client.Object, error) {
					sa := utils.ResourceBuilder.CreateServiceAccount("fake-cdi-sa")
					return sa, nil
				}),

			Entry("verify - unused crd deleted",
				func() (client.Object, error) {
					crd := &extv1.CustomResourceDefinition{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "apiextensions.k8s.io/v1",
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
				func() (client.Object, error) {
					role := utils.ResourceBuilder.CreateRole("fake-role", nil)
					return role, nil
				}),

			Entry("verify - unused role binding deleted",
				func() (client.Object, error) {
					role := utils.ResourceBuilder.CreateRoleBinding("fake-role", "fake-role", "fake-role", "fake-role")
					return role, nil
				}),
			Entry("verify - unused cluster role deleted",
				func() (client.Object, error) {
					role := utils.ResourceBuilder.CreateClusterRole("fake-cluster-role", nil)
					return role, nil
				}),
			Entry("verify - unused cluster role binding deleted",
				func() (client.Object, error) {
					role := utils.ResourceBuilder.CreateClusterRoleBinding("fake-cluster-role", "fake-cluster-role", "fake-cluster-role", "fake-cluster-role")
					return role, nil
				}),
		)

	})

	Describe("getCertificateDefinitions function", func() {
		const (
			defaultSignerLifetime = 48 * time.Hour
			defaultSignerRefresh  = 24 * time.Hour
			defaultServerLifetime = 24 * time.Hour
			defaultServerRefresh  = 12 * time.Hour
			defaultClientLifetime = 24 * time.Hour
			defaultClientRefresh  = 12 * time.Hour
		)

		DescribeTable("should use supplied cert config", func(certConfig *cdiv1.CDICertConfig, signerLifetime, signerRefresh, serverLifetime, serverRefresh, clientLifetime, clientRefresh time.Duration) {
			cdi := createCDI("", "")
			cdi.Spec.CertConfig = certConfig
			reconciler := createReconciler(createClient())

			cds := reconciler.getCertificateDefinitions(cdi)
			Expect(cds).To(HaveLen(4))
			for _, cd := range cds {
				Expect(cd.SignerConfig.Lifetime).To(Equal(signerLifetime))
				Expect(cd.SignerConfig.Refresh).To(Equal(signerRefresh))

				if cd.TargetService != nil {
					Expect(cd.TargetConfig.Lifetime).To(Equal(serverLifetime))
					Expect(cd.TargetConfig.Refresh).To(Equal(serverRefresh))
				}

				if cd.TargetUser != nil {
					Expect(cd.TargetConfig.Lifetime).To(Equal(clientLifetime))
					Expect(cd.TargetConfig.Refresh).To(Equal(clientRefresh))
				}
			}
		},
			Entry("with empty cert config", &cdiv1.CDICertConfig{},
				defaultSignerLifetime, defaultSignerRefresh, defaultServerLifetime, defaultServerRefresh, defaultClientLifetime, defaultClientRefresh),
			Entry("with CA cert config", &cdiv1.CDICertConfig{
				CA: &cdiv1.CertConfig{
					Duration:    &metav1.Duration{Duration: 100 * time.Hour},
					RenewBefore: &metav1.Duration{Duration: 10 * time.Hour},
				},
			}, 100*time.Hour, 90*time.Hour, defaultServerLifetime, defaultServerRefresh, defaultClientLifetime, defaultClientRefresh),
			Entry("with Server cert config", &cdiv1.CDICertConfig{
				Server: &cdiv1.CertConfig{
					Duration:    &metav1.Duration{Duration: 12 * time.Hour},
					RenewBefore: &metav1.Duration{Duration: 2 * time.Hour},
				},
			}, defaultSignerLifetime, defaultSignerRefresh, 12*time.Hour, 10*time.Hour, defaultClientLifetime, defaultClientRefresh),
			Entry("with Client cert config", &cdiv1.CDICertConfig{
				Client: &cdiv1.CertConfig{
					Duration:    &metav1.Duration{Duration: 12 * time.Hour},
					RenewBefore: &metav1.Duration{Duration: 2 * time.Hour},
				},
			}, defaultSignerLifetime, defaultSignerRefresh, defaultServerLifetime, defaultServerRefresh, 12*time.Hour, 10*time.Hour),
		)
	})
})

func getModifiedResource(reconciler *ReconcileCDI, modify modifyResource, tomodify isModifySubject) (client.Object, client.Object, error) {
	resources, err := getAllResources(reconciler)
	if err != nil {
		return nil, nil, err
	}

	//find the resource to modify
	var orig client.Object
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

func getCDI(client client.Client, cdi *cdiv1.CDI) (*cdiv1.CDI, error) {
	result, err := getObject(client, cdi)
	if err != nil {
		return nil, err
	}
	return result.(*cdiv1.CDI), nil
}

func getSCC(client client.Client, scc *secv1.SecurityContextConstraints) (*secv1.SecurityContextConstraints, error) {
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

		d, err := getDeployment(args.client, d)
		Expect(err).ToNot(HaveOccurred())
		if d.Spec.Replicas != nil {
			d.Status.Replicas = *d.Spec.Replicas
			d.Status.ReadyReplicas = d.Status.Replicas
			err = args.client.Status().Update(context.TODO(), d)
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
			err = args.client.Status().Update(context.TODO(), d)
			Expect(err).ToNot(HaveOccurred())
		}
	}
	doReconcile(args)
}

func getDeployment(client client.Client, deployment *appsv1.Deployment) (*appsv1.Deployment, error) {
	result, err := getObject(client, deployment)
	if err != nil {
		return nil, err
	}
	return result.(*appsv1.Deployment), nil
}

func getObject(c client.Client, obj client.Object) (client.Object, error) {
	metaObj := obj.(metav1.Object)
	key := client.ObjectKey{Namespace: metaObj.GetNamespace(), Name: metaObj.GetName()}

	typ := reflect.ValueOf(obj).Elem().Type()
	result := reflect.New(typ).Interface().(client.Object)

	if err := c.Get(context.TODO(), key, result); err != nil {
		return nil, err
	}

	return result, nil
}

func getAllResources(reconciler *ReconcileCDI) ([]client.Object, error) {
	var result []client.Object
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

func createArgs(objs ...client.Object) *args {
	cdi := createCDI("cdi", "good uid")
	objs = append(objs, cdi)
	client := createClient(objs...)
	reconciler := createReconciler(client)

	return &args{
		cdi:        cdi,
		client:     client,
		reconciler: reconciler,
	}
}

func doReconcile(args *args) {
	result, err := args.reconciler.Reconcile(context.TODO(), reconcileRequest(args.cdi.Name))
	Expect(err).ToNot(HaveOccurred())
	Expect(result.Requeue).To(BeFalse())

	args.cdi, err = getCDI(args.client, args.cdi)
	Expect(err).ToNot(HaveOccurred())
}

func doReconcileError(args *args) {
	result, err := args.reconciler.Reconcile(context.TODO(), reconcileRequest(args.cdi.Name))
	Expect(err).To(HaveOccurred())
	Expect(result.Requeue).To(BeFalse())

	args.cdi, err = getCDI(args.client, args.cdi)
	Expect(err).ToNot(HaveOccurred())
}

func doReconcileRequeue(args *args) {
	result, err := args.reconciler.Reconcile(context.TODO(), reconcileRequest(args.cdi.Name))
	Expect(err).ToNot(HaveOccurred())
	Expect(result.Requeue || result.RequeueAfter > 0).To(BeTrue())

	args.cdi, err = getCDI(args.client, args.cdi)
	Expect(err).ToNot(HaveOccurred())
}

func doReconcileExpectDelete(args *args) {
	result, err := args.reconciler.Reconcile(context.TODO(), reconcileRequest(args.cdi.Name))
	Expect(err).ToNot(HaveOccurred())
	Expect(result.Requeue).To(BeFalse())

	_, err = getCDI(args.client, args.cdi)
	Expect(err).To(HaveOccurred())
	Expect(errors.IsNotFound(err)).To(BeTrue())
}

func createClient(objs ...client.Object) client.Client {
	var runtimeObjs []runtime.Object
	for _, obj := range objs {
		runtimeObjs = append(runtimeObjs, obj)
	}
	return fakeClient.NewClientBuilder().WithScheme(scheme.Scheme).WithRuntimeObjects(runtimeObjs...).Build()
}

func createCDI(name, uid string) *cdiv1.CDI {
	return &cdiv1.CDI{
		TypeMeta: metav1.TypeMeta{
			Kind:       "CDI",
			APIVersion: "cdis.cdi.kubevirt.io",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			UID:  types.UID(uid),
			Labels: map[string]string{
				common.AppKubernetesManagedByLabel: "tests",
				common.AppKubernetesPartOfLabel:    "testing",
				common.AppKubernetesVersionLabel:   "v0.0.0-tests",
				common.AppKubernetesComponentLabel: "storage",
			},
		},
	}
}

func createReconcilerWithVersion(client client.Client, version string) *ReconcileCDI {
	r := createReconciler(client)
	r.namespacedArgs.OperatorVersion = version
	return r
}

func createReconciler(client client.Client) *ReconcileCDI {
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

	err := rules.SetupRules(namespace)
	if err != nil {
		panic(err)
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
		haveRoutes:     true,
	}
	callbackDispatcher := callbacks.NewCallbackDispatcher(log, client, client, scheme.Scheme, namespace)
	getCache := func() cache.Cache {
		return nil
	}
	r.reconciler = sdkr.NewReconciler(r, log, client, callbackDispatcher, scheme.Scheme, getCache, createVersionLabel, updateVersionLabel, LastAppliedConfigAnnotation, certPollInterval, finalizerName, false, recorder).
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
	match[normalCreateSuccess+" *v1.CustomResourceDefinition storageprofiles.cdi.kubevirt.io"] = false
	match[normalCreateSuccess+" *v1.CustomResourceDefinition datasources.cdi.kubevirt.io"] = false
	match[normalCreateSuccess+" *v1.CustomResourceDefinition dataimportcrons.cdi.kubevirt.io"] = false
	match[normalCreateSuccess+" *v1.CustomResourceDefinition objecttransfers.cdi.kubevirt.io"] = false
	match[normalCreateSuccess+" *v1.CustomResourceDefinition volumeimportsources.cdi.kubevirt.io"] = false
	match[normalCreateSuccess+" *v1.CustomResourceDefinition volumeuploadsources.cdi.kubevirt.io"] = false
	match[normalCreateSuccess+" *v1.CustomResourceDefinition volumeclonesources.cdi.kubevirt.io"] = false
	match[normalCreateSuccess+" *v1.CustomResourceDefinition ovirtvolumepopulators.forklift.cdi.kubevirt.io"] = false
	match[normalCreateSuccess+" *v1.CustomResourceDefinition openstackvolumepopulators.forklift.cdi.kubevirt.io"] = false
	match[normalCreateSuccess+" *v1.ClusterRole cdi-uploadproxy"] = false
	match[normalCreateSuccess+" *v1.ClusterRoleBinding cdi-uploadproxy"] = false
	match[normalCreateSuccess+" *v1.ClusterRole cdi-cronjob"] = false
	match[normalCreateSuccess+" *v1.ClusterRoleBinding cdi-cronjob"] = false
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
	match[normalCreateSuccess+" *v1.ServiceAccount cdi-uploadproxy"] = false
	match[normalCreateSuccess+" *v1.Service cdi-uploadproxy"] = false
	match[normalCreateSuccess+" *v1.RoleBinding cdi-uploadproxy"] = false
	match[normalCreateSuccess+" *v1.Role cdi-uploadproxy"] = false
	match[normalCreateSuccess+" *v1.Deployment cdi-uploadproxy"] = false
	match[normalCreateSuccess+" *v1.ServiceAccount cdi-cronjob"] = false
	match[normalCreateSuccess+" *v1.APIService v1beta1.upload.cdi.kubevirt.io"] = false
	match[normalCreateSuccess+" *v1.ValidatingWebhookConfiguration cdi-api-datavolume-validate"] = false
	match[normalCreateSuccess+" *v1.MutatingWebhookConfiguration cdi-api-datavolume-mutate"] = false
	match[normalCreateSuccess+" *v1.ValidatingWebhookConfiguration cdi-api-validate"] = false
	match[normalCreateSuccess+" *v1.ValidatingWebhookConfiguration cdi-api-populator-validate"] = false
	match[normalCreateSuccess+" *v1.ValidatingWebhookConfiguration objecttransfer-api-validate"] = false
	match[normalCreateSuccess+" *v1.ValidatingWebhookConfiguration cdi-api-dataimportcron-validate"] = false
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
