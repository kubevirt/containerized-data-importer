/*
Copyright 2025 The CDI Authors.

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
	"strings"

	popv1beta1 "github.com/kubernetes-csi/volume-data-source-validator/client/apis/volumepopulator/v1beta1"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/source"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	forklift "kubevirt.io/containerized-data-importer-api/pkg/apis/forklift/v1beta1"
	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
	"kubevirt.io/containerized-data-importer/pkg/util"
)

var volumePopulatorSources = []runtime.Object{
	&cdiv1.VolumeImportSource{},
	&cdiv1.VolumeCloneSource{},
	&cdiv1.VolumeUploadSource{},
	&forklift.OpenstackVolumePopulator{},
	&forklift.OvirtVolumePopulator{},
}

func ensureVolumePopulatorsExist(ctx context.Context, c client.Client, scheme *runtime.Scheme) (bool, error) {
	cdi, err := cc.GetActiveCDI(ctx, c)
	if err != nil {
		return false, err
	}
	if cdi == nil {
		return false, fmt.Errorf("no active CDI")
	}
	installerLabels := util.GetRecommendedInstallerLabelsFromCr(cdi)
	changed := false

	for _, source := range volumePopulatorSources {
		gvk, err := c.GroupVersionKindFor(source)
		if err != nil {
			return false, err
		}
		popName := strings.ToLower(gvk.Kind)
		desiredPop := &popv1beta1.VolumePopulator{
			ObjectMeta: metav1.ObjectMeta{
				Name: popName,
			},
		}

		result, err := controllerutil.CreateOrUpdate(ctx, c, desiredPop, func() error {
			util.SetRecommendedLabels(desiredPop, installerLabels, "cdi-operator")
			desiredPop.SourceKind = metav1.GroupKind(gvk.GroupKind())
			return controllerutil.SetControllerReference(cdi, desiredPop, scheme)
		})
		if err != nil {
			return false, fmt.Errorf("failed to create/update VolumePopulator %s: %w", popName, err)
		}

		if result != controllerutil.OperationResultNone {
			changed = true
		}
	}
	return changed, nil
}

func (r *ReconcileCDI) watchPopulators() error {
	if !r.haveVolumeDataSourceValidator {
		log.Info("Not watching VolumePopulators")
		return nil
	}
	var populator client.Object = &popv1beta1.VolumePopulator{}
	return r.controller.Watch(source.Kind(r.getCache(), populator, enqueueCDI(r.client)))
}

func haveVolumeDataSourceValidator(c client.Client) (bool, error) {
	err := c.List(context.TODO(), &popv1beta1.VolumePopulatorList{}, &client.ListOptions{
		Limit: 1,
	})
	if err != nil {
		if meta.IsNoMatchError(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
