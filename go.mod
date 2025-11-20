module kubevirt.io/containerized-data-importer

go 1.24.0

require (
	cloud.google.com/go/storage v1.38.0
	github.com/appscode/jsonpatch v1.0.1
	github.com/aws/aws-sdk-go v1.44.302
	github.com/containers/image/v5 v5.32.0
	github.com/coreos/go-semver v0.3.1
	github.com/docker/go-units v0.5.0
	github.com/emicklei/go-restful/v3 v3.11.0
	github.com/evanphx/json-patch/v5 v5.9.0
	github.com/ghodss/yaml v1.0.0
	github.com/go-jose/go-jose/v3 v3.0.3
	github.com/go-logr/logr v1.4.2
	github.com/golang/snappy v0.0.4
	github.com/google/uuid v1.6.0
	github.com/gophercloud/gophercloud/v2 v2.0.0-rc.1
	github.com/gophercloud/utils/v2 v2.0.0-20240529145014-bdd9ea767dd2
	github.com/kelseyhightower/envconfig v1.4.0
	github.com/klauspost/compress v1.17.9
	github.com/kubernetes-csi/external-snapshotter/client/v6 v6.0.1
	github.com/kubernetes-csi/lib-volume-populator v1.2.1-0.20230316163120-b62a0eee2c56
	github.com/kubernetes-csi/volume-data-source-validator/client v0.0.0-20230911161012-c2e130d28434
	github.com/kubevirt/monitoring/pkg/metrics/parser v0.0.0-20230627123556-81a891d4462a
	github.com/onsi/ginkgo/v2 v2.19.0
	github.com/onsi/gomega v1.33.1
	github.com/openshift/api v0.0.0-20241107155230-d37bb9f7e380
	github.com/openshift/client-go v0.0.0-20241001162912-da6d55e4611f
	github.com/openshift/custom-resource-status v1.1.2
	github.com/openshift/library-go v0.0.0-20240621150525-4bb4238aef81
	github.com/operator-framework/operator-lifecycle-manager v0.0.0-20190725173916-b56e63a643cc
	github.com/ovirt/go-ovirt v0.0.0-20210809163552-d4276e35d3db
	github.com/ovirt/go-ovirt-client v0.9.0
	github.com/ovirt/go-ovirt-client-log-klog v1.0.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring v0.74.0
	github.com/prometheus/client_golang v1.19.1
	github.com/prometheus/client_model v0.6.1
	github.com/rhobs/operator-observability-toolkit v0.0.29
	github.com/robfig/cron/v3 v3.0.1
	github.com/rs/cors v1.7.0
	github.com/ulikunitz/xz v0.5.12
	github.com/vmware/govmomi v0.23.1
	go.uber.org/zap v1.26.0
	golang.org/x/sys v0.38.0
	google.golang.org/api v0.169.0
	gopkg.in/fsnotify.v1 v1.4.7
	k8s.io/api v0.31.5
	k8s.io/apiextensions-apiserver v0.31.5
	k8s.io/apimachinery v0.31.5
	k8s.io/apiserver v0.31.5
	k8s.io/client-go v8.0.0+incompatible
	k8s.io/cluster-bootstrap v0.31.5
	k8s.io/code-generator v0.31.5
	k8s.io/component-helpers v0.31.5
	k8s.io/klog/v2 v2.130.1
	k8s.io/kube-aggregator v0.31.5
	k8s.io/kube-openapi v0.0.0-20240228011516-70dd3763d340
	k8s.io/utils v0.0.0-20240921022957-49e7df575cb6
	kubevirt.io/containerized-data-importer-api v0.0.0
	kubevirt.io/controller-lifecycle-operator-sdk v0.2.7
	kubevirt.io/controller-lifecycle-operator-sdk/api v0.0.0-20220329064328-f3cc58c6ed90
	kubevirt.io/qe-tools v0.1.8
	libguestfs.org/libnbd v1.11.5
	sigs.k8s.io/controller-runtime v0.19.5
)

require (
	cloud.google.com/go v0.112.1 // indirect
	cloud.google.com/go/compute/metadata v0.3.0 // indirect
	cloud.google.com/go/iam v1.1.6 // indirect
	github.com/BurntSushi/toml v1.4.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/blang/semver v3.5.1+incompatible // indirect
	github.com/blang/semver/v4 v4.0.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/containers/libtrust v0.0.0-20230121012942-c1716e8a8d01 // indirect
	github.com/containers/ocicrypt v1.2.0 // indirect
	github.com/containers/storage v1.55.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/distribution/reference v0.6.0 // indirect
	github.com/docker/distribution v2.8.3+incompatible // indirect
	github.com/docker/docker v27.1.1+incompatible // indirect
	github.com/docker/docker-credential-helpers v0.8.2 // indirect
	github.com/docker/go-connections v0.5.0 // indirect
	github.com/evanphx/json-patch v5.6.0+incompatible // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/fsnotify/fsnotify v1.7.0 // indirect
	github.com/fxamacker/cbor/v2 v2.7.0 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-logr/zapr v1.3.0 // indirect
	github.com/go-openapi/jsonpointer v0.21.0 // indirect
	github.com/go-openapi/jsonreference v0.21.0 // indirect
	github.com/go-openapi/swag v0.23.0 // indirect
	github.com/go-task/slim-sprig/v3 v3.0.0 // indirect
	github.com/gofrs/uuid/v5 v5.2.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/gnostic-models v0.6.8 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/pprof v0.0.0-20240727154555-813a5fbdbec8 // indirect
	github.com/google/s2a-go v0.1.7 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.2 // indirect
	github.com/googleapis/gax-go/v2 v2.12.2 // indirect
	github.com/gorilla/mux v1.8.1 // indirect
	github.com/gorilla/websocket v1.5.0 // indirect
	github.com/grafana/regexp v0.0.0-20221122212121-6b5c0a4cb7fd // indirect
	github.com/imdario/mergo v0.3.16 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/pgzip v1.2.6 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mattn/go-sqlite3 v1.14.22 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/moby/spdystream v0.4.0 // indirect
	github.com/moby/sys/mountinfo v0.7.2 // indirect
	github.com/moby/sys/user v0.2.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/mxk/go-flowrate v0.0.0-20140419014527-cca7078d478f // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.0 // indirect
	github.com/opencontainers/runtime-spec v1.2.0 // indirect
	github.com/ovirt/go-ovirt-client-log/v2 v2.2.0 // indirect
	github.com/prometheus/common v0.55.0 // indirect
	github.com/prometheus/procfs v0.15.1 // indirect
	github.com/robfig/cron v1.2.0 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/syndtr/gocapability v0.0.0-20200815063812-42c35b437635 // indirect
	github.com/vbatts/tar-split v0.11.5 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.53.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.53.0 // indirect
	go.opentelemetry.io/otel v1.28.0 // indirect
	go.opentelemetry.io/otel/metric v1.28.0 // indirect
	go.opentelemetry.io/otel/trace v1.28.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/crypto v0.45.0 // indirect
	golang.org/x/exp v0.0.0-20240613232115-7f521ea00fb8 // indirect
	golang.org/x/mod v0.29.0 // indirect
	golang.org/x/net v0.47.0 // indirect
	golang.org/x/oauth2 v0.27.0 // indirect
	golang.org/x/sync v0.18.0 // indirect
	golang.org/x/term v0.37.0 // indirect
	golang.org/x/text v0.31.0 // indirect
	golang.org/x/time v0.5.0 // indirect
	golang.org/x/tools v0.38.0 // indirect
	golang.org/x/tools/go/packages/packagestest v0.1.1-deprecated // indirect
	gomodules.xyz/jsonpatch/v2 v2.4.0 // indirect
	google.golang.org/genproto v0.0.0-20240227224415-6ceb2ff114de // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20240528184218-531527333157 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240701130421-f6361c86f094 // indirect
	google.golang.org/grpc v1.65.0 // indirect
	google.golang.org/protobuf v1.34.2 // indirect
	gopkg.in/evanphx/json-patch.v4 v4.12.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/component-base v0.31.5 // indirect
	k8s.io/gengo/v2 v2.0.0-20240228010128-51d4e06bde70 // indirect
	sigs.k8s.io/json v0.0.0-20221116044647-bc3834ca7abd // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.4.1 // indirect
	sigs.k8s.io/yaml v1.4.0 // indirect
)

replace (
	github.com/chzyer/logex => github.com/chzyer/logex v1.2.1
	github.com/openshift/api => github.com/openshift/api v0.0.0-20250129180039-d15841be6bde
	github.com/openshift/client-go => github.com/openshift/client-go v0.0.0-20241107164952-923091dd2b1a
	github.com/openshift/library-go => github.com/openshift/library-go v0.0.0-20250128093732-a69305d8f397
	github.com/operator-framework/operator-lifecycle-manager => github.com/operator-framework/operator-lifecycle-manager v0.0.0-20190128024246-5eb7ae5bdb7a

	k8s.io/api => k8s.io/api v0.31.5
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.31.5
	k8s.io/apimachinery => k8s.io/apimachinery v0.31.5
	k8s.io/apiserver => k8s.io/apiserver v0.31.5
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.31.5
	k8s.io/client-go => k8s.io/client-go v0.31.5
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.31.5
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.31.5
	k8s.io/code-generator => k8s.io/code-generator v0.31.5
	k8s.io/component-base => k8s.io/component-base v0.31.5
	k8s.io/component-helpers => k8s.io/component-helpers v0.31.5
	k8s.io/controller-manager => k8s.io/controller-manager v0.31.5
	k8s.io/cri-api => k8s.io/cri-api v0.31.5
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.31.5
	k8s.io/dynamic-resource-allocation => dynamic-resource-allocation v0.31.5
	k8s.io/endpointslice => k8s.io/endpointslice v0.31.5
	k8s.io/kms => k8s.io/kms v0.31.5
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.31.5
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.31.5
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.31.5
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.31.5
	k8s.io/kubectl => k8s.io/kubectl v0.31.5
	k8s.io/kubelet => k8s.io/kubelet v0.31.5
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.31.5
	k8s.io/metrics => k8s.io/metrics v0.31.5
	k8s.io/mount-utils => k8s.io/mount-utils v0.31.5
	k8s.io/pod-security-admission => k8s.io/pod-security-admission v0.31.5
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.31.5
	k8s.io/sample-cli-plugin => k8s.io/sample-cli-plugin v0.31.5
	k8s.io/sample-controller => k8s.io/sample-controller v0.31.5

	kubevirt.io/containerized-data-importer-api => ./staging/src/kubevirt.io/containerized-data-importer-api
	kubevirt.io/controller-lifecycle-operator-sdk/api => kubevirt.io/controller-lifecycle-operator-sdk/api v0.0.0-20220329064328-f3cc58c6ed90
	sigs.k8s.io/controller-runtime => sigs.k8s.io/controller-runtime v0.19.5
)
