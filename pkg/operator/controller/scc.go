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
	"fmt"

	"github.com/go-logr/logr"
	secv1 "github.com/openshift/api/security/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"kubevirt.io/containerized-data-importer/pkg/operator"
	sdk "kubevirt.io/controller-lifecycle-operator-sdk/pkg/sdk"
)

const sccName = "containerized-data-importer"

func ensureSCCExists(logger logr.Logger, c client.Client, saNamespace, saName string) error {
	scc := &secv1.SecurityContextConstraints{}
	userName := fmt.Sprintf("system:serviceaccount:%s:%s", saNamespace, saName)

	err := c.Get(context.TODO(), client.ObjectKey{Name: sccName}, scc)
	if meta.IsNoMatchError(err) {
		// not in openshift
		logger.V(3).Info("No match error for SCC, must not be in openshift")
		return nil
	} else if errors.IsNotFound(err) {
		scc = &secv1.SecurityContextConstraints{
			ObjectMeta: metav1.ObjectMeta{
				Name: sccName,
				Labels: map[string]string{
					"cdi.kubevirt.io": "",
				},
			},
			Priority: &[]int32{10}[0],
			FSGroup: secv1.FSGroupStrategyOptions{
				Type: secv1.FSGroupStrategyRunAsAny,
			},
			RequiredDropCapabilities: []corev1.Capability{
				"MKNOD",
			},
			RunAsUser: secv1.RunAsUserStrategyOptions{
				Type: secv1.RunAsUserStrategyRunAsAny,
			},
			SELinuxContext: secv1.SELinuxContextStrategyOptions{
				Type: secv1.SELinuxStrategyMustRunAs,
			},
			SupplementalGroups: secv1.SupplementalGroupsStrategyOptions{
				Type: secv1.SupplementalGroupsStrategyRunAsAny,
			},
			Volumes: []secv1.FSType{
				secv1.FSTypeConfigMap,
				secv1.FSTypeDownwardAPI,
				secv1.FSTypeEmptyDir,
				secv1.FSTypePersistentVolumeClaim,
				secv1.FSProjected,
				secv1.FSTypeSecret,
			},
			Users: []string{
				userName,
			},
		}

		if err = operator.SetOwnerRuntime(c, scc); err != nil {
			return err
		}

		return c.Create(context.TODO(), scc)
	} else if err != nil {
		return err
	}

	if !sdk.ContainsStringValue(scc.Users, userName) {
		scc.Users = append(scc.Users, userName)

		return c.Update(context.TODO(), scc)
	}

	return nil
}

func (r *ReconcileCDI) watchSecurityContextConstraints() error {
	err := r.controller.Watch(
		&source.Kind{Type: &secv1.SecurityContextConstraints{}},
		enqueueCDI(r.client),
	)
	if err != nil {
		if meta.IsNoMatchError(err) {
			log.Info("Not watching SecurityContextConstraints")
			return nil
		}

		return err
	}

	return nil
}
