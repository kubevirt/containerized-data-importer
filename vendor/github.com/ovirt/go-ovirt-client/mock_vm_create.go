package ovirtclient

import (
	"fmt"
	"net"
	"time"

	"github.com/google/uuid"
)

func (m *mockClient) CreateVM(
	clusterID ClusterID,
	templateID TemplateID,
	name string,
	params OptionalVMParameters,
	retries ...RetryStrategy,
) (result VM, err error) {
	retries = defaultRetries(retries, defaultWriteTimeouts())

	if err := validateVMCreationParameters(clusterID, templateID, name, params); err != nil {
		return nil, err
	}
	if params == nil {
		params = &vmParams{}
	}
	if name == "" {
		return nil, newError(EBadArgument, "The name parameter is required for VM creation.")
	}
	err = retry(
		fmt.Sprintf("creating VM %s", name),
		m.logger,
		retries,
		func() error {
			m.lock.Lock()
			defer m.lock.Unlock()
			if _, ok := m.clusters[clusterID]; !ok {
				return newError(ENotFound, "cluster with ID %s not found", clusterID)
			}
			tpl, ok := m.templates[templateID]
			if !ok {
				return newError(ENotFound, "template with ID %s not found", templateID)
			}
			if tpl.status != TemplateStatusOK {
				return newError(EConflict, "template in status \"%s\"", tpl.status)
			}

			for _, vm := range m.vms {
				if vm.name == name {
					return newError(EConflict, "A VM with the name \"%s\" already exists.", name)
				}
			}

			cpu := m.createVMCPU(params, tpl)

			vm := m.createVM(name, params, clusterID, templateID, cpu)

			m.attachVMDisksFromTemplate(tpl, vm, params)

			if clone := params.Clone(); clone != nil && *clone {
				vm.templateID = "00000000-0000-0000-0000-000000000000"
			}

			m.vmIPs[vm.id] = map[string][]net.IP{}

			result = vm
			return nil
		},
	)

	return result, err
}

func (m *mockClient) createVM(
	name string,
	params OptionalVMParameters,
	clusterID ClusterID,
	templateID TemplateID,
	cpu *vmCPU,
) *vm {
	id := uuid.Must(uuid.NewUUID()).String()
	init := params.Initialization()
	if init == nil {
		init = &initialization{}
	}
	memory := int64(1073741824)
	if params.Memory() != nil {
		memory = *params.Memory()
	}
	var memPolicy *memoryPolicy
	if memoryPolicyParams := params.MemoryPolicy(); memoryPolicyParams != nil {
		var guaranteed *int64
		if guaranteedMemory := (*memoryPolicyParams).Guaranteed(); guaranteedMemory != nil {
			guaranteed = guaranteedMemory
		}
		memPolicy = &memoryPolicy{
			guaranteed,
		}
	}

	var pp *vmPlacementPolicy
	if params.PlacementPolicy() != nil {
		placementPolicyParams := *params.PlacementPolicy()
		pp = &vmPlacementPolicy{
			placementPolicyParams.Affinity(),
			placementPolicyParams.HostIDs(),
		}
	}

	vm := &vm{
		client:          m,
		id:              id,
		name:            name,
		comment:         params.Comment(),
		clusterID:       clusterID,
		templateID:      templateID,
		status:          VMStatusDown,
		cpu:             cpu,
		hugePages:       params.HugePages(),
		memory:          memory,
		initialization:  init,
		placementPolicy: pp,
		memoryPolicy:    memPolicy,
	}
	m.vms[id] = vm
	return vm
}

func (m *mockClient) attachVMDisksFromTemplate(tpl *template, vm *vm, params OptionalVMParameters) {
	m.vmDiskAttachmentsByVM[vm.id] = make(
		map[string]*diskAttachment,
		len(m.templateDiskAttachmentsByTemplate[tpl.id]),
	)
	for _, attachment := range m.templateDiskAttachmentsByTemplate[tpl.id] {
		disk := m.disks[attachment.diskID]
		var sparse *bool
		for _, diskParam := range params.Disks() {
			if diskParam.DiskID() == disk.ID() {
				sparse = diskParam.Sparse()
				if format := diskParam.Format(); format != nil {
					if *format != disk.Format() {
						m.logger.Warningf(
							"the VM creation client requested a conversion from from %s to %s; the mock library does not support this and the source image data will be used unmodified which may lead to errors",
							disk.format,
							format,
						)
						disk.format = *format
					}
				}
				break
			}
		}
		newDisk := disk.clone(sparse)
		_ = newDisk.Lock()
		newDisk.alias = fmt.Sprintf("disk-%s", generateRandomID(5, m.nonSecureRandom))
		m.disks[newDisk.ID()] = newDisk

		go func() {
			time.Sleep(time.Second)
			newDisk.Unlock()
		}()

		diskAttachment := &diskAttachment{
			client:        m,
			id:            m.GenerateUUID(),
			vmid:          vm.id,
			diskID:        newDisk.ID(),
			diskInterface: attachment.diskInterface,
			bootable:      attachment.bootable,
			active:        attachment.active,
		}
		m.vmDiskAttachmentsByVM[vm.id][diskAttachment.id] = diskAttachment
		m.vmDiskAttachmentsByDisk[disk.id] = diskAttachment
	}
}

func (m *mockClient) createVMCPU(params OptionalVMParameters, tpl *template) *vmCPU {
	var cpu *vmCPU
	cpuParams := params.CPU()
	switch {
	case cpuParams != nil:
		cpu = &vmCPU{
			topo: &vmCPUTopo{
				cores:   cpuParams.Cores(),
				sockets: cpuParams.Sockets(),
				threads: cpuParams.Threads(),
			},
		}
	case tpl.cpu != nil:
		cpu = tpl.cpu.clone()
	default:
		cpu = &vmCPU{
			topo: &vmCPUTopo{
				cores:   1,
				sockets: 1,
				threads: 1,
			},
		}
	}
	return cpu
}
