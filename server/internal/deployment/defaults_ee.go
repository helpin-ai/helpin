//go:build ee

package deployment

// WidgetIdentityMode retains the SaaS default for new installations.
const WidgetIdentityMode = "enforced"

// DefaultModules is the edition’s default product surface.
const DefaultModules = "pm,docs,crm,support,automation,agents"

// AllowUnverifiedSignup permits an explicit local/operator auth policy.
const AllowUnverifiedSignup = false
