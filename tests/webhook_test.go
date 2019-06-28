package tests

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"kubevirt.io/containerized-data-importer/tests/framework"
	"kubevirt.io/containerized-data-importer/tests/utils"
)

var _ = Describe("Clone Auth Webhook tests", func() {
	const serviceAccountName = "cdi-auth-webhook-test"

	f := framework.NewFrameworkOrDie("clone-auth-webhook-test")

	Describe("Verify DataVolume validation", func() {
		Context("Authorization checks", func() {
			var err error
			var targetNamespace *corev1.Namespace

			BeforeEach(func() {
				targetNamespace, err = f.CreateNamespace("cdi-auth-webhook-test", nil)
				Expect(err).ToNot(HaveOccurred())

				createServiceAccount(f.K8sClient, targetNamespace.Name, serviceAccountName)

				addPermissionToNamespace(f.K8sClient, serviceAccountName, targetNamespace.Name, targetNamespace.Name)
			})

			AfterEach(func() {
				if targetNamespace != nil {
					err = f.K8sClient.CoreV1().Namespaces().Delete(targetNamespace.Name, &metav1.DeleteOptions{})
					Expect(err).ToNot(HaveOccurred())
				}
			})

			It("should deny/allow user when creating datavolume", func() {
				srcPVCDef := utils.NewPVCDefinition("source-pvc", "1G", nil, nil)
				srcPVCDef.Namespace = f.Namespace.Name
				f.CreateAndPopulateSourcePVC(srcPVCDef, "fill-source", fmt.Sprintf("echo \"hello world\" > %s/data.txt", utils.DefaultPvcMountPath))

				targetDV := utils.NewCloningDataVolume("target-dv", "1G", srcPVCDef)

				client, err := f.GetCdiClientForServiceAccount(targetNamespace.Name, serviceAccountName)
				Expect(err).ToNot(HaveOccurred())

				// can't list dvs in source
				_, err = client.CdiV1alpha1().DataVolumes(f.Namespace.Name).List(metav1.ListOptions{})
				Expect(err).To(HaveOccurred())

				// can list dvs in dest
				_, err = client.CdiV1alpha1().DataVolumes(targetNamespace.Name).List(metav1.ListOptions{})
				Expect(err).ToNot(HaveOccurred())

				// can't create clone of dv in source
				_, err = client.CdiV1alpha1().DataVolumes(targetNamespace.Name).Create(targetDV)
				Expect(err).To(HaveOccurred())

				addPermissionToNamespace(f.K8sClient, serviceAccountName, targetNamespace.Name, f.Namespace.Name)

				// now can list dvs in source
				_, err = client.CdiV1alpha1().DataVolumes(f.Namespace.Name).List(metav1.ListOptions{})
				Expect(err).ToNot(HaveOccurred())

				// now can create clone of dv in source
				_, err = client.CdiV1alpha1().DataVolumes(targetNamespace.Name).Create(targetDV)
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})
})

func createServiceAccount(client kubernetes.Interface, namespace, name string) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	_, err := client.CoreV1().ServiceAccounts(namespace).Create(sa)
	Expect(err).ToNot(HaveOccurred())
}

func addPermissionToNamespace(client kubernetes.Interface, sa, saNamespace, targetNamesace string) {
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name: sa,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{
					"cdi.kubevirt.io",
				},
				Resources: []string{
					"datavolumes",
					"datavolumes/clone-init",
				},
				Verbs: []string{
					"*",
				},
			},
		},
	}

	_, err := client.RbacV1().Roles(targetNamesace).Create(role)
	Expect(err).ToNot(HaveOccurred())

	rb := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: sa,
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "Role",
			Name:     sa,
			APIGroup: "rbac.authorization.k8s.io",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      sa,
				Namespace: saNamespace,
			},
		},
	}

	_, err = client.RbacV1().RoleBindings(targetNamesace).Create(rb)
	Expect(err).ToNot(HaveOccurred())
}
