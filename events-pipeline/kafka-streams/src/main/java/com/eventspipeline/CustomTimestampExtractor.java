package com.eventspipeline;

import java.time.Instant;

import org.apache.kafka.clients.consumer.ConsumerRecord;
import org.apache.kafka.streams.processor.TimestampExtractor;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;

public class CustomTimestampExtractor implements TimestampExtractor {
    private static final ObjectMapper mapper = new ObjectMapper();

    @Override
    public long extract(ConsumerRecord<Object, Object> record, long previousTimestamp) {
        // Assuming the event timestamp is stored in a field called "_timestamp"
        Object event = record.value();
        long timestamp = 0;

        try {
            // Parse the event and extract the timestamp field
            JsonNode eventJson = mapper.readTree(event.toString());
            String timestampString = eventJson.get("timestamp").asText();
            Instant instant = Instant.parse(timestampString);
            timestamp = instant.toEpochMilli();
        } catch (Exception e) {
            e.printStackTrace();
        }

        return timestamp;
    }
}
