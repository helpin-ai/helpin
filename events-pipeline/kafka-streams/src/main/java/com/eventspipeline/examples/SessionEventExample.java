package com.eventspipeline;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ObjectNode;
import org.apache.commons.codec.digest.DigestUtils;
import org.apache.kafka.clients.consumer.ConsumerConfig;
import org.apache.kafka.common.serialization.Serdes;
import org.apache.kafka.streams.KafkaStreams;
import org.apache.kafka.streams.StreamsBuilder;
import org.apache.kafka.streams.StreamsConfig;
import org.apache.kafka.streams.kstream.*;
import org.apache.kafka.streams.Topology;
import java.time.Duration;
import java.time.Instant;
import java.time.ZoneOffset;
import java.time.format.DateTimeFormatter;
import java.util.Properties;

public class SessionEventExample {
    static final String TOPIC_INPUT = "rust-async-event-transformed-test";
    static final Duration INACTIVITY_GAP = Duration.ofMinutes(30);
    static final String TOPIC_OUTPUT = "user-sessions";

    private static final ObjectMapper mapper = new ObjectMapper();

    public static void main(final String[] args) {
        final String bootstrapServers = args.length > 0 ? args[0] : "localhost:9092";
        final KafkaStreams streams = new KafkaStreams(
                buildTopology(),
                streamsConfig(bootstrapServers, "/tmp/kafka-streams")
        );

        streams.cleanUp();
        streams.start();
    }

    static Properties streamsConfig(final String bootstrapServers, final String stateDir) {
        final Properties config = new Properties();
        config.put(StreamsConfig.APPLICATION_ID_CONFIG, "session-windows-example");
        config.put(StreamsConfig.CLIENT_ID_CONFIG, "session-windows-example-client");
        config.put(StreamsConfig.BOOTSTRAP_SERVERS_CONFIG, bootstrapServers);
        config.put(StreamsConfig.STATE_DIR_CONFIG, stateDir);
        config.put(ConsumerConfig.AUTO_OFFSET_RESET_CONFIG, "earliest");
        config.put(StreamsConfig.CACHE_MAX_BYTES_BUFFERING_CONFIG, 0);
        return config;
    }

    static Topology buildTopology() {
        final StreamsBuilder builder = new StreamsBuilder();
        builder.stream(TOPIC_INPUT, Consumed.with(Serdes.String(), Serdes.String()))
                .mapValues(SessionEventExample::processEvent)
                .to(TOPIC_OUTPUT, Produced.with(Serdes.String(), Serdes.String()));
        return builder.build();
    }

    private static String processEvent(String eventJson) {
        try {
            JsonNode event = mapper.readTree(eventJson);
            String projectId = event.get("project_id").asText();
            String userId = event.get("user_anonymous_id").asText();
            String timestamp = event.get("_timestamp").asText();

            Instant instant = Instant.parse(timestamp);
            Instant startOfPeriod = instant.minusNanos(instant.getNano()).minusSeconds(instant.getEpochSecond() % 1800);
            String startOfPeriodStr = DateTimeFormatter.ofPattern("yyyy-MM-dd'T'HH:mm:ss'Z'").withZone(ZoneOffset.UTC).format(startOfPeriod);

            String sessionId = DigestUtils.sha256Hex(startOfPeriodStr + userId + projectId);
            ((ObjectNode) event).put("session_id", sessionId);
            return event.toString();

        } catch (Exception e) {
            throw new RuntimeException(e);
        }
    }
}
