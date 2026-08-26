use async_trait::async_trait;
use futures::future::try_join_all;

use crate::api::CaptureError;
use crate::enrichment::bot_resolver::BotResolver;
use crate::enrichment::database_state::EnrichmentDatabaseState;
use crate::enrichment::handler::{EnrichmentHandler, MyError};
use crate::enrichment::ua_resolver::UaResolver;
use crate::pipeline::{EnrichedEventEnvelopeV1, RawArchiveEnvelopeV1};

use super::nats_event_sink::NatsEventPublisher;
use super::{EventSink, EventTypes};

#[derive(Clone)]
pub struct EnrichingNatsSink {
    publisher: NatsEventPublisher,
    handler: EnrichmentHandler,
    databases: EnrichmentDatabaseState,
    bot_resolver: BotResolver,
    ua_resolver: UaResolver,
}

impl EnrichingNatsSink {
    pub fn new(publisher: NatsEventPublisher, databases: EnrichmentDatabaseState) -> Self {
        Self {
            publisher,
            handler: EnrichmentHandler::new(),
            databases,
            bot_resolver: BotResolver::new(),
            ua_resolver: UaResolver::new(),
        }
    }

    async fn process(&self, event: EventTypes) -> Result<(), CaptureError> {
        let EventTypes::Processed(processed) = event else {
            return Err(CaptureError::NonRetryableSinkError(
                "inline enrichment accepts only processed capture events".to_string(),
            ));
        };
        let raw_event = self.publisher.archive_enabled().then(|| processed.clone());
        let (geo_resolver, ip2proxy_resolver) = self.databases.snapshot().await;
        let transformed = match self
            .handler
            .process_event(
                processed,
                geo_resolver.as_ref(),
                ip2proxy_resolver.as_ref(),
                &self.bot_resolver,
                &self.ua_resolver,
            )
            .await
        {
            Ok(event) => event,
            Err(error) => match error.downcast::<MyError>() {
                Ok(rejected) => {
                    metrics::counter!("capture_enrichment_rejections_total", 1);
                    if let Err(publish_error) =
                        self.publisher.publish_failed(&rejected.failed_event).await
                    {
                        metrics::counter!("capture_dlq_publish_failures_total", 1);
                        tracing::warn!(%publish_error, "capture DLQ publish failed");
                        // Preserve the original event through the durable fallback. Returning
                        // success here would lose both the event and its rejection record.
                        return Err(CaptureError::RetryableSinkError);
                    }
                    return Ok(());
                }
                Err(error) => {
                    return Err(CaptureError::NonRetryableSinkError(format!(
                        "inline enrichment failed: {error}"
                    )))
                }
            },
        };

        let enriched = EnrichedEventEnvelopeV1::new(transformed);
        match raw_event {
            Some(mut raw_event) => {
                raw_event.authorization = None;
                raw_event.event.ip = Some(enriched.event.source_ip.clone());
                let raw = RawArchiveEnvelopeV1 {
                    schema_version: 1,
                    event_id: raw_event.event_id.to_string(),
                    event_received_at: raw_event.event.received_at.clone(),
                    event: raw_event,
                };
                self.publisher.publish_event(&enriched, &raw).await
            }
            None => self.publisher.publish_enriched(&enriched).await,
        }
    }
}

#[async_trait]
impl EventSink for EnrichingNatsSink {
    async fn send(&self, event: EventTypes) -> Result<(), CaptureError> {
        self.process(event).await
    }

    async fn send_batch(&self, events: Vec<EventTypes>) -> Result<(), CaptureError> {
        try_join_all(events.into_iter().map(|event| self.process(event)))
            .await
            .map(|_| ())
    }
}
