/** Utils contains the common functionality such as Kafka connection configuration etc. */
use rdkafka::{config::RDKafkaLogLevel, ClientConfig};

pub fn create_consumer_kafka_config(brokers: String) -> ClientConfig {
    let is_auth_enabled = std::env::var("KAFKA_AUTH").unwrap_or_else(|_| "false".into());
    let security_protocol =
        std::env::var("KAFKA_SECURITY_PROTOCOL").unwrap_or_else(|_| "SASL_SSL".into());
    let sasl = std::env::var("KAFKA_SASL").unwrap_or_else(|_| "SCRAM-SHA-256".into());
    let group_id =
        std::env::var("KAFKA_GROUP_ID").unwrap_or_else(|_| "helpin-rust-consumer".into());

    let mut config = ClientConfig::new();

    config.set("bootstrap.servers", &brokers);
    if is_auth_enabled == "true" {
        if security_protocol == "SSL" {
            config.set("security.protocol", "SSL");
            config.set("ssl.ca.location", "data/tls/consumer/ca.crt"); // The path to your CA certificate
            config.set("ssl.certificate.location", "data/tls/consumer/client.crt"); // The path to your client certificate
            config.set("ssl.key.location", "data/tls/consumer/client.key"); // The path to your client private key
        } else {
            config.set("security.protocol", security_protocol);
            config.set("sasl.mechanisms", sasl);
            config.set(
                "sasl.username",
                std::env::var("KAFKA_USERNAME").unwrap_or_else(|_| "".into()),
            );
            config.set(
                "sasl.password",
                std::env::var("KAFKA_PASSWORD").unwrap_or_else(|_| "".into()),
            );
        }
    }

    config.set("enable.auto.commit", "false");
    config.set("fetch.wait.max.ms", "500");
    config.set("fetch.min.bytes", "16384");
    // config.set("fetch.max.bytes", "52428800");
    // config.set("max.partition.fetch.bytes", "1048576");
    // config.set("fetch.wait.max.ms", "0");
    config.set("group.id", group_id);
    config.set("auto.offset.reset", "earliest");
    // config.set_log_level(RDKafkaLogLevel::Debug);

    return config;
}

pub fn create_producer_kafka_config(brokers: String, topic: String) -> ClientConfig {
    let is_auth_enabled = std::env::var("KAFKA_AUTH").unwrap_or_else(|_| "false".into());
    let security_protocol =
        std::env::var("KAFKA_SECURITY_PROTOCOL").unwrap_or_else(|_| "SASL_SSL".into());
    let sasl = std::env::var("KAFKA_SASL").unwrap_or_else(|_| "SCRAM-SHA-256".into());
    let mut config = ClientConfig::new();
    config.set("bootstrap.servers", &brokers);
    config.set("queue.buffering.max.ms", "100");
    config.set("linger.ms", "100");
    config.set("compression.type", "lz4");
    config.set("batch.size", "32000");
    config.set("statistics.interval.ms", "10000");
    config.set_log_level(RDKafkaLogLevel::Debug);

    // config.set("queue.buffering.max.messages", "2097151"); // queue length
    if is_auth_enabled == "true" {
        if security_protocol == "SSL" {
            config.set("security.protocol", "SSL");

            config.set("ssl.ca.location", "data/tls/producer/ca.crt"); // The path to your CA certificate
            config.set("ssl.certificate.location", "data/tls/producer/client.crt"); // The path to your client certificate
            config.set("ssl.key.location", "data/tls/producer/client.key"); // The path to your client private key
        } else {
            config.set("security.protocol", security_protocol);
            config.set("sasl.mechanisms", sasl);
            config.set(
                "sasl.username",
                std::env::var("KAFKA_USERNAME").unwrap_or_else(|_| "".into()),
            );
            config.set(
                "sasl.password",
                std::env::var("KAFKA_PASSWORD").unwrap_or_else(|_| "".into()),
            );
        }
    }
    return config;
}
