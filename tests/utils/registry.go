package utils

const (
	TinyCoreImageName    = "cdi-func-test-tinycore"
	CirrosQcow2ImageName = "cdi-func-test-cirros-qcow2"
)

type RegistryImageBuilder struct {
	scheme string
	base   string
	image  string
	tag    string
}

type RegistryOption func(r *RegistryImageBuilder)

func NewRegistryImage(opts ...RegistryOption) *RegistryImageBuilder {
	registryImageBuilder := &RegistryImageBuilder{}
	for _, f := range opts {
		f(registryImageBuilder)
	}
	return registryImageBuilder
}

func (r *RegistryImageBuilder) String() string {
	url := r.scheme + r.base + "/" + r.image
	if r.tag != "" {
		url += ":" + r.tag
	}
	return url
}

func WithBase(base string) RegistryOption {
	return func(r *RegistryImageBuilder) {
		r.base = base
	}
}

func WithImage(image string) RegistryOption {
	return func(r *RegistryImageBuilder) {
		r.image = image
	}
}

func WithScheme(scheme string) RegistryOption {
	return func(r *RegistryImageBuilder) {
		r.scheme = scheme
	}
}

func WithTag(tag string) RegistryOption {
	return func(r *RegistryImageBuilder) {
		r.tag = tag
	}
}

func WithDocker() RegistryOption {
	return WithScheme("docker://")
}
