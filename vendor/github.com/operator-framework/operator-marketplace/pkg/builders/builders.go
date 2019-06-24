package builders

// OwnerNameLabel is the label used to mark ownership over a given resources.
// When this label is set, the reconciler should handle these resources when the owner
// is deleted.
const OwnerNameLabel string = "csc-owner-name"

// OwnerNamespaceLabel is the label used to mark ownership over a given resources.
// When this label is set, the reconciler should handle these resources when the owner
// is deleted.
const OwnerNamespaceLabel string = "csc-owner-namespace"

// OpsrcOwnerNameLabel is the label used to mark ownership over resources
// that are owned by the OperatorSource. When this label is set, the reconciler
// should handle these resources when the OperatorSource is deleted.
const OpsrcOwnerNameLabel string = "opsrc-owner-name"

// OpsrcOwnerNamespaceLabel is the label used to mark ownership over resources
// that are owned by the OperatorSource. When this label is set, the reconciler
// should handle these resources when the OperatorSource is deleted.
const OpsrcOwnerNamespaceLabel string = "opsrc-owner-namespace"
