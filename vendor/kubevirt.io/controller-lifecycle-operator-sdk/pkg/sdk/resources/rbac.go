package resources

import (
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateClusterRole create cluster role
func (b *ResourceBuilder) CreateClusterRole(name string, policyRules []rbacv1.PolicyRule) *rbacv1.ClusterRole {
	return CreateClusterRole(name, policyRules, b.WithCommonLabels(nil))
}

// CreateOperatorClusterRole create operator cluster role
func (b *ResourceBuilder) CreateOperatorClusterRole(name string, policyRules []rbacv1.PolicyRule) *rbacv1.ClusterRole {
	return CreateClusterRole(name, policyRules, b.WithOperatorLabels(nil))
}

// CreateAggregateClusterRole creates aggregate cluster role
func (b *ResourceBuilder) CreateAggregateClusterRole(name, aggregateTo string, policyRules []rbacv1.PolicyRule) *rbacv1.ClusterRole {
	labels := map[string]string{
		"rbac.authorization.k8s.io/aggregate-to-" + aggregateTo: "true",
	}
	return CreateClusterRole(name, policyRules, b.WithCommonLabels(labels))
}

// CreateRoleBinding creates role binding
func (b *ResourceBuilder) CreateRoleBinding(name, roleRef, serviceAccount, serviceAccountNamespace string) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "RoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: b.WithCommonLabels(nil),
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "Role",
			Name:     roleRef,
			APIGroup: "rbac.authorization.k8s.io",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      serviceAccount,
				Namespace: serviceAccountNamespace,
			},
		},
	}
}

// CreateRole creates role
func (b *ResourceBuilder) CreateRole(name string, rules []rbacv1.PolicyRule) *rbacv1.Role {
	return &rbacv1.Role{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "Role",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: b.WithCommonLabels(nil),
		},
		Rules: rules,
	}
}

// CreateClusterRoleBinding creates cluster role binding
func (b *ResourceBuilder) CreateClusterRoleBinding(name, roleRef, serviceAccount, serviceAccountNamespace string) *rbacv1.ClusterRoleBinding {
	return CreateClusterRoleBinding(name, roleRef, serviceAccount, serviceAccountNamespace, b.WithCommonLabels(nil))
}

// CreateOperatorClusterRoleBinding creates operator cluster role binding
func (b *ResourceBuilder) CreateOperatorClusterRoleBinding(name, roleRef, serviceAccount, serviceAccountNamespace string) *rbacv1.ClusterRoleBinding {
	return CreateClusterRoleBinding(name, roleRef, serviceAccount, serviceAccountNamespace, b.WithOperatorLabels(nil))
}

// CreateServiceAccount creates service account
func (b *ResourceBuilder) CreateServiceAccount(name string) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ServiceAccount",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: b.WithCommonLabels(nil),
		},
	}
}

// CreateOperatorServiceAccount creates service account
func (b *ResourceBuilder) CreateOperatorServiceAccount(name, namespace string) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ServiceAccount",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    b.WithOperatorLabels(nil),
		},
	}
}

// CreateClusterRoleBinding creates cluster role binding
func CreateClusterRoleBinding(name, roleRef, serviceAccount, serviceAccountNamespace string, labels map[string]string) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     roleRef,
			APIGroup: "rbac.authorization.k8s.io",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      serviceAccount,
				Namespace: serviceAccountNamespace,
			},
		},
	}
}

// CreateClusterRole creates a cluster role
func CreateClusterRole(name string, rules []rbacv1.PolicyRule, labels map[string]string) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRole",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
		Rules: rules,
	}
}
