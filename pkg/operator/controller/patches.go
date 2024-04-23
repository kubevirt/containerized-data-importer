package controller

import (
	"crypto/sha1" //nolint:gosec // See #nosec directive
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"

	jsonpatch "github.com/evanphx/json-patch/v5"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
)

// The Customizer structure is used for customizing components with a collection of patches.
// It includes an array of CustomizeComponentsPatch, along with a hash value.
// The Apply method allows applying these patches to a group of objects.
type Customizer struct {
	Patches []v1beta1.CustomizeComponentsPatch
	hash    string
}

// Hash provides the hash of the patches.
func (c *Customizer) Hash() string {
	return c.hash
}

// GetPatches provides slice of patches.
func (c *Customizer) GetPatches() []v1beta1.CustomizeComponentsPatch {
	return c.Patches
}

// GetPatchesForResource provides slice of patches for specific resource.
func (c *Customizer) GetPatchesForResource(resourceType, name string) []v1beta1.CustomizeComponentsPatch {
	allPatches := c.Patches
	patches := make([]v1beta1.CustomizeComponentsPatch, 0)

	for _, p := range allPatches {
		if valueMatchesKey(p.ResourceType, resourceType) && valueMatchesKey(p.ResourceName, name) {
			patches = append(patches, p)
		}
	}

	return patches
}

func valueMatchesKey(value, key string) bool {
	if value == "*" {
		return true
	}

	return strings.EqualFold(key, value)
}

// Apply applies all patches to the slice of objects.
func (c *Customizer) Apply(objects []client.Object) error {
	var deployments []*appsv1.Deployment
	var services []*corev1.Service
	var validatingWebhooks []*admissionregistrationv1.ValidatingWebhookConfiguration
	var mutatingWebhooks []*admissionregistrationv1.MutatingWebhookConfiguration
	var apiServices []*apiregistrationv1.APIService

	for _, obj := range objects {
		kind := obj.GetObjectKind().GroupVersionKind().Kind
		switch kind {
		case "Deployment":
			deployments = append(deployments, obj.(*appsv1.Deployment))
		case "Service":
			services = append(services, obj.(*corev1.Service))
		case "ValidatingWebhookConfiguration":
			validatingWebhooks = append(validatingWebhooks, obj.(*admissionregistrationv1.ValidatingWebhookConfiguration))
		case "MutatingWebhookConfiguration":
			mutatingWebhooks = append(mutatingWebhooks, obj.(*admissionregistrationv1.MutatingWebhookConfiguration))
		case "APIService":
			apiServices = append(apiServices, obj.(*apiregistrationv1.APIService))
		}
	}

	err := c.GenericApplyPatches(deployments)
	if err != nil {
		return err
	}
	err = c.GenericApplyPatches(services)
	if err != nil {
		return err
	}
	err = c.GenericApplyPatches(validatingWebhooks)
	if err != nil {
		return err
	}
	err = c.GenericApplyPatches(mutatingWebhooks)
	if err != nil {
		return err
	}
	err = c.GenericApplyPatches(apiServices)
	if err != nil {
		return err
	}
	return nil
}

// GenericApplyPatches applies patches to a slice of resources.
func (c *Customizer) GenericApplyPatches(objects interface{}) error {
	switch reflect.TypeOf(objects).Kind() {
	case reflect.Slice:
		s := reflect.ValueOf(objects)
		for i := 0; i < s.Len(); i++ {
			o := s.Index(i)
			obj, ok := o.Interface().(runtime.Object)
			if !ok {
				return errors.New("slice must contain objects of type 'runtime.Object'")
			}

			kind := obj.GetObjectKind().GroupVersionKind().Kind

			v := reflect.Indirect(o).FieldByName("ObjectMeta").FieldByName("Name")
			name := v.String()

			patches := c.GetPatchesForResource(kind, name)

			if len(patches) > 0 {
				patches = append(patches, v1beta1.CustomizeComponentsPatch{
					Patch: fmt.Sprintf(`{"metadata":{"annotations":{"%s":"%s"}}}`, cc.AnnCdiCustomizeComponentHash, c.hash),
					Type:  v1beta1.StrategicMergePatchType,
				})
				if err := applyPatches(obj, patches); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func applyPatches(obj runtime.Object, patches []v1beta1.CustomizeComponentsPatch) error {
	if len(patches) == 0 {
		return nil
	}

	for _, p := range patches {
		err := applyPatch(obj, p)
		if err != nil {
			return err
		}
	}

	return nil
}

func applyPatch(obj runtime.Object, patch v1beta1.CustomizeComponentsPatch) error {
	if obj == nil {
		return nil
	}

	old, err := json.Marshal(obj)
	if err != nil {
		return err
	}

	// reset the object in preparation to unmarshal, since unmarshal does not guarantee that fields
	// in obj that are removed by patch are cleared
	value := reflect.ValueOf(obj)
	value.Elem().Set(reflect.New(value.Type().Elem()).Elem())

	switch patch.Type {
	case v1beta1.JSONPatchType:
		patch, err := jsonpatch.DecodePatch([]byte(patch.Patch))
		if err != nil {
			return err
		}
		opts := jsonpatch.NewApplyOptions()
		opts.AllowMissingPathOnRemove = true
		opts.EnsurePathExistsOnAdd = true
		modified, err := patch.ApplyWithOptions(old, opts)
		if err != nil {
			return err
		}

		if err = json.Unmarshal(modified, obj); err != nil {
			return err
		}
	case v1beta1.MergePatchType:
		modified, err := jsonpatch.MergePatch(old, []byte(patch.Patch))
		if err != nil {
			return err
		}

		if err := json.Unmarshal(modified, obj); err != nil {
			return err
		}
	case v1beta1.StrategicMergePatchType:
		mergedByte, err := strategicpatch.StrategicMergePatch(old, []byte(patch.Patch), obj)
		if err != nil {
			return err
		}

		if err = json.Unmarshal(mergedByte, obj); err != nil {
			return err
		}
	default:
		return fmt.Errorf("PatchType is not supported")
	}

	return nil
}

// NewCustomizer returns a new Customizer.
func NewCustomizer(customizations v1beta1.CustomizeComponents) (*Customizer, error) {
	hash, err := getHash(customizations)
	if err != nil {
		return &Customizer{}, err
	}

	patches := customizations.Patches
	flagPatches := flagsToPatches(customizations.Flags)
	patches = append(patches, flagPatches...)

	return &Customizer{
		Patches: patches,
		hash:    hash,
	}, nil
}

func flagsToPatches(flags *v1beta1.Flags) []v1beta1.CustomizeComponentsPatch {
	patches := []v1beta1.CustomizeComponentsPatch{}
	if flags == nil {
		return patches
	}
	patches = addFlagsPatch(common.CDIApiServerResourceName, "Deployment", flags.API, patches)
	patches = addFlagsPatch(common.CDIControllerResourceName, "Deployment", flags.Controller, patches)
	patches = addFlagsPatch(common.CDIUploadProxyResourceName, "Deployment", flags.UploadProxy, patches)

	return patches
}

func addFlagsPatch(name, resource string, flags map[string]string, patches []v1beta1.CustomizeComponentsPatch) []v1beta1.CustomizeComponentsPatch {
	if len(flags) == 0 {
		return patches
	}

	return append(patches, v1beta1.CustomizeComponentsPatch{
		ResourceName: name,
		ResourceType: resource,
		Patch:        fmt.Sprintf(`{"spec":{"template":{"spec":{"containers":[{"name":%q,"args":["%s"]}]}}}}`, name, strings.Join(flagsToArray(flags), `","`)),
		Type:         v1beta1.StrategicMergePatchType,
	})
}

func flagsToArray(flags map[string]string) []string {
	farr := make([]string, 0)

	for flag, v := range flags {
		farr = append(farr, fmt.Sprintf("-%s", strings.ToLower(flag)))
		if v != "" {
			farr = append(farr, v)
		}
	}

	return farr
}

func getHash(customizations v1beta1.CustomizeComponents) (string, error) {
	// #nosec CWE: 326 - Use of weak cryptographic primitive (http://cwe.mitre.org/data/definitions/326.html)
	// reason: sha1 is not used for encryption but for creating a hash value
	hasher := sha1.New()

	sort.SliceStable(customizations.Patches, func(i, j int) bool {
		return len(customizations.Patches[i].Patch) < len(customizations.Patches[j].Patch)
	})

	values, err := json.Marshal(customizations)
	if err != nil {
		return "", err
	}
	hasher.Write(values)

	return hex.EncodeToString(hasher.Sum(nil)), nil
}
