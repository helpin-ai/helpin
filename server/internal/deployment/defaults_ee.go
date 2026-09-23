//go:build ee

package deployment

// EditionName identifies the edition in operator-facing status responses.
const EditionName = "enterprise"

// WidgetIdentityMode retains the SaaS default for new installations.
const WidgetIdentityMode = "enforced"

// DefaultModules is the edition’s default product surface.
const DefaultModules = "pm,docs,crm,support,automation,agents"

// SetupGuideDefault applies when SETUP_SUCCESS_ENABLED is unset. Enterprise keeps
// the Setup guide off unless SETUP_SUCCESS_ENABLED opts in.
const SetupGuideDefault = false

// AllowUnverifiedSignup permits an explicit local/operator auth policy.
const AllowUnverifiedSignup = false

const DefaultWidgetOrigin = "https://client.helpin.ai"
const DefaultSDKLoaderURL = "https://cdn.helpin.ai/lib.js"

const DefaultReplyDomain = "replies.helpin.email"
const DefaultRouteDomain = "on.helpin.email"
