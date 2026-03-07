package temporalapp

import (
	"crypto/tls"
	"net"
	"strings"

	tclient "go.temporal.io/sdk/client"

	"github.com/helpin-ai/helpin/server/internal/config"
)

// BuildClientOptions returns Temporal SDK client options for local or cloud deployments.
func BuildClientOptions(cfg *config.Config) tclient.Options {
	options := tclient.Options{
		HostPort:  cfg.TemporalAddress,
		Namespace: cfg.TemporalNamespace,
	}

	if strings.TrimSpace(cfg.TemporalAPIKey) != "" {
		options.Credentials = tclient.NewAPIKeyStaticCredentials(strings.TrimSpace(cfg.TemporalAPIKey))
	}

	if cfg.TemporalTLSEnabled {
		options.ConnectionOptions.TLS = &tls.Config{
			ServerName: temporalTLSServerName(cfg.TemporalAddress, cfg.TemporalTLSServerName),
			MinVersion: tls.VersionTLS12,
		}
	}

	return options
}

func temporalTLSServerName(address, override string) string {
	if trimmed := strings.TrimSpace(override); trimmed != "" {
		return trimmed
	}

	host, _, err := net.SplitHostPort(strings.TrimSpace(address))
	if err == nil {
		return host
	}
	return strings.TrimSpace(address)
}
