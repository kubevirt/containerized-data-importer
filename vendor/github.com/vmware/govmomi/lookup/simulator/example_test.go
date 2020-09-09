/*
Copyright (c) 2020 VMware, Inc. All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package simulator_test

import (
	"context"
	"fmt"
	"log"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/lookup"
	lsim "github.com/vmware/govmomi/lookup/simulator"
	"github.com/vmware/govmomi/lookup/types"
	"github.com/vmware/govmomi/simulator"
)

func ExampleServiceRegistration() {
	model := simulator.VPX()

	// TODO: using simulator.Run() would be simpler,
	// but access to lookup namespace Registry is not exported in that case.
	// Using lookup/simulator.New() directly in this example gives us access.
	defer model.Remove()
	err := model.Create()
	if err != nil {
		log.Fatal(err)
	}

	s := model.Service.NewServer()
	defer s.Close()

	sdk := lsim.New()

	model.Service.RegisterSDK(sdk)

	ctx := context.Background()

	vc, err := govmomi.NewClient(ctx, s.URL, true)
	if err != nil {
		log.Fatal(err)
	}

	// Note that ServiceRegistration.Info is generated the first time RetrieveServiceContent()
	// is called, so we do that here before modifying the Info list.
	c, err := lookup.NewClient(ctx, vc.Client)
	if err != nil {
		log.Fatal(err)
	}

	// Get a pointer to the in-memory lookup.ServiceRegistration object, which we can modify directly.
	r := sdk.Get(*c.ServiceContent.ServiceRegistration).(*lsim.ServiceRegistration)

	// Change the NodeId
	for i := range r.Info {
		if r.Info[i].ServiceType.Type == "vcenterserver" {
			r.Info[i].NodeId = "example-id"
			break
		}
	}

	filter := &types.LookupServiceRegistrationFilter{
		ServiceType: &types.LookupServiceRegistrationServiceType{
			Product: "com.vmware.cis",
			Type:    "vcenterserver",
		},
	}

	info, err := c.List(ctx, filter)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(info[0].NodeId)

	// Output:
	// example-id
}
