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
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/source"

	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
	"kubevirt.io/containerized-data-importer/pkg/operator"
	"kubevirt.io/containerized-data-importer/pkg/util"
	sdk "kubevirt.io/controller-lifecycle-operator-sdk/pkg/sdk"
)

const sccName = "containerized-data-importer"

func setSCC(scc *secv1.SecurityContextConstraints) {
	// Ensure we are just good citizens that don't want to compete against other prioritized SCCs
	scc.Priority = nil
	scc.RunAsUser = secv1.RunAsUserStrategyOptions{
		Type: secv1.RunAsUserStrategyMustRunAsNonRoot,
	}
	scc.SELinuxContext = secv1.SELinuxContextStrategyOptions{
		Type: secv1.SELinuxStrategyMustRunAs,
	}
	scc.SupplementalGroups = secv1.SupplementalGroupsStrategyOptions{
		Type: secv1.SupplementalGroupsStrategyMustRunAs,
	}
	scc.SeccompProfiles = []string{
		"runtime/default",
	}
	scc.DefaultAddCapabilities = nil
	scc.RequiredDropCapabilities = []corev1.Capability{
		"ALL",
	}
	scc.Volumes = []secv1.FSType{
		secv1.FSTypeConfigMap,
		secv1.FSTypeDownwardAPI,
		secv1.FSTypeEmptyDir,
		secv1.FSTypePersistentVolumeClaim,
		secv1.FSProjected,
		secv1.FSTypeSecret,
	}
}

func ensureSCCExists(ctx context.Context, logger logr.Logger, c client.Client, saNamespace, saName, cronSaName string) (bool, error) {
	scc := &secv1.SecurityContextConstraints{}
	userName := fmt.Sprintf("system:serviceaccount:%s:%s", saNamespace, saName)
	cronUserName := fmt.Sprintf("system:serviceaccount:%s:%s", saNamespace, cronSaName)

	err := c.Get(ctx, client.ObjectKey{Name: sccName}, scc)
	if meta.IsNoMatchError(err) {
		// not in openshift
		logger.V(3).Info("No match error for SCC, must not be in openshift")
		return false, nil
	} else if errors.IsNotFound(err) {
		cr, err := cc.GetActiveCDI(ctx, c)
		if err != nil {
			return false, err
		}
		if cr == nil {
			return false, fmt.Errorf("no active CDI")
		}
		installerLabels := util.GetRecommendedInstallerLabelsFromCr(cr)

		scc = &secv1.SecurityContextConstraints{
			ObjectMeta: metav1.ObjectMeta{
				Name: sccName,
				Labels: map[string]string{
					"cdi.kubevirt.io": "",
				},
			},
			Users: []string{
				userName,
				cronUserName,
			},
		}

		setSCC(scc)

		util.SetRecommendedLabels(scc, installerLabels, "cdi-operator")

		if err = operator.SetOwnerRuntime(c, scc); err != nil {
			return false, err
		}

		if err := c.Create(ctx, scc); err != nil {
			return false, err
		}

		return true, nil
	} else if err != nil {
		return false, err
	}

	origSCC := scc.DeepCopy()

	setSCC(scc)

	if !sdk.ContainsStringValue(scc.Users, userName) {
		scc.Users = append(scc.Users, userName)
	}
	if !sdk.ContainsStringValue(scc.Users, cronUserName) {
		scc.Users = append(scc.Users, cronUserName)
	}

	if !apiequality.Semantic.DeepEqual(origSCC, scc) {
		if err := c.Update(context.TODO(), scc); err != nil {
			return false, err
		}

		return true, nil
	}

	return false, nil
}

func (r *ReconcileCDI) watchSecurityContextConstraints() error {
	err := r.uncachedClient.List(context.TODO(), &secv1.SecurityContextConstraintsList{}, &client.ListOptions{
		Limit: 1,
	})
	if err == nil {
		var scc client.Object = &secv1.SecurityContextConstraints{}
		return r.controller.Watch(source.Kind(r.getCache(), scc, enqueueCDI(r.client)))
	}
	if meta.IsNoMatchError(err) {
		log.Info("Not watching SecurityContextConstraints")
		return nil
	}

	return err
}
