package tests

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	authv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"kubevirt.io/containerized-data-importer/pkg/clone"
	"kubevirt.io/containerized-data-importer/tests/framework"
	"kubevirt.io/containerized-data-importer/tests/utils"
)

type sarProxy struct {
	client kubernetes.Interface
}

func (p *sarProxy) Create(sar *authv1.SubjectAccessReview) (*authv1.SubjectAccessReview, error) {
	return p.client.AuthorizationV1().SubjectAccessReviews().Create(context.TODO(), sar, metav1.CreateOptions{})
}

var _ = Describe("Clone Auth Webhook tests", func() {
	const serviceAccountName = "cdi-auth-webhook-test"

	var cdiRole = &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name: "explicit-role",
		},
		Rules: []rbacv1.PolicyRule{
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
		},
	}

	var explicitRole = &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name: "explicit-role",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{
					"cdi.kubevirt.io",
				},
				Resources: []string{
					"datavolumes",
					"datavolumes/source",
				},
				Verbs: []string{
					"*",
				},
			},
		},
	}

	var implicitRole = &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name: "implicit-role",
		},
		Rules: []rbacv1.PolicyRule{
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
					"",
				},
				Resources: []string{
					"pods",
				},
				Verbs: []string{
					"create",
				},
			},
		},
	}

	var createServiceAccount = func(client kubernetes.Interface, namespace, name string) {
		sa := &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
		}

		_, err := client.CoreV1().ServiceAccounts(namespace).Create(context.TODO(), sa, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())
	}

	var addPermissionToNamespace = func(client kubernetes.Interface, role *rbacv1.Role, saNamespace, sa, targetNamesace string) {
		_, err := client.RbacV1().Roles(targetNamesace).Create(context.TODO(), role, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())

		rb := &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "rb",
			},
			RoleRef: rbacv1.RoleRef{
				Kind:     "Role",
				Name:     role.Name,
				APIGroup: "rbac.authorization.k8s.io",
			},
		}

		if sa != "" {
			rb.Subjects = []rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Name:      sa,
					Namespace: saNamespace,
				},
			}
		} else {
			rb.Subjects = []rbacv1.Subject{
				{
					Kind:     "Group",
					Name:     "system:serviceaccounts",
					APIGroup: "rbac.authorization.k8s.io",
				},
			}
		}

		_, err = client.RbacV1().RoleBindings(targetNamesace).Create(context.TODO(), rb, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())
	}

	f := framework.NewFramework("clone-auth-webhook-test")

	Describe("Verify DataVolume validation", func() {
		Context("Authorization checks", func() {
			var err error
			var targetNamespace *corev1.Namespace

			BeforeEach(func() {
				targetNamespace, err = f.CreateNamespace("cdi-auth-webhook-test", nil)
				Expect(err).ToNot(HaveOccurred())

				createServiceAccount(f.K8sClient, targetNamespace.Name, serviceAccountName)

				addPermissionToNamespace(f.K8sClient, cdiRole, targetNamespace.Name, serviceAccountName, targetNamespace.Name)
			})

			AfterEach(func() {
				if targetNamespace != nil {
					err = f.K8sClient.CoreV1().Namespaces().Delete(context.TODO(), targetNamespace.Name, metav1.DeleteOptions{})
					Expect(err).ToNot(HaveOccurred())
				}
			})

			DescribeTable("should deny/allow user when creating datavolume", func(role *rbacv1.Role, saName string) {
				srcPVCDef := utils.NewPVCDefinition("source-pvc", "1G", nil, nil)
				srcPVCDef.Namespace = f.Namespace.Name
				f.CreateAndPopulateSourcePVC(srcPVCDef, "fill-source", fmt.Sprintf("echo \"hello world\" > %s/data.txt", utils.DefaultPvcMountPath))

				targetDV := utils.NewCloningDataVolume("target-dv", "1G", srcPVCDef)

				client, err := f.GetCdiClientForServiceAccount(targetNamespace.Name, serviceAccountName)
				Expect(err).ToNot(HaveOccurred())

				// can't list dvs in source
				_, err = client.CdiV1beta1().DataVolumes(f.Namespace.Name).List(context.TODO(), metav1.ListOptions{})
				Expect(err).To(HaveOccurred())

				// can list dvs in dest
				_, err = client.CdiV1beta1().DataVolumes(targetNamespace.Name).List(context.TODO(), metav1.ListOptions{})
				Expect(err).ToNot(HaveOccurred())

				// can't create clone of dv in source
				_, err = client.CdiV1beta1().DataVolumes(targetNamespace.Name).Create(context.TODO(), targetDV, metav1.CreateOptions{})
				Expect(err).To(HaveOccurred())

				// let's do manual check as well
				allowed, reason, err := clone.CanServiceAccountClonePVC(&sarProxy{client: f.K8sClient},
					srcPVCDef.Namespace,
					srcPVCDef.Name,
					targetNamespace.Name,
					serviceAccountName,
				)
				Expect(allowed).To(BeFalse())
				Expect(reason).ToNot(BeEmpty())
				Expect(err).ToNot(HaveOccurred())

				addPermissionToNamespace(f.K8sClient, role, targetNamespace.Name, saName, f.Namespace.Name)

				// now can list dvs in source
				Eventually(func() error {
					_, err = client.CdiV1beta1().DataVolumes(f.Namespace.Name).List(context.TODO(), metav1.ListOptions{})
					return err
				}, 60*time.Second, 2*time.Second).ShouldNot(HaveOccurred())

				// now can create clone of dv in source
				_, err = client.CdiV1beta1().DataVolumes(targetNamespace.Name).Create(context.TODO(), targetDV, metav1.CreateOptions{})
				Expect(err).ToNot(HaveOccurred())

				// let's do another manual check as well
				allowed, reason, err = clone.CanServiceAccountClonePVC(&sarProxy{client: f.K8sClient},
					srcPVCDef.Namespace,
					srcPVCDef.Name,
					targetNamespace.Name,
					serviceAccountName,
				)
				Expect(allowed).To(BeTrue())
				Expect(reason).To(BeEmpty())
				Expect(err).ToNot(HaveOccurred())
			},
				Entry("[test_id:3935]when using explicit CDI permissions", explicitRole, serviceAccountName),
				Entry("when using explicit CDI permissions and all serviceaccounts", explicitRole, ""),
				Entry("[test_id:3936]when using implicit CDI permissions", implicitRole, serviceAccountName),
			)
		})
	})
})
