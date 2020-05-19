/*
 * This file is part of the CDI project
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * Copyright 2020 Red Hat, Inc.
 *
 */

package naming

import (
	"strings"

	"github.com/openshift/library-go/pkg/build/naming"
	kvalidation "k8s.io/apimachinery/pkg/util/validation"
)

// GetResourceName creates a name with provided suffix, and shortens if needed
// with the length restriction for pods/resources
func GetResourceName(base, suffix string) string {
	return naming.GetName(base, suffix, kvalidation.DNS1123SubdomainMaxLength)
}

// GetLabelNameFromResourceName creates a name with the length restriction for labels, and shortens if needed
func GetLabelNameFromResourceName(resourceName string) string {
	// resourceName can have dots, service name cannot
	base := strings.ReplaceAll(resourceName, ".", "-")

	if len(base) <= kvalidation.DNS1035LabelMaxLength {
		return base
	}

	// TODO: GetName does not work correctly with empty suffix (leaves trailing '-'), check if we want to:
	// - put our own small suffix - it has the advantage, that if some name is shortened we can see it was our shortener
	// - put very long suffix so it will dropped by GetName
	// - extend/fix GetName
	// - write our own GetName
	return naming.GetName(base, "cdi", kvalidation.DNS1035LabelMaxLength)
}

// GetServiceNameFromResourceName creates a name with the length restriction for service (label), and shortens if needed
func GetServiceNameFromResourceName(name string) string {
	// The name of a Service object must be a valid DNS label name.
	return GetLabelNameFromResourceName(name)
}
