package ovirtclient

import (
	ovirtsdk4 "github.com/ovirt/go-ovirt"
)

//go:generate go run scripts/rest.go -i "StorageDomain" -n "storage domain"

// StorageDomainClient contains the portion of the goVirt API that deals with storage domains.
type StorageDomainClient interface {
	// ListStorageDomains lists all storage domains.
	ListStorageDomains(retries ...RetryStrategy) ([]StorageDomain, error)
	// GetStorageDomain returns a single storage domain, or an error if the storage domain could not be found.
	GetStorageDomain(id string, retries ...RetryStrategy) (StorageDomain, error)
}

// StorageDomain represents a storage domain returned from the oVirt Engine API.
type StorageDomain interface {
	// ID is the unique identified for the storage system connected to oVirt.
	ID() string
	// Name is the user-given name for the storage domain.
	Name() string
	// Available returns the number of available bytes on the storage domain
	Available() uint64
	// Status returns the status of the storage domain. This status may be unknown if the storage domain is external.
	// Check ExternalStatus as well.
	Status() StorageDomainStatus
	// ExternalStatus returns the external status of a storage domain.
	ExternalStatus() StorageDomainExternalStatus
}

// StorageDomainStatus represents the status a domain can be in. Either this status field, or the
// StorageDomainExternalStatus must be set.
//
// Note: this is not well documented due to missing source documentation. If you know something about these statuses
// please contribute here:
// https://github.com/oVirt/ovirt-engine-api-model/blob/master/src/main/java/types/StorageDomainStatus.java
type StorageDomainStatus string

const (
	// StorageDomainStatusActivating indicates that the storage domain is currently activating and will soon be active.
	StorageDomainStatusActivating StorageDomainStatus = "activating"
	// StorageDomainStatusActive is the normal status for a storage domain when it's working.
	StorageDomainStatusActive StorageDomainStatus = "active"
	// StorageDomainStatusDetaching is the status when it is being disconnected.
	StorageDomainStatusDetaching StorageDomainStatus = "detaching"
	// StorageDomainStatusInactive is an undocumented status of the storage domain.
	StorageDomainStatusInactive StorageDomainStatus = "inactive"
	// StorageDomainStatusLocked is an undocumented status of the storage domain.
	StorageDomainStatusLocked StorageDomainStatus = "locked"
	// StorageDomainStatusMaintenance is an undocumented status of the storage domain.
	StorageDomainStatusMaintenance StorageDomainStatus = "maintenance"
	// StorageDomainStatusMixed is an undocumented status of the storage domain.
	StorageDomainStatusMixed StorageDomainStatus = "mixed"
	// StorageDomainStatusPreparingForMaintenance is an undocumented status of the storage domain.
	StorageDomainStatusPreparingForMaintenance StorageDomainStatus = "preparing_for_maintenance"
	// StorageDomainStatusUnattached is an undocumented status of the storage domain.
	StorageDomainStatusUnattached StorageDomainStatus = "unattached"
	// StorageDomainStatusUnknown is an undocumented status of the storage domain.
	StorageDomainStatusUnknown StorageDomainStatus = "unknown"
	// StorageDomainStatusNA indicates that the storage domain does not have a status. Please check the external status
	// instead.
	StorageDomainStatusNA StorageDomainStatus = ""
)

// StorageDomainExternalStatus represents the status of an external storage domain. This status is updated externally.
//
// Note: this is not well-defined as the oVirt model has only a very generic description. See
// https://github.com/oVirt/ovirt-engine-api-model/blob/9869596c298925538d510de5019195b488970738/src/main/java/types/ExternalStatus.java
// for details.
type StorageDomainExternalStatus string

const (
	// StorageDomainExternalStatusNA represents an external status that is not applicable.
	// Most likely, the status should be obtained from StorageDomainStatus, since the
	// storage domain in question is not an external storage.
	StorageDomainExternalStatusNA StorageDomainExternalStatus = ""
	// StorageDomainExternalStatusError indicates an error state.
	StorageDomainExternalStatusError StorageDomainExternalStatus = "error"
	// StorageDomainExternalStatusFailure indicates a failure state.
	StorageDomainExternalStatusFailure StorageDomainExternalStatus = "failure"
	// StorageDomainExternalStatusInfo indicates an OK status, but there is information available for the administrator
	// that might be relevant.
	StorageDomainExternalStatusInfo StorageDomainExternalStatus = "info"
	// StorageDomainExternalStatusOk indicates a working status.
	StorageDomainExternalStatusOk StorageDomainExternalStatus = "ok"
	// StorageDomainExternalStatusWarning indicates that the storage domain has warnings that may be relevant for the
	// administrator.
	StorageDomainExternalStatusWarning StorageDomainExternalStatus = "warning"
)

func convertSDKStorageDomain(sdkStorageDomain *ovirtsdk4.StorageDomain, client Client) (StorageDomain, error) {
	id, ok := sdkStorageDomain.Id()
	if !ok {
		return nil, newError(EFieldMissing, "failed to fetch ID of storage domain")
	}
	name, ok := sdkStorageDomain.Name()
	if !ok {
		return nil, newError(EFieldMissing, "failed to fetch name of storage domain")
	}
	available, ok := sdkStorageDomain.Available()
	if !ok {
		// If this is not OK the status probably doesn't allow for reading disk space (e.g. unattached), so we return 0.
		available = 0
	}
	if available < 0 {
		return nil, newError(EBug, "invalid available bytes returned from storage domain: %d", available)
	}
	// It is OK for the storage domain status to not be present if the external status is present.
	status, _ := sdkStorageDomain.Status()
	// It is OK for the storage domain external status to not be present if the status is present.
	externalStatus, _ := sdkStorageDomain.ExternalStatus()
	if status == "" && externalStatus == "" {
		return nil, newError(EFieldMissing, "neither the status nor the external status is set for storage domain %s", id)
	}

	return &storageDomain{
		client: client,

		id:             id,
		name:           name,
		available:      uint64(available),
		status:         StorageDomainStatus(status),
		externalStatus: StorageDomainExternalStatus(externalStatus),
	}, nil
}

type storageDomain struct {
	client Client

	id             string
	name           string
	available      uint64
	status         StorageDomainStatus
	externalStatus StorageDomainExternalStatus
}

func (s storageDomain) ID() string {
	return s.id
}

func (s storageDomain) Name() string {
	return s.name
}

func (s storageDomain) Available() uint64 {
	return s.available
}

func (s storageDomain) Status() StorageDomainStatus {
	return s.status
}

func (s storageDomain) ExternalStatus() StorageDomainExternalStatus {
	return s.externalStatus
}
