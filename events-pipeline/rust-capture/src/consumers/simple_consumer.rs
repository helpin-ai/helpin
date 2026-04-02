use dotenv::dotenv;
use events_pipeline::enrichment::bot_resolver::BotResolver;
use events_pipeline::enrichment::handler::{EnrichmentHandler, MyError};
use events_pipeline::enrichment::ua_resolver::UaResolver;
use events_pipeline::events::transform_event::TransformedEvent;
use events_pipeline::geo::downloader::download_and_save;
use events_pipeline::geo::resolver::GeoResolver;
use events_pipeline::health::HealthRegistry;
use events_pipeline::ip2location::downloader::ip2proxy_download_and_save;
use events_pipeline::ip2location::resolver::IP2ProxyResolver;
use events_pipeline::sinks;
use events_pipeline::sinks::EventSink;
use events_pipeline::utils::kafka_config::create_consumer_kafka_config;
use futures::stream::FuturesUnordered;
use futures::FutureExt; // Make sure to import FutureExt
use futures::{StreamExt, TryStreamExt};
use rdkafka::config::ClientConfig;
use rdkafka::consumer::stream_consumer::StreamConsumer;
use rdkafka::consumer::{CommitMode, Consumer};
use rdkafka::message::{BorrowedMessage, Message, OwnedMessage};
use tokio::sync::{Mutex, Semaphore};
use tracing_subscriber::prelude::*;

use std::env;
use std::error::Error;
use std::path::Path;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::Arc;
use std::time::Instant;
use tokio::signal::unix::{signal, SignalKind};
use tokio::time::{sleep, Duration};

use tokio_stream::wrappers::ReceiverStream;

async fn record_borrowed_message_receipt(msg: &BorrowedMessage<'_>) {
    // Simulate some work that must be done in the same order as messages are
    // received; i.e., before truly parallel processing can begin.
    // tracing::info!("Message received: {}", msg.offset());
}

async fn record_owned_message_receipt(_msg: &OwnedMessage) {
    // Like `record_borrowed_message_receipt`, but takes an `OwnedMessage`
    // instead, as in a real-world use case  an `OwnedMessage` might be more
    // convenient than a `BorrowedMessage`.
}

// Emulates an expensive, synchronous computation.
fn get_payload(msg: OwnedMessage) -> String {
    tracing::debug!("Starting expensive computation on message {}", msg.offset());
    match msg.payload_view::<str>() {
        Some(Ok(payload)) => payload.to_owned(),
        Some(Err(_)) => "Message payload is not a string".to_owned(),
        None => "No payload".to_owned(),
    }
}

async fn handle_maxmind_db() {
    let app_env = env::var("APP_ENV").expect("APP_ENV must be set");
    let db_path = events_pipeline::geo::downloader::target_path();
    let path = Path::new(&db_path);
    if app_env != "dev" && !path.exists() {
        // Initial MaxMind database download
        tracing::info!("📁 Downloading maxmind database...");
        match download_and_save().await {
            Ok(()) => tracing::info!("✅ Maxmind db loaded."),
            Err(e) => {
                tracing::error!("🔥 Failed to download MaxMind database: {}", e);
                return;
                // return Err(e);
            }
        }
    }

    if app_env == "dev" {
        tracing::info!("📁 Skipping maxmind database download in dev mode.");
    } else {
        // Periodic MaxMind database update
        tokio::spawn(async move {
            tracing::info!(
                "🔄 Starting MaxMind database update task. Maxmind db will be updated every 12th hour."
            );
            loop {
                sleep(Duration::from_secs(12 * 60 * 60)).await; // Sleep for 12 hours
                match download_and_save().await {
                    Ok(()) => tracing::info!("Successfully updated MaxMind database."),
                    Err(e) => tracing::error!("Failed to update MaxMind database: {}", e),
                }
            }
        });
    }
}

async fn handle_ip2proxy_download() {
    let app_env = env::var("APP_ENV").expect("APP_ENV must be set");
    let path = Path::new("data/IP2PROXY-IP-PROXYTYPE-COUNTRY.BIN");
    println!("path: {:?}", path);
    println!("path exists: {:?}", path.exists());
    if !path.exists() {
        // Initial IP2Proxy database download
        tracing::info!("📁 Downloading IP2Proxy database...");
        match ip2proxy_download_and_save().await {
            Ok(()) => tracing::info!("✅ IP2Proxy db loaded."),
            Err(e) => {
                tracing::error!("🔥 Failed to download IP2Proxy database: {}", e);
                return;
                // return Err(e);
            }
        }
    }

    if app_env == "dev" {
        tracing::info!("📁 Skipping IP2Proxy database download in dev mode.");
    } else {
        // Periodic IP2Proxy database update
        tokio::spawn(async move {
            tracing::info!(
                "🔄 Starting IP2Proxy database update task. IP2Proxy db will be updated every 24th hour."
            );
            loop {
                sleep(Duration::from_secs(24 * 60 * 60)).await; // Sleep for 24 hours
                match ip2proxy_download_and_save().await {
                    Ok(()) => tracing::info!("Successfully updated IP2Proxy database."),
                    Err(e) => tracing::error!("Failed to update IP2Proxy database: {}", e),
                }
            }
        });
    }
}

// Encapsulate all the cloned variables into a struct.
#[derive(Clone)]
struct ProcessingContext {
    bot_resolver: BotResolver,
    geo_resolver: GeoResolver,
    ip2proxy_resolver: IP2ProxyResolver,
    handler: EnrichmentHandler,
    sink: sinks::kafka_event_sink::KafkaSink,
    failed_sink: sinks::kafka_event_sink::KafkaSink,
    uap: UaResolver,
}

#[derive(Debug)]
struct EventSinkError(&'static str);

impl std::fmt::Display for EventSinkError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "{}", self.0)
    }
}

impl std::error::Error for EventSinkError {}

// Emulates an expensive, synchronous computation.
async fn expensive_computation<'a>(
    msg: OwnedMessage,
    context: ProcessingContext,
) -> Result<TransformedEvent, Box<dyn std::error::Error + Send>> {
    // let start_time = Instant::now(); // Start measuring time
    let payload_value = get_payload(msg);
    // let elapsed_time = start_time.elapsed(); // Calculate elapsed time
    // println!("Time taken: {:?}", elapsed_time); // Print the elapsed time
    let result = context
        .handler
        .process_payload(
            &payload_value,
            &context.geo_resolver,
            &context.ip2proxy_resolver,
            &context.bot_resolver,
            &context.uap,
        )
        .await;
    result
}

// Creates all the resources and runs the event loop. The event loop will:
//   1) receive a stream of messages from the `StreamConsumer`.
//   2) filter out eventual Kafka errors.
//   3) send the message to a thread pool for processing.
//   4) produce the result to the output topic.
// `tokio::spawn` is used to handle IO-bound tasks in parallel (e.g., producing
// the messages), while `tokio::task::spawn_blocking` is used to handle the
// simulated CPU-bound task.
async fn start_simple_consumer() {
    // Shutdown channel
    let (shutdown_trigger, shutdown_tripwire) = oneshot::channel::<()>();

    // Listener for termination signals
    tokio::spawn(async move {
        let mut interrupt = signal(SignalKind::interrupt()).unwrap();
        let mut terminate = signal(SignalKind::terminate()).unwrap();

        tokio::select! {
            _ = interrupt.recv() => {
                tracing::info!("Received SIGINT");
            },
            _ = terminate.recv() => {
            tracing::info!("Received SIGINT");
            },
        };

        // Send the shutdown signal
        let _ = shutdown_trigger.send(());
    });

    // in-flight max tasks
    let in_flight_limiter = Arc::new(Semaphore::new(1000)); // Max 100 in-flight tasks.

    let brokers = std::env::var("KAFKA_BROKERS").unwrap_or_else(|_| "localhost".into());
    let topic = std::env::var("KAFKA_TOPIC").expect("Expected KAFKA_TOPIC");
    let output_topic = std::env::var("KAFKA_TRANSFORMATION_TOPIC")
        .unwrap_or_else(|_| "eventpipeline-transformation".into());

    // Print the topic and broker information.
    tracing::info!(
        "Starting consumer for topic '{}' on brokers '{}' and producing to topic '{}'",
        topic,
        brokers,
        &output_topic
    );

    let sink: sinks::kafka_event_sink::KafkaSink = sinks::kafka_event_sink::KafkaSink::new(
        output_topic,
        std::env::var("KAFKA_BROKERS").unwrap_or_else(|_| "localhost".into()),
        HealthRegistry::new(),
    )
    .unwrap();

    let failed_sink: sinks::kafka_event_sink::KafkaSink = sinks::kafka_event_sink::KafkaSink::new(
        std::env::var("KAFKA_TRANSFORMATION_ERROR_TOPIC")
            .unwrap_or_else(|_| "eventpipeline-transformation-error".into()),
        std::env::var("KAFKA_BROKERS").unwrap_or_else(|_| "localhost".into()),
        HealthRegistry::new(),
    )
    .unwrap();

    let handler = EnrichmentHandler::new();
    let geo_resolver = GeoResolver::new(&events_pipeline::geo::downloader::target_path()).unwrap();
    let ip2proxy_resolver: IP2ProxyResolver =
        IP2ProxyResolver::new("data/IP2PROXY-IP-PROXYTYPE-COUNTRY.BIN").unwrap();
    let bot_resolver = BotResolver::new();
    let config: ClientConfig = create_consumer_kafka_config(brokers);
    let consumer: Arc<StreamConsumer> =
        Arc::new(config.create().expect("Consumer creation failed"));
    let ua_parser = UaResolver::new();
    ua_parser.seed_to_lru_cache().unwrap();
    consumer
        .subscribe(&[&topic])
        .expect("Can't subscribe to specified topics");

    // Create the ProcessingContext outside the for_each.
    let context = ProcessingContext {
        bot_resolver: bot_resolver,
        geo_resolver: geo_resolver,
        handler: handler,
        sink: sink,
        failed_sink: failed_sink,
        uap: ua_parser,
        ip2proxy_resolver: ip2proxy_resolver,
    };

    // Create the outer pipeline on the message stream.
    let stream_processor = consumer
        .stream()
        .take_until(shutdown_tripwire.map(|_| ())) // Stop consuming when the shutdown signal is received
        .try_for_each(|borrowed_message| {
            let context = context.clone();
            let consumer_ref = consumer.clone();
            let permit_future = Arc::clone(&in_flight_limiter).acquire_owned(); // Do not await here, return Future instead

            async move {
                let permit = permit_future.await.expect("Failed to acquire permit");

                // Process each message
                record_borrowed_message_receipt(&borrowed_message).await;
                // Borrowed messages can't outlive the consumer they are received from, so they need to
                // be owned in order to be sent to a separate thread.
                let owned_message = borrowed_message.detach();
                record_owned_message_receipt(&owned_message).await;
                tokio::spawn(async move {
                    // The body of this block will be executed on the main thread pool,
                    // but we perform `expensive_computation` on a separate thread pool
                    // for CPU-intensive tasks via `tokio::task::spawn_blocking`.
                    let context_in_blocking = context.clone(); // clone context here

                    let computation_result = tokio::task::spawn_blocking(move || {
                        expensive_computation(owned_message, context_in_blocking)
                    })
                    .await
                    .expect("failed to wait for expensive computation");

                    // Track whether processing succeeded for offset commit
                    let mut send_ok = false;

                    // computation_result is a Result, so it can be matched on directly.
                    match computation_result.await {
                        Ok(transformed_event) => {
                            let event = sinks::EventTypes::Transformed(transformed_event);

                            match context.sink.send(event).await {
                                Ok(()) => {
                                    tracing::debug!("Successfully sent transformed event to sink");
                                    send_ok = true;
                                }
                                Err(e) => tracing::error!("Failed to send event to sink: {:?}", e),
                            }
                        }
                        Err(e) => match e.downcast::<MyError>() {
                            Ok(my_error) => {
                                let failed_event = my_error.failed_event;
                                let event = sinks::EventTypes::Failed(failed_event.clone());
                                match context.failed_sink.send(event).await {
                                    Ok(()) => {
                                        tracing::debug!("Successfully sent failed event to sink");
                                        send_ok = true;
                                    }
                                    Err(e) => {
                                        tracing::error!(
                                            "Failed to send failed event to sink: {:?}",
                                            e
                                        )
                                    }
                                }
                            }
                            Err(err) => {
                                tracing::error!("Failed to process payload: {:?}", err);
                            }
                        },
                    }

                    // Manually commit offset after successful processing
                    if send_ok {
                        if let Err(e) = consumer_ref.commit_consumer_state(CommitMode::Async) {
                            tracing::error!("Failed to commit consumer offset: {:?}", e);
                        }
                    }

                    drop(permit); // Release permit when task finishes.
                });

                Ok(())
            }
        });

    tracing::info!("Starting event loop");
    stream_processor.await.expect("stream processing failed");
    tracing::info!("Stream processing terminated");

    // After the stream processing terminated, wait for all in-flight tasks to finish.
    while in_flight_limiter.available_permits() != 1000 {
        tokio::time::sleep(tokio::time::Duration::from_millis(100)).await;
    }
    tracing::info!("All in-flight tasks completed, shutdown complete");
}

#[tokio::main]
async fn main() {
    dotenv().ok();
    tracing::info!("Starting the simple consumer");
    let log_level = std::env::var("LOG_LEVEL").unwrap_or_else(|_| "INFO".to_string());
    let filter = format!("{},tower_http=ERROR", log_level);

    // let console_layer = console_subscriber::spawn();
    // tracing_subscriber::registry()
    // .with(console_layer)
    // .with(
    //     tracing_subscriber::fmt::layer()
    //         .with_ansi(false)
    //         .without_time()
    //         .with_filter(if log_level == "DEBUG" {
    //             tracing_subscriber::filter::LevelFilter::DEBUG
    //         } else {
    //             tracing_subscriber::filter::LevelFilter::INFO
    //         }),
    // )
    // .init();

    // // Initialize tracing
    tracing_subscriber::fmt()
        .with_env_filter(tracing_subscriber::EnvFilter::new(filter))
        .init();

    handle_maxmind_db().await;
    handle_ip2proxy_download().await;

    let num_workers = 1;
    (0..num_workers)
        .map(|_| tokio::spawn(start_simple_consumer()))
        .collect::<FuturesUnordered<_>>()
        .for_each(|_| async { () })
        .await
}
