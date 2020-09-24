package openapi

import extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

// OperatorConfigStatus provides JSONSchemaProps for the Status struct
func OperatorConfigStatus(statusName string, operatorName string) extv1.JSONSchemaProps {
	return extv1.JSONSchemaProps{
		Type:        "object",
		Description: statusName + " defines the status of the " + operatorName + " installation",
		Properties: map[string]extv1.JSONSchemaProps{
			"targetVersion": {
				Description: "The desired version of the " + operatorName + " resource",
				Type:        "string",
			},
			"observedVersion": {
				Description: "The observed version of the " + operatorName + " resource",
				Type:        "string",
			},
			"operatorVersion": {
				Description: "The version of the " + operatorName + " resource as defined by the operator",
				Type:        "string",
			},
			"phase": {
				Description: "Phase is the current phase of the " + operatorName + " deployment",
				Type:        "string",
			},
			"conditions": {
				Description: "A list of current conditions of the " + operatorName + "resource",
				Type:        "array",
				Items: &extv1.JSONSchemaPropsOrArray{
					Schema: &extv1.JSONSchemaProps{
						Type:        "object",
						Description: "Condition represents the state of the operator's reconciliation functionality.",
						Properties: map[string]extv1.JSONSchemaProps{
							"lastHeartbeatTime": {
								Type:   "string",
								Format: "date-time",
							},
							"lastTransitionTime": {
								Type:   "string",
								Format: "date-time",
							},
							"message": {
								Type: "string",
							},
							"reason": {
								Type: "string",
							},
							"status": {
								Type: "string",
							},
							"type": {
								Description: "ConditionType is the state of the operator's reconciliation functionality.",
								Type:        "string",
							},
						},
						Required: []string{
							"status",
							"type",
						},
					},
				},
			},
		},
	}
}
