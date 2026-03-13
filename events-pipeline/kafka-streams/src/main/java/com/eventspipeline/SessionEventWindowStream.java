package com.eventspipeline;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ObjectNode;
import org.apache.commons.codec.digest.DigestUtils;
import org.apache.kafka.clients.consumer.ConsumerConfig;
import org.apache.kafka.common.serialization.Serdes;
import org.apache.kafka.streams.KafkaStreams;
import org.apache.kafka.streams.KeyValue;
import org.apache.kafka.streams.StreamsBuilder;
import org.apache.kafka.streams.StreamsConfig;
import org.apache.kafka.streams.kstream.*;
import org.apache.kafka.streams.Topology;
import java.time.Duration;
import java.time.Instant;
import java.time.ZoneOffset;
import java.time.format.DateTimeFormatter;
import java.util.Properties;
import io.github.cdimascio.dotenv.Dotenv;
import io.github.cdimascio.dotenv.DotenvException;

public class SessionEventWindowStream {
    static String TOPIC_INPUT;
    static final Duration INACTIVITY_GAP = Duration.ofMinutes(30);

    static String TOPIC_OUTPUT;
    static String AUTH_REQUIRED;
    static String SECURITY_PROTOCOL;

    private static final ObjectMapper mapper = new ObjectMapper();

    public static void main(final String[] args) {
        Dotenv dotenv = Dotenv.configure().ignoreIfMissing().load();
        TOPIC_INPUT = dotenv.get("KAFKA_TRANSFORMATION_TOPIC");
        TOPIC_OUTPUT = dotenv.get("KAFKA_SESSIONIZED_TOPIC");
        AUTH_REQUIRED = dotenv.get("KAFKA_AUTH");
        SECURITY_PROTOCOL = dotenv.get("KAFKA_SECURITY_PROTOCOL");

        final String bootstrapServers = dotenv.get("KAFKA_BROKERS", "localhost:9092");
        final KafkaStreams streams = new KafkaStreams(
                buildTopology(),
                streamsConfig(bootstrapServers, "/tmp/kafka-streams", AUTH_REQUIRED, SECURITY_PROTOCOL));

        // Graceful shutdown hook
        Runtime.getRuntime().addShutdownHook(new Thread(streams::close));

        streams.cleanUp();
        streams.start();
    }

    static Properties streamsConfig(final String bootstrapServers, final String stateDir, String AUTH_REQUIRED,
            String SECURITY_PROTOCOL) {
        final Properties config = new Properties();
        config.put(StreamsConfig.APPLICATION_ID_CONFIG, "helpin-eventpipeline-kstreams");
        config.put(StreamsConfig.CLIENT_ID_CONFIG, "helpin-eventpipeline-sessionized-client");
        config.put(StreamsConfig.BOOTSTRAP_SERVERS_CONFIG, bootstrapServers);
        config.put(StreamsConfig.STATE_DIR_CONFIG, stateDir);
        config.put(ConsumerConfig.AUTO_OFFSET_RESET_CONFIG, "earliest");
        config.put(StreamsConfig.CACHE_MAX_BYTES_BUFFERING_CONFIG, 0);
        config.put(StreamsConfig.NUM_STREAM_THREADS_CONFIG, 4);
        // Set the custom timestamp extractor
        config.put(StreamsConfig.DEFAULT_TIMESTAMP_EXTRACTOR_CLASS_CONFIG, CustomTimestampExtractor.class.getName());
        config.put(StreamsConfig.DEFAULT_KEY_SERDE_CLASS_CONFIG,
                Serdes.String().getClass().getName());
        config.put(StreamsConfig.DEFAULT_VALUE_SERDE_CLASS_CONFIG,
                Serdes.String().getClass().getName());
        config.put(ConsumerConfig.GROUP_ID_CONFIG, "helpin-eventpipeline-kstreams");
        config.put("compression.type", "lz4");
        System.out.println("Auth required: " + AUTH_REQUIRED);
        System.out.println("Security protocol: " + SECURITY_PROTOCOL);
        if (AUTH_REQUIRED != null && AUTH_REQUIRED.equals("true") &&
                SECURITY_PROTOCOL != null) {

            config.put("security.protocol", SECURITY_PROTOCOL);

            if (SECURITY_PROTOCOL.equals("SASL_PLAINTEXT") || SECURITY_PROTOCOL.equals("SASL_SSL")) {
                // SASL/SCRAM authentication
                String saslMechanism = getEnvironmentVariable("KAFKA_SASL_MECHANISM");
                if (saslMechanism == null || saslMechanism.isEmpty()) {
                    saslMechanism = "SCRAM-SHA-512";
                }
                config.put("sasl.mechanism", saslMechanism);

                String saslUsername = getEnvironmentVariable("KAFKA_SASL_USERNAME");
                String saslPassword = getEnvironmentVariable("KAFKA_SASL_PASSWORD");
                if (saslUsername == null || saslPassword == null) {
                    throw new RuntimeException("KAFKA_SASL_USERNAME and KAFKA_SASL_PASSWORD are required for SASL auth.");
                }
                String jaasConfig = "org.apache.kafka.common.security.scram.ScramLoginModule required "
                        + "username=\"" + saslUsername + "\" "
                        + "password=\"" + saslPassword + "\";";
                config.put("sasl.jaas.config", jaasConfig);
                System.out.println("Configured SASL/" + saslMechanism + " auth for user: " + saslUsername);
            }

            if (SECURITY_PROTOCOL.equals("SSL") || SECURITY_PROTOCOL.equals("SASL_SSL")) {
                // TLS/SSL certificate configuration
                String truststorePassword = getEnvironmentVariable("TRUSTSTORE_PASSWORD");
                if (truststorePassword == null || truststorePassword.isEmpty()) {
                    throw new RuntimeException("No TRUSTSTORE_PASSWORD environment variable found.");
                }
                config.put("ssl.truststore.location", "src/main/java/com/eventspipeline/resources/tls/cluster.p12");
                config.put("ssl.truststore.password", truststorePassword);
                config.put("ssl.truststore.type", "PKCS12");

                String keystorePassword = getEnvironmentVariable("KEYSTORE_PASSWORD");
                if (keystorePassword == null || keystorePassword.isEmpty()) {
                    throw new RuntimeException("No KEYSTORE_PASSWORD environment variable found.");
                }
                config.put("ssl.keystore.location", "src/main/java/com/eventspipeline/resources/tls/user.p12");
                config.put("ssl.keystore.password", keystorePassword);
                config.put("ssl.keystore.type", "PKCS12");
                config.put("ssl.endpoint.identification.algorithm", "");
                System.out.println("Configured SSL/TLS auth");
            }
        }

        return config;
    }

    /**
     * Returns the value of the specified environment variable, as a String.
     * The method first checks the system environment variables, and if the value is
     * not found,
     * it checks the .env file.
     *
     * @param varName the name of the environment variable
     * @return the string value of the variable
     */
    public static String getEnvironmentVariable(String varName) {
        String varValue = System.getenv(varName);
        if (varValue == null || varValue.isEmpty()) {
            Dotenv dotenv = Dotenv.configure().ignoreIfMissing().load();
            varValue = dotenv.get(varName);
        }
        return varValue;
    }

    static Topology buildTopology() {
        final StreamsBuilder builder = new StreamsBuilder();
        // Reading the stream of events from the TOPIC_INPUT
        KStream<String, String> input = builder.stream(TOPIC_INPUT, Consumed.with(Serdes.String(), Serdes.String()));

        // Transforming the event's key using the processKey() method
        KTable<Windowed<String>, String> aggregated = input
                .filter((key, value) -> value != null) // Filters out records with null value
                .map((key, value) -> {
                    return new KeyValue<>(processKey(value), value);
                }) // .map((key, value) -> new KeyValue<>(processKey(value), value))
                   // Grouping events by the combination of project_id and user_anonymous_id
                .groupByKey(Grouped.with(Serdes.String(), Serdes.String()))
                // Creating session windows with the defined inactivity gap of 30 minutes
                .windowedBy(SessionWindows.with(INACTIVITY_GAP))
                // Performing aggregation on the grouped stream
                .aggregate(
                        // Initializer
                        () -> "",
                        // Aggregator
                        (key, value, aggValue) -> {
                            try {

                                ObjectNode event = (ObjectNode) mapper.readTree(value);
                                if (aggValue.isEmpty()) {
                                    // System.out.println("AggValue is empty");
                                    // If the aggregation value is empty, generating a new session id
                                    String processedKey = processKey(value);
                                    // Extracting "timestamp"
                                    String timestamp = event.get("timestamp").asText();

                                    // Convert to epoch milliseconds and floor to the closest 30-minute mark
                                    long timestampMillis = Instant.parse(timestamp).toEpochMilli();
                                    // long startOfWindow = timestampMillis / (30 * 60 * 1000) * (30 * 60 * 1000);
                                    String sessionId = DigestUtils.sha256Hex(timestampMillis + ":" + processedKey);
                                    event.put("session_id", sessionId);
                                    return mapper.writeValueAsString(event);
                                } else {
                                    // System.out.println("AggValue is not empty");
                                    // Otherwise, updating the session id with the existing one
                                    event.put("session_id", mapper.readTree(aggValue).get("session_id").asText());
                                    return mapper.writeValueAsString(event);
                                }
                            } catch (Exception e) {
                                throw new RuntimeException(e);
                            }
                        },
                        // Session Merger
                        (aggKey, aggOne, aggTwo) -> {
                            if (aggOne != null && !aggOne.isEmpty()) {
                                return aggOne;
                            } else {
                                return aggTwo;
                            }
                        });

        // Mapping to a regular (non-windowed) stream, and replacing the windowed key
        // with just the key
        aggregated.toStream()
                .filter((key, value) -> value != null)
                .map((key, value) -> new KeyValue<>(key.key(), value))
                // Writing the stream of session ids to TOPIC_OUTPUT
                .to(TOPIC_OUTPUT, Produced.with(Serdes.String(), Serdes.String()));
        return builder.build();
    }

    private static String processKey(String eventJson) {
        try {
            // Processing the event data from a JSON string
            JsonNode event = mapper.readTree(eventJson);
            // Extracting "project_id" and "user_anonymous_id"
            String projectId = event.get("project_id").asText();
            String userId = event.get("user_anonymous_id").asText();

            // Returning the key for Kafka Streams grouping
            return projectId + ":" + userId;

        } catch (Exception e) {
            throw new RuntimeException(e);
        }
    }

}
