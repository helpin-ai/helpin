# Roadmap

- [ ] Failover scenario when Kafka is down. Use Redis to store the events.
  - [ ] Add the kafka health check that checks if Kafka is up or down.
  - [ ] A consumer for the Redis which picks the event from the Redis and send to the Kafka Transformation topic.
- [ ]  Code coverage (right now we have not written any test cases. This should be done on priority.)