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

package webhooks

import (
	"fmt"
	core "k8s.io/api/core/v1"
)

func validateAffinity(affinity *core.Affinity) error {
	if affinity == nil || affinity.NodeAffinity == nil {
		return nil
	}
	required := affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution
	if required != nil {
		if err := validateNodeSelectorTerms(required.NodeSelectorTerms); err != nil {
			return err
		}
	}
	for _, term := range affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution {
		if term.Weight <= 0 || term.Weight > 100 {
			return fmt.Errorf("weight must be in the range 1-100")
		}
		if err := validateNodeSelectorTerm(term.Preference); err != nil {
			return err
		}
	}
	return nil
}

func validateNodeSelectorTerms(terms []core.NodeSelectorTerm) error {
	if len(terms) == 0 {
		return fmt.Errorf("must have at least one node selector term")
	}
	for _, term := range terms {
		if err := validateNodeSelectorTerm(term); err != nil {
			return err
		}
	}
	return nil
}

func validateNodeSelectorTerm(term core.NodeSelectorTerm) error {
	for _, req := range term.MatchExpressions {
		if err := validateNodeSelectorRequirement(req); err != nil {
			return err
		}
	}
	for _, req := range term.MatchFields {
		if err := validateNodeFieldSelectorRequirement(req); err != nil {
			return err
		}
	}
	return nil
}

func validateNodeSelectorRequirement(req core.NodeSelectorRequirement) error {
	switch req.Operator {
	case core.NodeSelectorOpIn, core.NodeSelectorOpNotIn:
		if len(req.Values) == 0 {
			return fmt.Errorf("Values must be specified when `operator` is 'In' or 'NotIn'")
		}
	case core.NodeSelectorOpExists, core.NodeSelectorOpDoesNotExist:
		if len(req.Values) > 0 {
			return fmt.Errorf("Values may not be specified when `operator` is 'Exists' or 'DoesNotExist'")
		}
	case core.NodeSelectorOpGt, core.NodeSelectorOpLt:
		if len(req.Values) != 1 {
			return fmt.Errorf("Must be only one value when `operator` is 'Lt' or 'Gt'")
		}
	default:
		return fmt.Errorf("Operator %s is not a valid selector operator", req.Operator)
	}
	return nil
}

func validateNodeFieldSelectorRequirement(req core.NodeSelectorRequirement) error {
	switch req.Operator {
	case core.NodeSelectorOpIn, core.NodeSelectorOpNotIn:
		if len(req.Values) != 1 {
			return fmt.Errorf("Must be only one value when `operator` is 'In' or 'NotIn'")
		}
	default:
		return fmt.Errorf("Operator %s is not a valid field selector operator", req.Operator)
	}
	return nil
}
