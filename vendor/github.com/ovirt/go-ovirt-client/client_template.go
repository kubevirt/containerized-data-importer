package ovirtclient

import (
	ovirtsdk4 "github.com/ovirt/go-ovirt"
)

//go:generate go run scripts/rest.go -i "Template" -n "template"

// TemplateClient represents the portion of the client that deals with VM templates.
type TemplateClient interface {
	ListTemplates(retries ...RetryStrategy) ([]Template, error)
	GetTemplate(id string, retries ...RetryStrategy) (Template, error)
}

// Template is a set of prepared configurations for VMs.
type Template interface {
	// ID returns the identifier of the template. This is typically a UUID.
	ID() string
	// Name is the human-readable name for the template.
	Name() string
	// Description is a longer description for the template.
	Description() string
}

func convertSDKTemplate(sdkTemplate *ovirtsdk4.Template, client Client) (Template, error) {
	id, ok := sdkTemplate.Id()
	if !ok {
		return nil, newError(EFieldMissing, "template does not contain ID")
	}
	name, ok := sdkTemplate.Name()
	if !ok {
		return nil, newError(EFieldMissing, "template does not contain a name")
	}
	description, ok := sdkTemplate.Description()
	if !ok {
		return nil, newError(EFieldMissing, "template does not contain a description")
	}
	return &template{
		client:      client,
		id:          id,
		name:        name,
		description: description,
	}, nil
}

type template struct {
	client      Client
	id          string
	name        string
	description string
}

func (t template) ID() string {
	return t.id
}

func (t template) Name() string {
	return t.name
}

func (t template) Description() string {
	return t.description
}
