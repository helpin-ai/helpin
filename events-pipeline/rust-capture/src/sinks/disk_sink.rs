use super::{EventSink, EventTypes};
use crate::api::CaptureError;
use std::fs::{self, File, OpenOptions};
use std::io;
use std::io::Write;
use std::path::PathBuf;
use std::sync::Arc;
use std::time::{Instant, SystemTime, UNIX_EPOCH};
use tokio::sync::Mutex;
use uuid::Uuid;

use async_trait::async_trait;

const DEFAULT_MAX_SEGMENT_BYTES: u64 = 10 * 1024 * 1024; // 10MB
const DEFAULT_MAX_SEGMENT_AGE_SECS: u64 = 60;

struct ActiveSegment {
    final_path: PathBuf,
    tmp_path: PathBuf,
    file: File,
    size: u64,
    created_at: Instant,
}

pub struct DiskSink {
    base_dir: PathBuf,
    segment: Arc<Mutex<Option<ActiveSegment>>>,
    max_segment_bytes: u64,
    max_segment_age_secs: u64,
}

impl Clone for DiskSink {
    fn clone(&self) -> Self {
        DiskSink {
            base_dir: self.base_dir.clone(),
            segment: self.segment.clone(),
            max_segment_bytes: self.max_segment_bytes,
            max_segment_age_secs: self.max_segment_age_secs,
        }
    }
}

impl DiskSink {
    pub fn new(base_dir: PathBuf) -> Self {
        Self::ensure_directories(&base_dir);
        Self::recover_orphaned_segments(&base_dir);

        DiskSink {
            base_dir,
            segment: Arc::new(Mutex::new(None)),
            max_segment_bytes: DEFAULT_MAX_SEGMENT_BYTES,
            max_segment_age_secs: DEFAULT_MAX_SEGMENT_AGE_SECS,
        }
    }

    #[cfg(test)]
    pub fn with_rotation(
        base_dir: PathBuf,
        max_segment_bytes: u64,
        max_segment_age_secs: u64,
    ) -> Self {
        let sink = Self::new(base_dir);
        DiskSink {
            max_segment_bytes,
            max_segment_age_secs,
            ..sink
        }
    }

    pub fn pending_dir(&self) -> PathBuf {
        self.base_dir.join("pending")
    }

    pub fn active_dir(&self) -> PathBuf {
        self.base_dir.join("active")
    }

    pub fn processing_dir(&self) -> PathBuf {
        self.base_dir.join("processing")
    }

    pub fn completed_dir(&self) -> PathBuf {
        self.base_dir.join("completed")
    }

    /// Finalize the current segment and close. Call during graceful shutdown.
    pub async fn close(&self) -> Result<(), CaptureError> {
        let mut guard = self.segment.lock().await;
        if let Some(segment) = guard.take() {
            Self::finalize_segment(segment)?;
        }
        Ok(())
    }

    fn ensure_directories(base_dir: &PathBuf) {
        let pending_dir = base_dir.join("pending");
        let active_dir = base_dir.join("active");
        let processing_dir = base_dir.join("processing");
        let completed_dir = base_dir.join("completed");

        for dir in [&pending_dir, &active_dir, &processing_dir, &completed_dir] {
            if let Err(e) = fs::create_dir_all(dir) {
                tracing::error!("Failed to create directory {:?}: {}", dir, e);
            }
        }
    }

    fn recover_orphaned_segments(base_dir: &PathBuf) {
        let active_dir = base_dir.join("active");
        let pending_dir = base_dir.join("pending");
        let entries = match fs::read_dir(&active_dir) {
            Ok(entries) => entries,
            Err(e) => {
                tracing::debug!(
                    "Skipping orphaned segment recovery for {:?}: {}",
                    active_dir,
                    e
                );
                return;
            }
        };

        for entry in entries.filter_map(|entry| entry.ok()) {
            let path = entry.path();
            if !path.to_string_lossy().ends_with(".jsonl.tmp") {
                continue;
            }

            let Some(file_name) = path.file_name().and_then(|name| name.to_str()) else {
                continue;
            };

            let final_name = file_name.trim_end_matches(".tmp");
            let final_path = pending_dir.join(final_name);

            if let Some(parent) = final_path.parent() {
                if let Err(e) = fs::create_dir_all(parent) {
                    tracing::error!(
                        "Failed to create pending directory {:?} during recovery: {}",
                        parent,
                        e
                    );
                    continue;
                }
            }

            match fs::rename(&path, &final_path) {
                Ok(()) => tracing::info!(
                    "Recovered orphaned disk segment {:?} into pending",
                    path.file_name().unwrap_or_default()
                ),
                Err(e) => {
                    tracing::warn!("Failed to recover orphaned disk segment {:?}: {}", path, e)
                }
            }
        }
    }

    fn create_new_segment(
        active_dir: &PathBuf,
        pending_dir: &PathBuf,
    ) -> Result<ActiveSegment, CaptureError> {
        let timestamp = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap_or_default()
            .as_secs();
        let id = Uuid::new_v4();
        let base_name = format!("{}_{}.jsonl", timestamp, id);
        let tmp_name = format!("{}.tmp", base_name);

        let final_path = pending_dir.join(&base_name);
        let tmp_path = active_dir.join(&tmp_name);

        let file = OpenOptions::new()
            .write(true)
            .create(true)
            .truncate(true)
            .open(&tmp_path)
            .map_err(|e| {
                CaptureError::NonRetryableSinkError(format!(
                    "Failed to create segment file {:?}: {}",
                    tmp_path, e
                ))
            })?;

        Ok(ActiveSegment {
            final_path,
            tmp_path,
            file,
            size: 0,
            created_at: Instant::now(),
        })
    }

    fn finalize_segment(mut segment: ActiveSegment) -> Result<(), CaptureError> {
        segment.file.flush().map_err(|e| {
            CaptureError::NonRetryableSinkError(format!("Failed to flush segment: {}", e))
        })?;
        drop(segment.file);

        if segment.size > 0 {
            if let Some(parent) = segment.final_path.parent() {
                fs::create_dir_all(parent).map_err(|e| {
                    CaptureError::NonRetryableSinkError(format!(
                        "Failed to ensure segment directory {:?}: {}",
                        parent, e
                    ))
                })?;
            }

            match fs::rename(&segment.tmp_path, &segment.final_path) {
                Ok(()) => {}
                Err(e) if e.kind() == io::ErrorKind::NotFound && segment.final_path.exists() => {
                    tracing::warn!(
                        "Disk segment already finalized externally: {:?}",
                        segment.final_path.file_name().unwrap_or_default()
                    );
                }
                Err(e) => {
                    return Err(CaptureError::NonRetryableSinkError(format!(
                        "Failed to rename {:?} to {:?}: {}",
                        segment.tmp_path, segment.final_path, e
                    )))
                }
            }
            tracing::info!(
                "Finalized disk segment: {:?} ({} bytes)",
                segment.final_path.file_name().unwrap_or_default(),
                segment.size
            );
            metrics::counter!("capture_disk_segments_finalized_total", 1);
        } else {
            let _ = fs::remove_file(&segment.tmp_path);
        }

        Ok(())
    }

    fn needs_rotation(&self, segment: &ActiveSegment) -> bool {
        segment.size >= self.max_segment_bytes
            || segment.created_at.elapsed().as_secs() >= self.max_segment_age_secs
    }
}

#[async_trait]
impl EventSink for DiskSink {
    async fn send(&self, event: EventTypes) -> Result<(), CaptureError> {
        self.send_batch(vec![event]).await
    }

    async fn send_batch(&self, events: Vec<EventTypes>) -> Result<(), CaptureError> {
        let mut guard = self.segment.lock().await;
        let pending_dir = self.pending_dir();
        let active_dir = self.active_dir();
        Self::ensure_directories(&self.base_dir);

        // Check if rotation is needed
        let needs_new = match guard.as_ref() {
            None => true,
            Some(seg) => self.needs_rotation(seg),
        };

        if needs_new {
            if let Some(old_segment) = guard.take() {
                Self::finalize_segment(old_segment)?;
            }
            *guard = Some(Self::create_new_segment(&active_dir, &pending_dir)?);
        }

        let segment = guard.as_mut().unwrap();

        for event in &events {
            let payload = serde_json::to_string(event).map_err(|e| {
                CaptureError::NonRetryableSinkError(format!("Failed to serialize event: {}", e))
            })?;
            let line = format!("{}\n", payload);
            let bytes = line.as_bytes();
            segment.file.write_all(bytes).map_err(|e| {
                CaptureError::NonRetryableSinkError(format!(
                    "Failed to write to disk segment: {}",
                    e
                ))
            })?;
            segment.size += bytes.len() as u64;
        }

        segment.file.flush().map_err(|e| {
            CaptureError::NonRetryableSinkError(format!("Failed to flush disk segment: {}", e))
        })?;

        metrics::counter!("capture_disk_events_written_total", events.len() as u64);
        tracing::debug!(
            "Wrote {} events to disk segment ({} bytes total)",
            events.len(),
            segment.size
        );

        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::events::event::{Event, ProcessedEvent};
    use std::collections::HashMap;

    fn make_test_event(api_key: &str) -> EventTypes {
        EventTypes::Processed(ProcessedEvent {
            event: Event {
                api_key: api_key.to_string(),
                event_type: "test".to_string(),
                user: HashMap::new(),
                ..Default::default()
            },
            event_id: Uuid::new_v4(),
        })
    }

    fn make_large_event(size_hint: usize) -> EventTypes {
        let padding = "x".repeat(size_hint);
        EventTypes::Processed(ProcessedEvent {
            event: Event {
                api_key: padding,
                event_type: "test".to_string(),
                user: HashMap::new(),
                ..Default::default()
            },
            event_id: Uuid::new_v4(),
        })
    }

    #[tokio::test]
    async fn test_creates_directory_structure() {
        let dir = std::env::temp_dir().join(format!("disk_sink_dirs_{}", Uuid::new_v4()));
        let _sink = DiskSink::new(dir.clone());

        assert!(dir.join("pending").exists());
        assert!(dir.join("active").exists());
        assert!(dir.join("processing").exists());
        assert!(dir.join("completed").exists());

        fs::remove_dir_all(&dir).unwrap();
    }

    #[tokio::test]
    async fn test_write_single_event() {
        let dir = std::env::temp_dir().join(format!("disk_sink_single_{}", Uuid::new_v4()));
        let sink = DiskSink::new(dir.clone());

        sink.send(make_test_event("test_key")).await.unwrap();
        sink.close().await.unwrap();

        // After close, segment should be finalized (renamed from .tmp to .jsonl)
        let pending = dir.join("pending");
        let files: Vec<_> = fs::read_dir(&pending)
            .unwrap()
            .filter_map(|e| e.ok())
            .filter(|e| e.path().extension().map_or(false, |ext| ext == "jsonl"))
            .collect();

        assert_eq!(files.len(), 1, "Should have exactly 1 finalized segment");

        let contents = fs::read_to_string(files[0].path()).unwrap();
        assert!(contents.contains("test_key"));
        assert_eq!(contents.lines().count(), 1);

        fs::remove_dir_all(&dir).unwrap();
    }

    #[tokio::test]
    async fn test_write_batch() {
        let dir = std::env::temp_dir().join(format!("disk_sink_batch_{}", Uuid::new_v4()));
        let sink = DiskSink::new(dir.clone());

        let events = vec![
            make_test_event("key1"),
            make_test_event("key2"),
            make_test_event("key3"),
        ];

        sink.send_batch(events).await.unwrap();
        sink.close().await.unwrap();

        let pending = dir.join("pending");
        let files: Vec<_> = fs::read_dir(&pending)
            .unwrap()
            .filter_map(|e| e.ok())
            .filter(|e| e.path().extension().map_or(false, |ext| ext == "jsonl"))
            .collect();

        assert_eq!(files.len(), 1);
        let contents = fs::read_to_string(files[0].path()).unwrap();
        assert_eq!(contents.lines().count(), 3);
        assert!(contents.contains("key1"));
        assert!(contents.contains("key2"));
        assert!(contents.contains("key3"));

        fs::remove_dir_all(&dir).unwrap();
    }

    #[tokio::test]
    async fn test_segment_rotation_by_size() {
        let dir = std::env::temp_dir().join(format!("disk_sink_rot_size_{}", Uuid::new_v4()));
        // Set max segment to 100 bytes — each event is larger than that
        let sink = DiskSink::with_rotation(dir.clone(), 100, 3600);

        // Write two events — each should trigger rotation
        sink.send(make_test_event("first")).await.unwrap();
        sink.send(make_test_event("second")).await.unwrap();
        sink.close().await.unwrap();

        let pending = dir.join("pending");
        let files: Vec<_> = fs::read_dir(&pending)
            .unwrap()
            .filter_map(|e| e.ok())
            .filter(|e| e.path().extension().map_or(false, |ext| ext == "jsonl"))
            .collect();

        // First event creates segment, second triggers rotation (new segment)
        assert_eq!(
            files.len(),
            2,
            "Should have 2 segments after size-based rotation"
        );

        fs::remove_dir_all(&dir).unwrap();
    }

    #[tokio::test]
    async fn test_segment_rotation_by_age() {
        let dir = std::env::temp_dir().join(format!("disk_sink_rot_age_{}", Uuid::new_v4()));
        // Set max segment age to 0 seconds — every write triggers rotation
        let sink = DiskSink::with_rotation(dir.clone(), u64::MAX, 0);

        sink.send(make_test_event("first")).await.unwrap();
        // Sleep to ensure the segment ages past 0 seconds
        tokio::time::sleep(std::time::Duration::from_millis(10)).await;
        sink.send(make_test_event("second")).await.unwrap();
        sink.close().await.unwrap();

        let pending = dir.join("pending");
        let files: Vec<_> = fs::read_dir(&pending)
            .unwrap()
            .filter_map(|e| e.ok())
            .filter(|e| e.path().extension().map_or(false, |ext| ext == "jsonl"))
            .collect();

        assert_eq!(
            files.len(),
            2,
            "Should have 2 segments after age-based rotation"
        );

        fs::remove_dir_all(&dir).unwrap();
    }

    #[tokio::test]
    async fn test_tmp_file_while_writing() {
        let dir = std::env::temp_dir().join(format!("disk_sink_tmp_{}", Uuid::new_v4()));
        let sink = DiskSink::new(dir.clone());

        sink.send(make_test_event("test")).await.unwrap();

        // Before close, the file should still be .tmp
        let active = dir.join("active");
        let tmp_files: Vec<_> = fs::read_dir(&active)
            .unwrap()
            .filter_map(|e| e.ok())
            .filter(|e| e.path().to_string_lossy().ends_with(".jsonl.tmp"))
            .collect();

        assert_eq!(tmp_files.len(), 1, "Should have 1 .tmp file before close");

        // No finalized .jsonl files yet
        let pending = dir.join("pending");
        let jsonl_files: Vec<_> = fs::read_dir(&pending)
            .unwrap()
            .filter_map(|e| e.ok())
            .filter(|e| {
                let path = e.path();
                path.extension().map_or(false, |ext| ext == "jsonl")
                    && !path.to_string_lossy().ends_with(".jsonl.tmp")
            })
            .collect();

        assert_eq!(
            jsonl_files.len(),
            0,
            "Should have no finalized files before close"
        );

        sink.close().await.unwrap();

        // After close, should have finalized .jsonl file
        let jsonl_files: Vec<_> = fs::read_dir(&pending)
            .unwrap()
            .filter_map(|e| e.ok())
            .filter(|e| {
                let path = e.path();
                path.extension().map_or(false, |ext| ext == "jsonl")
                    && !path.to_string_lossy().ends_with(".jsonl.tmp")
            })
            .collect();

        assert_eq!(
            jsonl_files.len(),
            1,
            "Should have 1 finalized file after close"
        );

        fs::remove_dir_all(&dir).unwrap();
    }

    #[tokio::test]
    async fn test_close_succeeds_if_segment_was_already_finalized() {
        let dir =
            std::env::temp_dir().join(format!("disk_sink_already_finalized_{}", Uuid::new_v4()));
        let sink = DiskSink::new(dir.clone());

        sink.send(make_test_event("race")).await.unwrap();

        let active = dir.join("active");
        let pending = dir.join("pending");
        let tmp_file = fs::read_dir(&active)
            .unwrap()
            .filter_map(|e| e.ok())
            .map(|e| e.path())
            .find(|path| path.to_string_lossy().ends_with(".jsonl.tmp"))
            .expect("tmp segment should exist");
        let final_name = tmp_file
            .file_name()
            .unwrap()
            .to_string_lossy()
            .trim_end_matches(".tmp")
            .to_string();
        let final_path = pending.join(final_name);

        fs::rename(&tmp_file, &final_path).unwrap();

        sink.close().await.unwrap();

        let contents = fs::read_to_string(final_path).unwrap();
        assert!(contents.contains("race"));

        fs::remove_dir_all(&dir).unwrap();
    }

    #[tokio::test]
    async fn test_recovers_orphaned_active_segments_on_startup() {
        let dir =
            std::env::temp_dir().join(format!("disk_sink_recover_orphaned_{}", Uuid::new_v4()));
        let active = dir.join("active");
        let pending = dir.join("pending");
        fs::create_dir_all(&active).unwrap();
        fs::create_dir_all(&pending).unwrap();

        let orphaned_tmp = active.join("123_orphaned.jsonl.tmp");
        fs::write(&orphaned_tmp, "{\"event\":\"orphaned\"}\n").unwrap();

        let sink = DiskSink::new(dir.clone());
        sink.close().await.unwrap();

        assert!(!orphaned_tmp.exists());
        let recovered = pending.join("123_orphaned.jsonl");
        assert!(recovered.exists());
        let contents = fs::read_to_string(recovered).unwrap();
        assert!(contents.contains("orphaned"));

        fs::remove_dir_all(&dir).unwrap();
    }

    #[tokio::test]
    async fn test_close_on_empty_sink() {
        let dir = std::env::temp_dir().join(format!("disk_sink_empty_close_{}", Uuid::new_v4()));
        let sink = DiskSink::new(dir.clone());

        // Close without writing anything — should not error
        let result = sink.close().await;
        assert!(result.is_ok());

        fs::remove_dir_all(&dir).unwrap();
    }

    #[tokio::test]
    async fn test_invalid_dir_returns_error() {
        let sink = DiskSink::new(PathBuf::from("/nonexistent/deeply/nested/fallback"));

        let result = sink.send(make_test_event("key")).await;
        assert!(result.is_err(), "Writing to invalid path should fail");
    }

    #[tokio::test]
    async fn test_events_are_valid_json() {
        let dir = std::env::temp_dir().join(format!("disk_sink_json_{}", Uuid::new_v4()));
        let sink = DiskSink::new(dir.clone());

        sink.send_batch(vec![make_test_event("json1"), make_test_event("json2")])
            .await
            .unwrap();
        sink.close().await.unwrap();

        let pending = dir.join("pending");
        let files: Vec<_> = fs::read_dir(&pending)
            .unwrap()
            .filter_map(|e| e.ok())
            .filter(|e| e.path().extension().map_or(false, |ext| ext == "jsonl"))
            .collect();

        for file in files {
            let contents = fs::read_to_string(file.path()).unwrap();
            for line in contents.lines() {
                let parsed: Result<serde_json::Value, _> = serde_json::from_str(line);
                assert!(
                    parsed.is_ok(),
                    "Each line should be valid JSON, got: {}",
                    line
                );
            }
        }

        fs::remove_dir_all(&dir).unwrap();
    }

    #[tokio::test]
    async fn test_multiple_batches_same_segment() {
        let dir = std::env::temp_dir().join(format!("disk_sink_multi_{}", Uuid::new_v4()));
        // Large segment size to avoid rotation
        let sink = DiskSink::with_rotation(dir.clone(), u64::MAX, 3600);

        sink.send(make_test_event("batch1")).await.unwrap();
        sink.send(make_test_event("batch2")).await.unwrap();
        sink.send(make_test_event("batch3")).await.unwrap();
        sink.close().await.unwrap();

        let pending = dir.join("pending");
        let files: Vec<_> = fs::read_dir(&pending)
            .unwrap()
            .filter_map(|e| e.ok())
            .filter(|e| e.path().extension().map_or(false, |ext| ext == "jsonl"))
            .collect();

        assert_eq!(files.len(), 1, "All events should be in 1 segment");
        let contents = fs::read_to_string(files[0].path()).unwrap();
        assert_eq!(contents.lines().count(), 3);

        fs::remove_dir_all(&dir).unwrap();
    }

    #[tokio::test]
    async fn test_segment_filename_format() {
        let dir = std::env::temp_dir().join(format!("disk_sink_fname_{}", Uuid::new_v4()));
        let sink = DiskSink::new(dir.clone());

        sink.send(make_test_event("test")).await.unwrap();
        sink.close().await.unwrap();

        let pending = dir.join("pending");
        let files: Vec<_> = fs::read_dir(&pending)
            .unwrap()
            .filter_map(|e| e.ok())
            .filter(|e| e.path().extension().map_or(false, |ext| ext == "jsonl"))
            .collect();

        assert_eq!(files.len(), 1);
        let name = files[0].file_name().to_string_lossy().to_string();
        // Format: {timestamp}_{uuid}.jsonl
        assert!(name.ends_with(".jsonl"));
        let parts: Vec<&str> = name.trim_end_matches(".jsonl").splitn(2, '_').collect();
        assert_eq!(parts.len(), 2, "Filename should be timestamp_uuid.jsonl");
        assert!(
            parts[0].parse::<u64>().is_ok(),
            "First part should be a timestamp"
        );

        fs::remove_dir_all(&dir).unwrap();
    }
}
