package ovirtclient

import (
	"fmt"
	"strconv"

	ovirtsdk "github.com/ovirt/go-ovirt"
)

type vmBuilderComponent func(params OptionalVMParameters, builder *ovirtsdk.VmBuilder)

func vmBuilderComment(params OptionalVMParameters, builder *ovirtsdk.VmBuilder) {
	if comment := params.Comment(); comment != "" {
		builder.Comment(comment)
	}
}

func vmBuilderCPU(params OptionalVMParameters, builder *ovirtsdk.VmBuilder) {
	if cpu := params.CPU(); cpu != nil {
		builder.CpuBuilder(
			ovirtsdk.NewCpuBuilder().TopologyBuilder(
				ovirtsdk.
					NewCpuTopologyBuilder().
					Cores(int64(cpu.Cores())).
					Threads(int64(cpu.Threads())).
					Sockets(int64(cpu.Sockets())),
			))
	}
}

func vmBuilderHugePages(params OptionalVMParameters, builder *ovirtsdk.VmBuilder) {
	var customProperties []*ovirtsdk.CustomProperty
	if hugePages := params.HugePages(); hugePages != nil {
		customProp, err := ovirtsdk.NewCustomPropertyBuilder().
			Name("hugepages").
			Value(strconv.FormatUint(uint64(*hugePages), 10)).
			Build()
		if err != nil {
			panic(newError(EBug, "Failed to build 'hugepages' custom property from value %d", hugePages))
		}
		customProperties = append(customProperties, customProp)
	}
	if len(customProperties) > 0 {
		builder.CustomPropertiesOfAny(customProperties...)
	}
}

func vmBuilderMemory(params OptionalVMParameters, builder *ovirtsdk.VmBuilder) {
	if memory := params.Memory(); memory != nil {
		builder.Memory(*memory)
	}
}

func vmBuilderInitialization(params OptionalVMParameters, builder *ovirtsdk.VmBuilder) {
	if init := params.Initialization(); init != nil {
		initBuilder := ovirtsdk.NewInitializationBuilder()

		if init.CustomScript() != "" {
			initBuilder.CustomScript(init.CustomScript())
		}
		if init.HostName() != "" {
			initBuilder.HostName(init.HostName())
		}
		builder.InitializationBuilder(initBuilder)
	}
}

func vmPlacementPolicyParameterConverter(params OptionalVMParameters, builder *ovirtsdk.VmBuilder) {
	if pp := params.PlacementPolicy(); pp != nil {
		placementPolicyBuilder := ovirtsdk.NewVmPlacementPolicyBuilder()
		if affinity := (*pp).Affinity(); affinity != nil {
			placementPolicyBuilder.Affinity(ovirtsdk.VmAffinity(*affinity))
		}
		hosts := make([]ovirtsdk.HostBuilder, len((*pp).HostIDs()))
		for i, hostID := range (*pp).HostIDs() {
			hostBuilder := ovirtsdk.NewHostBuilder().Id(hostID)
			hosts[i] = *hostBuilder
		}
		placementPolicyBuilder.HostsBuilderOfAny(hosts...)
		builder.PlacementPolicyBuilder(placementPolicyBuilder)
	}
}

func (o *oVirtClient) CreateVM(clusterID ClusterID, templateID TemplateID, name string, params OptionalVMParameters, retries ...RetryStrategy) (result VM, err error) {
	retries = defaultRetries(retries, defaultLongTimeouts())

	if err := validateVMCreationParameters(clusterID, templateID, name, params); err != nil {
		return nil, err
	}

	if params == nil {
		params = &vmParams{}
	}

	message := fmt.Sprintf("creating VM %s", name)
	vm, err := createSDKVM(clusterID, templateID, name, params)
	if err != nil {
		return nil, err
	}

	err = retry(
		message,
		o.logger,
		retries,
		func() error {
			vmCreateRequest := o.conn.SystemService().VmsService().Add().Vm(vm)
			if clone := params.Clone(); clone != nil {
				vmCreateRequest.Clone(*clone)
			}
			response, err := vmCreateRequest.Send()
			if err != nil {
				return err
			}
			vm, ok := response.Vm()
			if !ok {
				return newError(EFieldMissing, "missing VM in VM create response")
			}
			result, err = convertSDKVM(vm, o)
			if err != nil {
				return wrap(
					err,
					EBug,
					"failed to convert VM",
				)
			}
			return nil
		},
	)
	return result, err
}

func createSDKVM(
	clusterID ClusterID,
	templateID TemplateID,
	name string,
	params OptionalVMParameters,
) (*ovirtsdk.Vm, error) {
	builder := ovirtsdk.NewVmBuilder()
	builder.Cluster(ovirtsdk.NewClusterBuilder().Id(string(clusterID)).MustBuild())
	builder.Template(ovirtsdk.NewTemplateBuilder().Id(string(templateID)).MustBuild())
	builder.Name(name)
	parts := []vmBuilderComponent{
		vmBuilderComment,
		vmBuilderCPU,
		vmBuilderHugePages,
		vmBuilderInitialization,
		vmBuilderMemory,
		vmPlacementPolicyParameterConverter,
		vmBuilderMemoryPolicy,
	}

	for _, part := range parts {
		part(params, builder)
	}

	if params != nil {
		var diskAttachments []*ovirtsdk.DiskAttachment
		for i, d := range params.Disks() {
			diskAttachment := ovirtsdk.NewDiskAttachmentBuilder()
			diskBuilder := ovirtsdk.NewDiskBuilder()
			diskBuilder.Id(d.DiskID())
			if sparse := d.Sparse(); sparse != nil {
				diskBuilder.Sparse(*sparse)
			}
			if format := d.Format(); format != nil {
				diskBuilder.Format(ovirtsdk.DiskFormat(*format))
			}
			diskAttachment.DiskBuilder(diskBuilder)
			sdkDisk, err := diskAttachment.Build()
			if err != nil {
				return nil, wrap(err, EBadArgument, "Failed to convert disk %d.", i)
			}
			diskAttachments = append(diskAttachments, sdkDisk)
		}
		builder.DiskAttachmentsOfAny(diskAttachments...)
	}

	vm, err := builder.Build()
	if err != nil {
		return nil, wrap(err, EBug, "failed to build VM")
	}
	return vm, nil
}

func vmBuilderMemoryPolicy(params OptionalVMParameters, builder *ovirtsdk.VmBuilder) {
	if memPolicyParams := params.MemoryPolicy(); memPolicyParams != nil {
		memoryPolicyBuilder := ovirtsdk.NewMemoryPolicyBuilder()
		if guaranteed := (*memPolicyParams).Guaranteed(); guaranteed != nil {
			memoryPolicyBuilder.Guaranteed(*guaranteed)
		}
		builder.MemoryPolicyBuilder(memoryPolicyBuilder)
	}
}

func validateVMCreationParameters(clusterID ClusterID, templateID TemplateID, name string, params OptionalVMParameters) error {
	if name == "" {
		return newError(EBadArgument, "name cannot be empty for VM creation")
	}
	if clusterID == "" {
		return newError(EBadArgument, "cluster ID cannot be empty for VM creation")
	}
	if templateID == "" {
		return newError(EBadArgument, "template ID cannot be empty for VM creation")
	}
	if params == nil {
		return nil
	}

	memory := params.Memory()
	if memory == nil {
		mem := int64(1024 * 1024 * 1024)
		memory = &mem
	}
	guaranteedMemory := memory
	if memPolicy := params.MemoryPolicy(); memPolicy != nil {
		guaranteed := (*memPolicy).Guaranteed()
		if guaranteed != nil {
			guaranteedMemory = guaranteed
		}
	}
	if *guaranteedMemory > *memory {
		return newError(
			EBadArgument,
			"guaranteed memory is larger than the VM memory (%d > %d)",
			*guaranteedMemory,
			*memory,
		)
	}

	disks := params.Disks()
	diskIDs := map[string]int{}
	for i, d := range disks {
		if previousID, ok := diskIDs[d.DiskID()]; ok {
			return newError(
				EBadArgument,
				"Disk %s appears twice, in position %d and %d.",
				d.DiskID(),
				previousID,
				i,
			)
		}
	}

	return nil
}
