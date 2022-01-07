module kubevirt.io/containerized-data-importer

go 1.16

require (
	github.com/appscode/jsonpatch v0.0.0-20190108182946-7c0e3b262f30
	github.com/aws/aws-sdk-go v1.25.48
	github.com/containers/image/v5 v5.5.1
	github.com/containers/storage v1.32.4 // indirect
	github.com/coreos/go-semver v0.3.0
	github.com/coreos/prometheus-operator v0.38.1-0.20200424145508-7e176fda06cc
	github.com/docker/go-units v0.4.0
	github.com/elazarl/goproxy v0.0.0-20190911111923-ecfe977594f1 // indirect
	github.com/emicklei/go-restful v2.10.0+incompatible
	github.com/emicklei/go-restful-openapi v1.2.0
	github.com/ghodss/yaml v1.0.0
	github.com/go-logr/logr v1.2.0
	github.com/go-openapi/spec v0.19.3
	github.com/golang/snappy v0.0.3
	github.com/google/uuid v1.3.0
	github.com/gorhill/cronexpr v0.0.0-20180427100037-88b0669f7d75
	github.com/kelseyhightower/envconfig v1.4.0
	github.com/kubernetes-csi/external-snapshotter/v2 v2.1.1
	github.com/mrnold/go-libnbd v1.4.1-cdi
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.17.0
	github.com/opencontainers/runc v1.0.3 // indirect
	github.com/openshift/api v0.0.0
	github.com/openshift/client-go v0.0.0
	github.com/openshift/custom-resource-status v0.0.0-20200602122900-c002fd1547ca
	github.com/openshift/library-go v0.0.0-20211220195323-eca2c467c492
	github.com/operator-framework/operator-lifecycle-manager v0.0.0-20190725173916-b56e63a643cc
	github.com/ovirt/go-ovirt v0.0.0-20210809163552-d4276e35d3db
	github.com/ovirt/go-ovirt-client v0.6.0
	github.com/ovirt/go-ovirt-client-log-klog v1.0.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.11.0
	github.com/prometheus/client_model v0.2.0
	github.com/rs/cors v1.7.0
	github.com/ulikunitz/xz v0.5.10
	github.com/vmware/govmomi v0.23.1
	golang.org/x/sys v0.0.0-20211029165221-6e7872819dc8
	gopkg.in/fsnotify.v1 v1.4.7
	gopkg.in/square/go-jose.v2 v2.5.1
	k8s.io/api v0.23.0
	k8s.io/apiextensions-apiserver v0.23.0
	k8s.io/apimachinery v0.23.0
	k8s.io/apiserver v0.23.0
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/cluster-bootstrap v0.0.0
	k8s.io/code-generator v0.23.0
	k8s.io/klog/v2 v2.30.0
	k8s.io/kube-aggregator v0.23.0
	k8s.io/kube-openapi v0.0.0-20211115234752-e816edb12b65
	kubevirt.io/containerized-data-importer-api v0.0.0
	kubevirt.io/controller-lifecycle-operator-sdk v0.2.3
	kubevirt.io/qe-tools v0.1.6
	sigs.k8s.io/controller-runtime v0.11.0
)

replace (
	github.com/aws/aws-sdk-go => github.com/aws/aws-sdk-go v1.15.77
	github.com/openshift/api => github.com/openshift/api v0.0.0-20211217221424-8779abfbd571
	github.com/openshift/client-go => github.com/openshift/client-go v0.0.0-20211209144617-7385dd6338e3
	github.com/openshift/library-go => github.com/mhenriks/library-go v0.0.0-20211221173158-d6e4bcc60f0a
	github.com/operator-framework/operator-lifecycle-manager => github.com/operator-framework/operator-lifecycle-manager v0.0.0-20190128024246-5eb7ae5bdb7a

	k8s.io/api => k8s.io/api v0.23.0
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.23.0
	k8s.io/apimachinery => k8s.io/apimachinery v0.23.0
	k8s.io/apiserver => k8s.io/apiserver v0.23.0
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.23.0
	k8s.io/client-go => k8s.io/client-go v0.23.0
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.23.0
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.23.0
	k8s.io/code-generator => k8s.io/code-generator v0.23.0
	k8s.io/component-base => k8s.io/component-base v0.23.0
	k8s.io/component-helpers => k8s.io/component-helpers v0.23.0
	k8s.io/controller-manager => k8s.io/controller-manager v0.23.0
	k8s.io/cri-api => k8s.io/cri-api v0.23.0
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.23.0
	k8s.io/klog/v2 => k8s.io/klog/v2 v2.30.0
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.23.0
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.23.0
	k8s.io/kube-openapi => k8s.io/kube-openapi v0.0.0-20211115234752-e816edb12b65
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.23.0
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.23.0
	k8s.io/kubectl => k8s.io/kubectl v0.23.0
	k8s.io/kubelet => k8s.io/kubelet v0.23.0
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.23.0
	k8s.io/metrics => k8s.io/metrics v0.23.0
	k8s.io/mount-utils => k8s.io/mount-utils v0.23.0
	k8s.io/pod-security-admission => k8s.io/pod-security-admission v0.23.0
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.23.0

	kubevirt.io/containerized-data-importer-api => ./staging/src/kubevirt.io/containerized-data-importer-api
)
