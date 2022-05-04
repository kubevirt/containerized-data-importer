package ovirtclient

// DefaultBlankTemplateID returns the ID for the factory-default blank template. This should not be used
// as the template may be deleted from the oVirt engine. Instead, use the API call to find the blank template.
const DefaultBlankTemplateID TemplateID = "00000000-0000-0000-0000-000000000000"
