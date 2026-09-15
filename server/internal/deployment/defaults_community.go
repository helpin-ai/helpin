//go:build !ee

// Package deployment owns edition defaults independent of product services.
package deployment

// WidgetIdentityMode applies only when creating an installation. Saved policies
// are never rewritten when editions or release versions change.
const WidgetIdentityMode = "report_only"
