package validatingwebhook

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"k8s.io/client-go/kubernetes"

	"k8s.io/api/admission/v1beta1"
	authentication "k8s.io/api/authentication/v1"
	authorization "k8s.io/api/authorization/v1"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sfield "k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/klog"

	cdicorev1alpha1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"
	"kubevirt.io/containerized-data-importer/pkg/controller"
)

// ValidatingWebhook is the interfave implemented by webhooks
type ValidatingWebhook interface {
	http.Handler
	Admit(*v1beta1.AdmissionReview) *v1beta1.AdmissionResponse
}

type dataVolumeWebhook struct {
	client kubernetes.Interface
}

type pvcWebhook struct {
	client kubernetes.Interface
}

var _ ValidatingWebhook = &dataVolumeWebhook{}

var _ ValidatingWebhook = &pvcWebhook{}

// NewDataVolumeWebhook creates a new DataVolume webhook
func NewDataVolumeWebhook(client kubernetes.Interface) ValidatingWebhook {
	return &dataVolumeWebhook{client: client}
}

// NewPVCWebhook creates a new DataVolume webhook
func NewPVCWebhook(client kubernetes.Interface) ValidatingWebhook {
	return &pvcWebhook{client: client}
}

func toAdmissionReview(r *http.Request) (*v1beta1.AdmissionReview, error) {
	var body []byte
	if r.Body != nil {
		if data, err := ioutil.ReadAll(r.Body); err == nil {
			body = data
		}
	}

	// verify the content type is accurate
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		return nil, fmt.Errorf("contentType=%s, expect application/json", contentType)
	}

	ar := &v1beta1.AdmissionReview{}
	err := json.Unmarshal(body, ar)
	return ar, err
}

func toRejectedAdmissionResponse(causes []metav1.StatusCause) *v1beta1.AdmissionResponse {
	globalMessage := ""
	for _, cause := range causes {
		globalMessage = fmt.Sprintf("%s %s", globalMessage, cause.Message)
	}

	return &v1beta1.AdmissionResponse{
		Result: &metav1.Status{
			Message: globalMessage,
			Code:    http.StatusUnprocessableEntity,
			Details: &metav1.StatusDetails{
				Causes: causes,
			},
		},
	}
}

func toAdmissionResponseError(err error) *v1beta1.AdmissionResponse {
	klog.Infof("Returning admission response error %s", err)
	return &v1beta1.AdmissionResponse{
		Result: &metav1.Status{
			Message: err.Error(),
			Code:    http.StatusBadRequest,
		},
	}
}

func validateSourceURL(sourceURL string) string {
	if sourceURL == "" {
		return "source URL is empty"
	}
	url, err := url.ParseRequestURI(sourceURL)
	if err != nil {
		return fmt.Sprintf("Invalid source URL: %s", sourceURL)
	}
	if url.Scheme != "http" && url.Scheme != "https" {
		return fmt.Sprintf("Invalid source URL scheme: %s", sourceURL)
	}
	return ""
}

func (wh *dataVolumeWebhook) validateDataVolumeSpec(field *k8sfield.Path, spec *cdicorev1alpha1.DataVolumeSpec, userInfo authentication.UserInfo) []metav1.StatusCause {
	var causes []metav1.StatusCause
	var url string
	var sourceType string
	// spec source field should not be empty
	if &spec.Source == nil || (spec.Source.HTTP == nil && spec.Source.S3 == nil && spec.Source.PVC == nil && spec.Source.Upload == nil &&
		spec.Source.Blank == nil && spec.Source.Registry == nil) {
		causes = append(causes, metav1.StatusCause{
			Type:    metav1.CauseTypeFieldValueInvalid,
			Message: fmt.Sprintf("Missing Data volume source"),
			Field:   field.Child("source").String(),
		})
		return causes
	}

	if (spec.Source.HTTP != nil && (spec.Source.S3 != nil || spec.Source.PVC != nil || spec.Source.Upload != nil || spec.Source.Blank != nil || spec.Source.Registry != nil)) ||
		(spec.Source.S3 != nil && (spec.Source.PVC != nil || spec.Source.Upload != nil || spec.Source.Blank != nil || spec.Source.Registry != nil)) ||
		(spec.Source.PVC != nil && (spec.Source.Upload != nil || spec.Source.Blank != nil || spec.Source.Registry != nil)) ||
		(spec.Source.Upload != nil && (spec.Source.Blank != nil || spec.Source.Registry != nil)) ||
		(spec.Source.Blank != nil && spec.Source.Registry != nil) {
		causes = append(causes, metav1.StatusCause{
			Type:    metav1.CauseTypeFieldValueInvalid,
			Message: fmt.Sprintf("Multiple Data volume sources"),
			Field:   field.Child("source").String(),
		})
		return causes
	}
	// if source types are HTTP or S3, check if URL is valid
	if spec.Source.HTTP != nil || spec.Source.S3 != nil {
		if spec.Source.HTTP != nil {
			url = spec.Source.HTTP.URL
			sourceType = field.Child("source", "HTTP", "url").String()
		} else if spec.Source.S3 != nil {
			url = spec.Source.S3.URL
			sourceType = field.Child("source", "S3", "url").String()
		}
		err := validateSourceURL(url)
		if err != "" {
			causes = append(causes, metav1.StatusCause{
				Type:    metav1.CauseTypeFieldValueInvalid,
				Message: fmt.Sprintf("%s %s", field.Child("source").String(), err),
				Field:   sourceType,
			})
			return causes
		}
	}

	// Make sure contentType is either empty (kubevirt), or kubevirt or archive
	if spec.ContentType != "" && string(spec.ContentType) != string(cdicorev1alpha1.DataVolumeKubeVirt) && string(spec.ContentType) != string(cdicorev1alpha1.DataVolumeArchive) {
		sourceType = field.Child("contentType").String()
		causes = append(causes, metav1.StatusCause{
			Type:    metav1.CauseTypeFieldValueInvalid,
			Message: fmt.Sprintf("ContentType not one of: %s, %s", cdicorev1alpha1.DataVolumeKubeVirt, cdicorev1alpha1.DataVolumeArchive),
			Field:   sourceType,
		})
		return causes
	}

	if spec.Source.Blank != nil && string(spec.ContentType) == string(cdicorev1alpha1.DataVolumeArchive) {
		sourceType = field.Child("contentType").String()
		causes = append(causes, metav1.StatusCause{
			Type:    metav1.CauseTypeFieldValueInvalid,
			Message: fmt.Sprintf("SourceType cannot be blank and the contentType be archive"),
			Field:   sourceType,
		})
		return causes
	}

	if spec.Source.Registry != nil && spec.ContentType != "" && string(spec.ContentType) != string(cdicorev1alpha1.DataVolumeKubeVirt) {
		sourceType = field.Child("contentType").String()
		causes = append(causes, metav1.StatusCause{
			Type:    metav1.CauseTypeFieldValueInvalid,
			Message: fmt.Sprintf("ContentType must be " + string(cdicorev1alpha1.DataVolumeKubeVirt) + " when Source is Registry"),
			Field:   sourceType,
		})
		return causes
	}

	if spec.Source.PVC != nil {
		if spec.Source.PVC.Namespace == "" || spec.Source.PVC.Name == "" {
			causes = append(causes, metav1.StatusCause{
				Type:    metav1.CauseTypeFieldValueInvalid,
				Message: fmt.Sprintf("%s source PVC is not valid", field.Child("source", "PVC").String()),
				Field:   field.Child("source", "PVC").String(),
			})
			return causes
		}

		sourcePVC, err := wh.client.CoreV1().PersistentVolumeClaims(spec.Source.PVC.Namespace).Get(spec.Source.PVC.Name, metav1.GetOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				causes = append(causes, metav1.StatusCause{
					Type:    metav1.CauseTypeFieldValueNotFound,
					Message: fmt.Sprintf("Source PVC %s/%s doesn't exist", spec.Source.PVC.Namespace, spec.Source.PVC.Name),
					Field:   field.Child("source", "PVC").String(),
				})
				return causes
			}
		}
		err = controller.ValidateCanCloneSourceAndTargetSpec(&sourcePVC.Spec, spec.PVC)
		if err != nil {
			causes = append(causes, metav1.StatusCause{
				Type:    metav1.CauseTypeFieldValueInvalid,
				Message: err.Error(),
				Field:   field.Child("PVC").String(),
			})
			return causes
		}

		ok, reason, err := canCreatePodInNamespace(wh.client, spec.Source.PVC.Namespace, userInfo)
		if err != nil {
			causes = append(causes, metav1.StatusCause{
				Type:    metav1.CauseTypeUnexpectedServerResponse,
				Message: err.Error(),
				Field:   field.Child("source", "PVC", "namespace").String(),
			})
			return causes
		}

		if !ok {
			causes = append(causes, metav1.StatusCause{
				Type:    metav1.CauseTypeFieldValueInvalid,
				Message: reason,
				Field:   field.Child("source", "PVC", "namespace").String(),
			})
			return causes
		}
	}

	if spec.PVC == nil {
		causes = append(causes, metav1.StatusCause{
			Type:    metav1.CauseTypeFieldValueInvalid,
			Message: fmt.Sprintf("Missing Data volume PVC"),
			Field:   field.Child("PVC").String(),
		})
		return causes
	}
	if pvcSize, ok := spec.PVC.Resources.Requests["storage"]; ok {
		if pvcSize.IsZero() || pvcSize.Value() < 0 {
			causes = append(causes, metav1.StatusCause{
				Type:    metav1.CauseTypeFieldValueInvalid,
				Message: fmt.Sprintf("PVC size can't be equal or less than zero"),
				Field:   field.Child("PVC", "resources", "requests", "size").String(),
			})
			return causes
		}
	} else {
		causes = append(causes, metav1.StatusCause{
			Type:    metav1.CauseTypeFieldValueInvalid,
			Message: fmt.Sprintf("PVC size is missing"),
			Field:   field.Child("PVC", "resources", "requests", "size").String(),
		})
		return causes
	}

	accessModes := spec.PVC.AccessModes
	if len(accessModes) > 1 {
		causes = append(causes, metav1.StatusCause{
			Type:    metav1.CauseTypeFieldValueInvalid,
			Message: fmt.Sprintf("PVC multiple accessModes"),
			Field:   field.Child("PVC", "accessModes").String(),
		})
		return causes
	}
	// We know we have one access mode
	if accessModes[0] != v1.ReadWriteOnce && accessModes[0] != v1.ReadOnlyMany && accessModes[0] != v1.ReadWriteMany {
		causes = append(causes, metav1.StatusCause{
			Type:    metav1.CauseTypeFieldValueInvalid,
			Message: fmt.Sprintf("Unsupported value: \"%s\": supported values: \"ReadOnlyMany\", \"ReadWriteMany\", \"ReadWriteOnce\"", string(accessModes[0])),
			Field:   field.Child("PVC", "accessModes").String(),
		})
		return causes
	}
	return causes
}

func (wh *dataVolumeWebhook) Admit(ar *v1beta1.AdmissionReview) *v1beta1.AdmissionResponse {
	resource := metav1.GroupVersionResource{
		Group:    cdicorev1alpha1.SchemeGroupVersion.Group,
		Version:  cdicorev1alpha1.SchemeGroupVersion.Version,
		Resource: "datavolumes",
	}
	if ar.Request.Resource != resource {
		klog.Errorf("resource is %s but request is: %s", resource, ar.Request.Resource)
		err := fmt.Errorf("expect resource to be '%s'", resource.Resource)
		return toAdmissionResponseError(err)
	}

	raw := ar.Request.Object.Raw
	dv := cdicorev1alpha1.DataVolume{}

	err := json.Unmarshal(raw, &dv)

	if err != nil {
		return toAdmissionResponseError(err)
	}

	pvcs, err := wh.client.CoreV1().PersistentVolumeClaims(dv.GetNamespace()).List(metav1.ListOptions{})
	if err != nil {
		return toAdmissionResponseError(err)
	}
	if ar.Request.Operation == v1beta1.Create {
		for _, pvc := range pvcs.Items {
			if pvc.Name == dv.GetName() {
				klog.Errorf("destination PVC %s/%s already exists", dv.GetNamespace(), dv.GetName())
				var causes []metav1.StatusCause
				causes = append(causes, metav1.StatusCause{
					Type:    metav1.CauseTypeFieldValueDuplicate,
					Message: fmt.Sprintf("Destination PVC already exists"),
					Field:   k8sfield.NewPath("DataVolume").Child("Name").String(),
				})
				return toRejectedAdmissionResponse(causes)
			}
		}
	}

	causes := wh.validateDataVolumeSpec(k8sfield.NewPath("spec"), &dv.Spec, ar.Request.UserInfo)
	if len(causes) > 0 {
		klog.Infof("rejected DataVolume admission")
		return toRejectedAdmissionResponse(causes)
	}

	return allowed()
}

func allowed() *v1beta1.AdmissionResponse {
	response := &v1beta1.AdmissionResponse{}
	response.Allowed = true
	return response
}

func serve(wh ValidatingWebhook, resp http.ResponseWriter, req *http.Request) {

	response := v1beta1.AdmissionReview{}
	review, err := toAdmissionReview(req)

	if err != nil {
		resp.WriteHeader(http.StatusBadRequest)
		return
	}

	reviewResponse := wh.Admit(review)
	if reviewResponse != nil {
		response.Response = reviewResponse
		response.Response.UID = review.Request.UID
	}
	// reset the Object and OldObject, they are not needed in a response.
	review.Request.Object = runtime.RawExtension{}
	review.Request.OldObject = runtime.RawExtension{}

	responseBytes, err := json.Marshal(response)
	if err != nil {
		klog.Errorf("failed json encode webhook response: %s", err)
		resp.WriteHeader(http.StatusBadRequest)
		return
	}
	if _, err := resp.Write(responseBytes); err != nil {
		klog.Errorf("failed to write webhook response: %s", err)
		resp.WriteHeader(http.StatusBadRequest)
		return
	}
	resp.WriteHeader(http.StatusOK)
}

func (wh *dataVolumeWebhook) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	serve(wh, resp, req)
}

func (wh *pvcWebhook) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	serve(wh, resp, req)
}

func (wh *pvcWebhook) Admit(ar *v1beta1.AdmissionReview) *v1beta1.AdmissionResponse {
	resource := metav1.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "persistentvolumeclaims",
	}
	if ar.Request.Resource != resource {
		klog.Errorf("resource is %s but request is: %s", resource, ar.Request.Resource)
		err := fmt.Errorf("expect resource to be '%s'", resource.Resource)
		return toAdmissionResponseError(err)
	}

	pvc, oldPVC := &v1.PersistentVolumeClaim{}, &v1.PersistentVolumeClaim{}

	err := json.Unmarshal(ar.Request.Object.Raw, &pvc)
	if err != nil {
		return toAdmissionResponseError(err)
	}

	cloneSrc, exists := pvc.Annotations[controller.AnnCloneRequest]
	if !exists {
		return allowed()
	}

	if len(ar.Request.OldObject.Raw) > 0 {
		err := json.Unmarshal(ar.Request.OldObject.Raw, &oldPVC)
		if err != nil {
			return toAdmissionResponseError(err)
		}

		oldCloneSrc, exists := oldPVC.Annotations[controller.AnnCloneRequest]
		if exists && cloneSrc == oldCloneSrc {
			// already checked
			return allowed()
		}
	}

	namespace, name := controller.ParseSourcePvcAnnotation(cloneSrc, "/")
	if namespace == "" || name == "" {
		return allowed()
	}

	ok, reason, err := canCreatePodInNamespace(wh.client, namespace, ar.Request.UserInfo)
	if err != nil {
		return toAdmissionResponseError(err)
	}

	if !ok {
		causes := []metav1.StatusCause{
			{
				Type:    metav1.CauseTypeFieldValueInvalid,
				Message: reason,
				Field:   k8sfield.NewPath("PersistentVolumeClaim").Child("Annotations").String(),
			},
		}
		return toRejectedAdmissionResponse(causes)
	}

	return allowed()
}

func canCreatePodInNamespace(client kubernetes.Interface, namespace string, userInfo authentication.UserInfo) (bool, string, error) {
	var newExtra map[string]authorization.ExtraValue
	if len(userInfo.Extra) > 0 {
		newExtra = make(map[string]authorization.ExtraValue)
		for k, v := range userInfo.Extra {
			newExtra[k] = authorization.ExtraValue(v)
		}
	}

	sar := &authorization.SubjectAccessReview{
		Spec: authorization.SubjectAccessReviewSpec{
			User:   userInfo.Username,
			Groups: userInfo.Groups,
			Extra:  newExtra,
			ResourceAttributes: &authorization.ResourceAttributes{
				Namespace: namespace,
				Verb:      "create",
				Group:     "",
				Version:   "v1",
				Resource:  "pods",
			},
		},
	}

	klog.V(3).Infof("Sending SubjectAccessReview %+v", sar)

	response, err := client.AuthorizationV1().SubjectAccessReviews().Create(sar)
	if err != nil {
		return false, "", err
	}

	klog.V(3).Infof("SubjectAccessReview response %+v", response)

	if !response.Status.Allowed {
		return false, fmt.Sprintf("User %s has insufficient permissions in clone source namespace %s", userInfo.Username, namespace), nil
	}

	return true, "", nil
}
