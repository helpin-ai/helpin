package chmigrate

import (
	"fmt"
	"os"
	"strings"
)

const (
	defaultKafkaBrokers = "kafka:29092"
	defaultKafkaTopic   = "helpin.events.sessionized"
	defaultKafkaGroup   = "helpin-clickhouse-local-v1"
)

func renderMigrationSQL(contents string) (string, error) {
	if err := validateKafkaEnvironment(); err != nil {
		return "", err
	}
	brokers := envOrDefault("KAFKA_BROKERS", defaultKafkaBrokers)
	topic := envOrDefault("KAFKA_SESSIONIZED_TOPIC", defaultKafkaTopic)
	group := envOrDefault("CLICKHOUSE_KAFKA_GROUP", defaultKafkaGroup)

	rendered := strings.ReplaceAll(contents, quoteSQL(defaultKafkaBrokers), quoteSQL(brokers))
	rendered = strings.ReplaceAll(rendered, quoteSQL(defaultKafkaTopic), quoteSQL(topic))
	rendered = strings.ReplaceAll(rendered, quoteSQL(defaultKafkaGroup), quoteSQL(group))

	authSettings, err := kafkaAuthSettings()
	if err != nil {
		return "", err
	}
	if authSettings != "" {
		rendered = strings.ReplaceAll(
			rendered,
			"kafka_num_consumers = 1",
			"kafka_num_consumers = 1,"+authSettings,
		)
	}
	return rendered, nil
}

func validateKafkaEnvironment() error {
	if !strings.EqualFold(
		strings.TrimSpace(os.Getenv("CLICKHOUSE_MIGRATION_REQUIRE_KAFKA_CONFIG")),
		"true",
	) {
		return nil
	}
	for _, name := range []string{"KAFKA_BROKERS", "KAFKA_SESSIONIZED_TOPIC", "KAFKA_AUTH"} {
		if value, ok := os.LookupEnv(name); !ok || strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required for ClickHouse migrations", name)
		}
	}
	return nil
}

func kafkaAuthSettings() (string, error) {
	if !strings.EqualFold(strings.TrimSpace(os.Getenv("KAFKA_AUTH")), "true") {
		return "", nil
	}
	protocol := strings.ToLower(strings.TrimSpace(os.Getenv("KAFKA_SECURITY_PROTOCOL")))
	mechanism := strings.TrimSpace(os.Getenv("KAFKA_SASL"))
	username := strings.TrimSpace(os.Getenv("KAFKA_USERNAME"))
	password := os.Getenv("KAFKA_PASSWORD")
	if protocol == "" || mechanism == "" || username == "" || password == "" {
		return "", fmt.Errorf(
			"KAFKA_SECURITY_PROTOCOL, KAFKA_SASL, KAFKA_USERNAME, and KAFKA_PASSWORD are required when KAFKA_AUTH=true",
		)
	}
	return fmt.Sprintf(`
    kafka_security_protocol = %s,
    kafka_sasl_mechanism = %s,
    kafka_sasl_username = %s,
    kafka_sasl_password = %s`,
		quoteSQL(protocol),
		quoteSQL(mechanism),
		quoteSQL(username),
		quoteSQL(password),
	), nil
}

func envOrDefault(name string, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func quoteSQL(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `'`, `\'`)
	return "'" + value + "'"
}
