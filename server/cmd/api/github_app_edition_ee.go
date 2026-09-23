//go:build ee

package main

// gitHubAppManifestEnabled is false in Enterprise: the operator provisions the
// GitHub App through GITHUB_APP_* configuration.
const gitHubAppManifestEnabled = false
