package ovirtclient

import (
	ovirtsdk4 "github.com/ovirt/go-ovirt"
)

//go:generate go run scripts/rest/rest.go -i "Tag" -n "tag"

// TagClient describes the functions related to oVirt tags.
type TagClient interface {
	// GetTag returns a single tag based on its ID.
	GetTag(id string, retries ...RetryStrategy) (Tag, error)
	// ListTags returns all tags on the oVirt engine.
	ListTags(retries ...RetryStrategy) ([]Tag, error)
	// CreateTag creates a new tag with a name and description.
	CreateTag(name string, description string, retries ...RetryStrategy) (result Tag, err error)
	// RemoveTag removes the tag with the specified ID.
	RemoveTag(tagID string, retries ...RetryStrategy) error
}

// TagData is the core of Tag, providing only the data access functions, but not the client
// functions.
type TagData interface {
	// ID returns the auto-generated identifier for this tag.
	ID() string
	// Name returns the user-give name for this tag.
	Name() string
	// Description returns the user-give description for this tag.
	Description() string
}

// Tag is the interface defining the fields for tag.
type Tag interface {
	TagData
	Remove(retries ...RetryStrategy) error
}

func convertSDKTag(sdkObject *ovirtsdk4.Tag, client *oVirtClient) (Tag, error) {
	id, ok := sdkObject.Id()
	if !ok {
		return nil, newFieldNotFound("tag", "id")
	}
	name, ok := sdkObject.Name()
	if !ok {
		return nil, newFieldNotFound("tag", name)
	}
	description, ok := sdkObject.Description()
	if !ok {
		return nil, newFieldNotFound("tag", description)
	}
	return &tag{
		client:      client,
		id:          id,
		name:        name,
		description: description,
	}, nil
}

type tag struct {
	client      Client
	id          string
	name        string
	description string
}

func (n tag) ID() string {
	return n.id
}

func (n tag) Name() string {
	return n.name
}

func (n tag) Description() string {
	return n.description
}

func (n *tag) Remove(retries ...RetryStrategy) error {
	return n.client.RemoveTag(n.id, retries...)
}
