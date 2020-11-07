package tests

import (
	"context"
	"reflect"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	cdiClientset "kubevirt.io/containerized-data-importer/pkg/client/clientset/versioned"
	"kubevirt.io/containerized-data-importer/tests/framework"
	"kubevirt.io/containerized-data-importer/tests/utils"
)

var _ = Describe("Aggregated role in-action tests", func() {
	var createServiceAccount = func(client kubernetes.Interface, namespace, name string) {
		sa := &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
		}

		_, err := client.CoreV1().ServiceAccounts(namespace).Create(context.TODO(), sa, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())
	}

	var createRoleBinding = func(client kubernetes.Interface, clusterRoleName, namespace, serviceAccount string) {
		rb := &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: serviceAccount,
			},
			RoleRef: rbacv1.RoleRef{
				Kind:     "ClusterRole",
				Name:     clusterRoleName,
				APIGroup: "rbac.authorization.k8s.io",
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Name:      serviceAccount,
					Namespace: namespace,
				},
			},
		}

		_, err := client.RbacV1().RoleBindings(namespace).Create(context.TODO(), rb, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())
	}

	f := framework.NewFramework("aggregated-role-tests")

	DescribeTable("admin/edit datavolume permission checks", func(user string) {
		var cdiClient *cdiClientset.Clientset
		var crClient client.Client
		var err error

		createServiceAccount(f.K8sClient, f.Namespace.Name, user)
		createRoleBinding(f.K8sClient, user, f.Namespace.Name, user)

		Eventually(func() error {
			cdiClient, err = f.GetCdiClientForServiceAccount(f.Namespace.Name, user)
			return err
		}, 60*time.Second, 2*time.Second).ShouldNot(HaveOccurred())

		rc, err := f.GetRESTConfigForServiceAccount(f.Namespace.Name, user)
		Expect(err).ToNot(HaveOccurred())

		crClient, err = client.New(rc, client.Options{Scheme: scheme.Scheme})
		Expect(err).ToNot(HaveOccurred())

		dv := utils.NewDataVolumeWithHTTPImport("test-"+user, "1Gi", "http://nonexistant.url")
		dv, err = cdiClient.CdiV1beta1().DataVolumes(f.Namespace.Name).Create(context.TODO(), dv, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())

		dvl, err := cdiClient.CdiV1beta1().DataVolumes(f.Namespace.Name).List(context.TODO(), metav1.ListOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(dvl.Items).To(HaveLen(1))

		dv, err = cdiClient.CdiV1beta1().DataVolumes(f.Namespace.Name).Get(context.TODO(), dv.Name, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())

		err = cdiClient.CdiV1beta1().DataVolumes(f.Namespace.Name).Delete(context.TODO(), dv.Name, metav1.DeleteOptions{})
		Expect(err).ToNot(HaveOccurred())

		dvl, err = cdiClient.CdiV1beta1().DataVolumes(f.Namespace.Name).List(context.TODO(), metav1.ListOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(dvl.Items).To(HaveLen(0))

		dv = utils.NewDataVolumeForUpload("upload-test-"+user, "1Gi")
		dv, err = cdiClient.CdiV1beta1().DataVolumes(f.Namespace.Name).Create(context.TODO(), dv, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())

		var pvc *corev1.PersistentVolumeClaim
		Eventually(func() error {
			pvc, err = f.K8sClient.CoreV1().PersistentVolumeClaims(f.Namespace.Name).Get(context.TODO(), dv.Name, metav1.GetOptions{})
			if err != nil {
				return err
			}
			return nil
		}, 90*time.Second, 2*time.Second).ShouldNot(HaveOccurred())
		f.ForceBindPvcIfDvIsWaitForFirstConsumer(dv)

		found, err := utils.WaitPVCPodStatusRunning(f.K8sClient, pvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(found).Should(BeTrue())

		token, err := utils.RequestUploadToken(cdiClient, pvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(token).ToNot(BeEmpty())

		cl, err := cdiClient.CdiV1beta1().CDIConfigs().List(context.TODO(), metav1.ListOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(cl.Items).To(HaveLen(1))

		_, err = cdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), cl.Items[0].Name, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())

		err = utils.UpdateCDIConfig(crClient, func(config *cdiv1.CDIConfigSpec) {
			config.ScratchSpaceStorageClass = &[]string{"foobar"}[0]
		})
		Expect(err).To(HaveOccurred())
	},
		Entry("[test_id:3948]can do everything with admin", "admin"),
		Entry("[test_id:3949]can do everything with edit", "edit"),
	)

	It("[test_id:3950]view datavolume permission checks", func() {
		const user = "view"
		var cdiClient cdiClientset.Interface
		var crClient client.Client
		var err error

		createServiceAccount(f.K8sClient, f.Namespace.Name, user)
		createRoleBinding(f.K8sClient, user, f.Namespace.Name, user)

		Eventually(func() error {
			cdiClient, err = f.GetCdiClientForServiceAccount(f.Namespace.Name, user)
			return err
		}, 60*time.Second, 2*time.Second).ShouldNot(HaveOccurred())

		rc, err := f.GetRESTConfigForServiceAccount(f.Namespace.Name, user)
		Expect(err).ToNot(HaveOccurred())

		crClient, err = client.New(rc, client.Options{Scheme: scheme.Scheme})
		Expect(err).ToNot(HaveOccurred())

		dv := utils.NewDataVolumeWithHTTPImport("test-"+user, "1Gi", "http://nonexistant.url")
		dv, err = cdiClient.CdiV1beta1().DataVolumes(f.Namespace.Name).Create(context.TODO(), dv, metav1.CreateOptions{})
		Expect(err).To(HaveOccurred())

		dvl, err := cdiClient.CdiV1beta1().DataVolumes(f.Namespace.Name).List(context.TODO(), metav1.ListOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(dvl.Items).To(HaveLen(0))

		_, err = cdiClient.CdiV1beta1().DataVolumes(f.Namespace.Name).Get(context.TODO(), "test-"+user, metav1.GetOptions{})
		Expect(err).To(HaveOccurred())
		Expect(k8serrors.IsNotFound(err)).To(BeTrue())

		cl, err := cdiClient.CdiV1beta1().CDIConfigs().List(context.TODO(), metav1.ListOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(cl.Items).To(HaveLen(1))

		_, err = cdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), cl.Items[0].Name, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())

		err = utils.UpdateCDIConfig(crClient, func(config *cdiv1.CDIConfigSpec) {
			config.ScratchSpaceStorageClass = &[]string{"foobar"}[0]
		})
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("Aggregated role definition tests", func() {
	var adminRules = []rbacv1.PolicyRule{
		{
			APIGroups: []string{
				"cdi.kubevirt.io",
			},
			Resources: []string{
				"datavolumes",
			},
			Verbs: []string{
				"*",
			},
		},
		{
			APIGroups: []string{
				"cdi.kubevirt.io",
			},
			Resources: []string{
				"datavolumes/source",
			},
			Verbs: []string{
				"create",
			},
		},
		{
			APIGroups: []string{
				"cdi.kubevirt.io",
			},
			Resources: []string{
				"cdiconfigs",
			},
			Verbs: []string{
				"get",
				"list",
				"watch",
				"patch",
				"update",
			},
		},
		{
			APIGroups: []string{
				"upload.cdi.kubevirt.io",
			},
			Resources: []string{
				"uploadtokenrequests",
			},
			Verbs: []string{
				"*",
			},
		},
	}

	var editRules = adminRules

	var viewRules = []rbacv1.PolicyRule{
		{
			APIGroups: []string{
				"cdi.kubevirt.io",
			},
			Resources: []string{
				"datavolumes",
			},
			Verbs: []string{
				"get",
				"list",
				"watch",
			},
		},
		{
			APIGroups: []string{
				"cdi.kubevirt.io",
			},
			Resources: []string{
				"datavolumes/source",
			},
			Verbs: []string{
				"create",
			},
		},
		{
			APIGroups: []string{
				"cdi.kubevirt.io",
			},
			Resources: []string{
				"cdiconfigs",
			},
			Verbs: []string{
				"get",
				"list",
				"watch",
			},
		},
	}

	f := framework.NewFramework("aggregated-role-definition-tests")

	DescribeTable("check all expected rules exist", func(role string, rules []rbacv1.PolicyRule) {
		clusterRole, err := f.K8sClient.RbacV1().ClusterRoles().Get(context.TODO(), role, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())

		found := false
		for _, expectedRule := range rules {
			for _, r := range clusterRole.Rules {
				if reflect.DeepEqual(expectedRule, r) {
					found = true
					break
				}
			}
		}
		Expect(found).To(BeTrue())
	},
		Entry("[test_id:3945]for admin", "admin", adminRules),
		Entry("[test_id:3946]for edit", "edit", editRules),
		Entry("[test_id:3947]for view", "view", viewRules),
	)
})
