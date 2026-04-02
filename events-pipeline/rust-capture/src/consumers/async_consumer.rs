use dotenv::dotenv;
use events_pipeline::enrichment::bot_resolver::BotResolver;
use events_pipeline::enrichment::handler::{EnrichmentHandler, MyError};
use events_pipeline::enrichment::ua_resolver::UaResolver;
use events_pipeline::geo::downloader::download_and_save;
use events_pipeline::geo::resolver::GeoResolver;
use events_pipeline::health::HealthRegistry;
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
use tokio::sync::Mutex;
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

async fn start_consumer() {
    let use_print_sink = env::var("PRINT_SINK").expect("PRINT_SINK must be set");
    let app_env = env::var("APP_ENV").expect("APP_ENV must be set");
    let db_path = events_pipeline::geo::downloader::target_path();
    let path = Path::new(&db_path);

    if use_print_sink == "true" {
        tracing::info!("Consumer using print sink. Exiting...");
        return;
        // return Ok(());
    }

    let brokers = std::env::var("KAFKA_BROKERS").unwrap_or_else(|_| "localhost".into());
    let topic = std::env::var("KAFKA_TOPIC").expect("Expected KAFKA_TOPIC");
    tracing::info!("Worker started");
    tracing::debug!(
        "Consumer: kafka topic and brokers are: {} {}",
        topic,
        brokers
    );
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
    // Producer related sink (reserved for future fallback use)
    let _disk_sink = Arc::new(sinks::disk_sink::DiskSink::new(
        "fallback-processed-events".into(),
    ));

    let sink: sinks::kafka_event_sink::KafkaSink = sinks::kafka_event_sink::KafkaSink::new(
        std::env::var("KAFKA_TRANSFORMATION_TOPIC").unwrap_or_else(|_| "transformed_events".into()),
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
    consumer
        .subscribe(&[&topic])
        .expect("Can't subscribe to specified topics");

    let (processor_sender, mut processor_receiver) =
        tokio::sync::mpsc::channel::<tokio::task::JoinHandle<()>>(1000);
    let processor_tasks = Arc::new(Mutex::new(FuturesUnordered::new()));

    let processor_tasks_spawn = Arc::clone(&processor_tasks);
    tokio::spawn(async move {
        while let Some(task) = processor_receiver.recv().await {
            processor_tasks_spawn.lock().await.push(task);
        }
    });

    // Create the ProcessingContext outside the for_each.
    let context = ProcessingContext {
        bot_resolver: bot_resolver,
        geo_resolver: geo_resolver,
        ip2proxy_resolver: ip2proxy_resolver,
        handler: handler,
        sink: sink,
        failed_sink: failed_sink,
        uap: ua_parser,
    };

    // Create the outer pipeline on the message stream.
    let stream_processor = consumer
        .stream()
        .take_until(shutdown_tripwire.map(|_| ())) // Stop consuming when the shutdown signal is received
        .try_for_each(|borrowed_message| {
            let context = context.clone();
            let processor_sender = processor_sender.clone();
            let consumer_ref = consumer.clone();
            async move {
                // Process each message
                record_borrowed_message_receipt(&borrowed_message).await;
                // Borrowed messages can't outlive the consumer they are received from, so they need to
                // be owned in order to be sent to a separate thread.
                let owned_message = borrowed_message.detach();
                record_owned_message_receipt(&owned_message).await;
                let processing_task = tokio::spawn(async move {
                    let payload_value = get_payload(owned_message);
                    let mut send_ok = false;

                    match context
                        .handler
                        .process_payload(
                            &payload_value,
                            &context.geo_resolver,
                            &context.ip2proxy_resolver,
                            &context.bot_resolver,
                            &context.uap,
                        )
                        .await
                    {
                        Ok(transformed_event) => {
                            let event = sinks::EventTypes::Transformed(transformed_event.clone());
                            match context.sink.send(event).await {
                                Ok(()) => {
                                    send_ok = true;
                                }
                                Err(e) => {
                                    tracing::error!("Failed to send event to sink: {:?}", e);
                                }
                            }
                        }
                        Err(e) => match e.downcast::<MyError>() {
                            Ok(my_error) => {
                                let failed_event = my_error.failed_event;
                                let event = sinks::EventTypes::Failed(failed_event.clone());
                                match context.failed_sink.send(event).await {
                                    Ok(()) => {
                                        send_ok = true;
                                    }
                                    Err(e) => {
                                        tracing::error!("Failed to send event to sink: {:?}", e);
                                    }
                                }
                            }
                            Err(err) => {
                                tracing::error!("Failed to process payload:{:?}", err);
                            }
                        },
                    }

                    // Manually commit offset after successful processing
                    if send_ok {
                        if let Err(e) = consumer_ref.commit_consumer_state(CommitMode::Async) {
                            tracing::error!("Failed to commit consumer offset: {:?}", e);
                        }
                    }
                });
                processor_sender
                    .send(processing_task)
                    .await
                    .expect("Failed to send processing task");
                Ok(())
            }
        });
    // Wait for the stream_processor to finish

    stream_processor.await.expect("stream processing failed");

    // Close the processor_sender
    drop(processor_sender);

    // Wait for all processing tasks to finish
    while processor_tasks.lock().await.next().await.is_some() {}
}

#[tokio::main]
async fn main() {
    dotenv().ok();

    let log_level = std::env::var("LOG_LEVEL").unwrap_or_else(|_| "INFO".to_string());
    let filter = format!("{},tower_http=ERROR", log_level);

    let console_layer = console_subscriber::spawn();
    tracing_subscriber::registry()
        .with(console_layer)
        .with(
            tracing_subscriber::fmt::layer()
                .with_ansi(false)
                .without_time()
                .with_filter(if log_level == "DEBUG" {
                    tracing_subscriber::filter::LevelFilter::DEBUG
                } else {
                    tracing_subscriber::filter::LevelFilter::INFO
                }),
        )
        .init();

    // // Initialize tracing
    // tracing_subscriber::fmt()
    //     .with_env_filter(tracing_subscriber::EnvFilter::new(filter))
    //     .init();

    handle_maxmind_db().await;
    let num_workers = 1;
    (0..num_workers)
        .map(|_| tokio::spawn(start_consumer()))
        .collect::<FuturesUnordered<_>>()
        .for_each(|_| async { () })
        .await
}
