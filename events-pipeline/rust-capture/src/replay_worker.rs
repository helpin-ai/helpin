use dotenv::dotenv;
use events_pipeline::health::HealthRegistry;
use events_pipeline::sinks::kafka_event_sink::KafkaSink;
use events_pipeline::sinks::{EventSink, EventTypes};
use std::env;
use std::fs;
use std::path::{Path, PathBuf};
use std::time::{Duration, SystemTime};
use tracing_subscriber::EnvFilter;

#[tokio::main]
async fn main() {
    dotenv().ok();

    let log_level = env::var("LOG_LEVEL").unwrap_or_else(|_| "INFO".to_string());
    tracing_subscriber::fmt()
        .with_env_filter(EnvFilter::new(log_level))
        .init();

    let base_dir =
        PathBuf::from(env::var("FALLBACK_DIR").unwrap_or_else(|_| "data/fallback".to_string()));
    let active_dir = base_dir.join("active");
    let pending_dir = base_dir.join("pending");
    let processing_dir = base_dir.join("processing");
    let completed_dir = base_dir.join("completed");

    for dir in [&active_dir, &pending_dir, &processing_dir, &completed_dir] {
        fs::create_dir_all(dir).unwrap_or_else(|e| {
            panic!("Failed to create directory {:?}: {}", dir, e);
        });
    }

    let brokers = env::var("KAFKA_BROKERS").expect("KAFKA_BROKERS must be set");
    let topic = env::var("KAFKA_TOPIC").expect("KAFKA_TOPIC must be set");
    let batch_size: usize = env::var("REPLAY_BATCH_SIZE")
        .ok()
        .and_then(|v| v.parse().ok())
        .unwrap_or(100);
    let poll_interval_secs: u64 = env::var("REPLAY_POLL_INTERVAL_SECS")
        .ok()
        .and_then(|v| v.parse().ok())
        .unwrap_or(5);
    let cleanup_age_hours: u64 = env::var("REPLAY_CLEANUP_HOURS")
        .ok()
        .and_then(|v| v.parse().ok())
        .unwrap_or(24);

    let sink = KafkaSink::new(topic, brokers, HealthRegistry::new())
        .expect("Failed to create Kafka sink for replay");

    tracing::info!("Replay worker started, watching {:?}", pending_dir);

    // Recovery: move interrupted processing files back to pending
    recover_processing_files(&processing_dir, &pending_dir);

    loop {
        match process_pending_files(
            &pending_dir,
            &processing_dir,
            &completed_dir,
            &sink,
            batch_size,
        )
        .await
        {
            Ok(files_processed) => {
                if files_processed > 0 {
                    tracing::info!("Replayed {} files to Kafka", files_processed);
                }
            }
            Err(e) => {
                tracing::error!("Error processing pending files: {}", e);
            }
        }

        if let Err(e) = cleanup_completed(&completed_dir, cleanup_age_hours) {
            tracing::warn!("Error cleaning up completed files: {}", e);
        }

        tokio::time::sleep(Duration::from_secs(poll_interval_secs)).await;
    }
}

/// On startup, move any files from processing/ back to pending/ (interrupted replays).
fn recover_processing_files(processing_dir: &Path, pending_dir: &Path) {
    let entries = match fs::read_dir(processing_dir) {
        Ok(e) => e,
        Err(_) => return,
    };

    for entry in entries.filter_map(|e| e.ok()) {
        let file_name = entry.file_name();
        let dest = pending_dir.join(&file_name);
        match fs::rename(entry.path(), &dest) {
            Ok(()) => tracing::info!("Recovered interrupted file: {:?}", file_name),
            Err(e) => tracing::error!(
                "Failed to recover processing file {:?}: {}",
                entry.path(),
                e
            ),
        }
    }
}

async fn process_pending_files(
    pending_dir: &Path,
    processing_dir: &Path,
    completed_dir: &Path,
    sink: &KafkaSink,
    batch_size: usize,
) -> Result<usize, String> {
    let entries =
        fs::read_dir(pending_dir).map_err(|e| format!("Failed to read pending dir: {}", e))?;

    let mut files: Vec<PathBuf> = entries
        .filter_map(|entry| entry.ok())
        .map(|entry| entry.path())
        .filter(|path| {
            path.extension().map_or(false, |ext| ext == "jsonl")
                && !path.to_string_lossy().ends_with(".jsonl.tmp")
        })
        .collect();

    files.sort(); // Oldest first (lexicographic sort on timestamp prefix)

    let mut processed = 0;
    for file_path in files {
        match replay_file(&file_path, processing_dir, completed_dir, sink, batch_size).await {
            Ok(()) => {
                processed += 1;
                metrics::counter!("replay_files_processed_total", 1);
            }
            Err(e) => {
                tracing::error!("Failed to replay file {:?}: {}", file_path, e);
                break; // Stop processing if Kafka is down
            }
        }
    }

    Ok(processed)
}

async fn replay_file(
    file_path: &Path,
    processing_dir: &Path,
    completed_dir: &Path,
    sink: &KafkaSink,
    batch_size: usize,
) -> Result<(), String> {
    let file_name = file_path
        .file_name()
        .ok_or_else(|| "Invalid file path".to_string())?;

    // Move to processing
    let processing_path = processing_dir.join(file_name);
    fs::rename(file_path, &processing_path)
        .map_err(|e| format!("Failed to move to processing: {}", e))?;

    tracing::info!("Replaying file: {:?}", file_name);

    // Read and replay
    let contents =
        fs::read_to_string(&processing_path).map_err(|e| format!("Failed to read file: {}", e))?;

    let mut batch: Vec<EventTypes> = Vec::with_capacity(batch_size);
    let mut total_events = 0;
    let mut failed_lines = 0;

    for line in contents.lines() {
        if line.trim().is_empty() {
            continue;
        }

        match serde_json::from_str::<EventTypes>(line) {
            Ok(event) => {
                batch.push(event);
                if batch.len() >= batch_size {
                    let count = batch.len();
                    if let Err(e) = sink.send_batch(batch).await {
                        // Move file back to pending for retry
                        let _ = fs::rename(&processing_path, file_path);
                        return Err(format!("Kafka send failed: {:?}", e));
                    }
                    total_events += count;
                    batch = Vec::with_capacity(batch_size);
                }
            }
            Err(e) => {
                tracing::warn!("Failed to deserialize event line: {}", e);
                failed_lines += 1;
            }
        }
    }

    // Send remaining batch
    if !batch.is_empty() {
        let remaining = batch.len();
        if let Err(e) = sink.send_batch(batch).await {
            let _ = fs::rename(&processing_path, file_path);
            return Err(format!("Kafka send failed: {:?}", e));
        }
        total_events += remaining;
    }

    // Move to completed
    let completed_path = completed_dir.join(file_name);
    fs::rename(&processing_path, &completed_path)
        .map_err(|e| format!("Failed to move to completed: {}", e))?;

    tracing::info!(
        "Replayed {:?}: {} events sent, {} lines skipped",
        file_name,
        total_events,
        failed_lines
    );
    metrics::counter!("replay_events_total", total_events as u64);

    Ok(())
}

fn cleanup_completed(completed_dir: &Path, max_age_hours: u64) -> Result<(), String> {
    let entries =
        fs::read_dir(completed_dir).map_err(|e| format!("Failed to read completed dir: {}", e))?;

    let max_age = Duration::from_secs(max_age_hours * 3600);
    let now = SystemTime::now();

    for entry in entries.filter_map(|e| e.ok()) {
        let metadata = match entry.metadata() {
            Ok(m) => m,
            Err(_) => continue,
        };

        if let Ok(modified) = metadata.modified() {
            if let Ok(age) = now.duration_since(modified) {
                if age > max_age {
                    if let Err(e) = fs::remove_file(entry.path()) {
                        tracing::warn!(
                            "Failed to remove old completed file {:?}: {}",
                            entry.path(),
                            e
                        );
                    } else {
                        tracing::debug!("Cleaned up completed file: {:?}", entry.path());
                    }
                }
            }
        }
    }

    Ok(())
}
