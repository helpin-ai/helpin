//go:build !ee

// Package deployment owns edition defaults independent of product services.
package deployment

// WidgetIdentityMode applies only when creating an installation. Saved policies
// are never rewritten when editions or release versions change.
const WidgetIdentityMode = "report_only"

// DefaultModules is the edition’s default product surface.
const DefaultModules = "support,docs,agents"

// AllowUnverifiedSignup permits an explicit local/operator auth policy.
const AllowUnverifiedSignup = true

const DefaultWidgetOrigin = ""
const DefaultSDKLoaderURL = ""

const DefaultReplyDomain = ""
const DefaultRouteDomain = ""
