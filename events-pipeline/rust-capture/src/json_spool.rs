use std::path::{Path, PathBuf};
use std::sync::Arc;

use anyhow::{anyhow, Context, Result};
use serde::{de::DeserializeOwned, Serialize};
use tokio::io::AsyncWriteExt;
use tokio::sync::Mutex;
use uuid::Uuid;

/// A deliberately small file-per-record spool for rare secondary-path
/// failures. Each record is fsynced and atomically renamed before success is
/// returned. Capacity is independent for each spool instance.
#[derive(Clone)]
pub struct JsonSpool {
    pending_dir: PathBuf,
    max_bytes: u64,
    lock: Arc<Mutex<()>>,
}

impl JsonSpool {
    pub fn new(base_dir: PathBuf, max_bytes: u64) -> Result<Self> {
        anyhow::ensure!(max_bytes > 0, "spool max bytes must be positive");
        let pending_dir = base_dir.join("pending");
        std::fs::create_dir_all(&pending_dir)
            .with_context(|| format!("create spool directory {pending_dir:?}"))?;
        for entry in std::fs::read_dir(&pending_dir)
            .with_context(|| format!("read spool directory {pending_dir:?}"))?
            .filter_map(|entry| entry.ok())
        {
            let path = entry.path();
            if path.extension().and_then(|extension| extension.to_str()) == Some("tmp") {
                let _ = std::fs::remove_file(path);
            }
        }
        Ok(Self {
            pending_dir,
            max_bytes,
            lock: Arc::new(Mutex::new(())),
        })
    }

    pub async fn append<T: Serialize>(&self, value: &T) -> Result<()> {
        let payload = serde_json::to_vec(value).context("serialize spool record")?;
        let _guard = self.lock.lock().await;
        let used = directory_bytes(&self.pending_dir)?;
        anyhow::ensure!(
            used.saturating_add(payload.len() as u64) <= self.max_bytes,
            "spool capacity exceeded ({used} + {} > {})",
            payload.len(),
            self.max_bytes
        );

        let id = Uuid::new_v4();
        let final_path = self.pending_dir.join(format!("{id}.json"));
        let temporary_path = self.pending_dir.join(format!("{id}.json.tmp"));
        let mut file = tokio::fs::OpenOptions::new()
            .create_new(true)
            .write(true)
            .open(&temporary_path)
            .await
            .with_context(|| format!("create spool record {temporary_path:?}"))?;
        file.write_all(&payload)
            .await
            .context("write spool record")?;
        file.sync_all().await.context("fsync spool record")?;
        drop(file);
        tokio::fs::rename(&temporary_path, &final_path)
            .await
            .context("finalize spool record")?;
        std::fs::File::open(&self.pending_dir)
            .context("open spool directory for fsync")?
            .sync_all()
            .context("fsync spool directory")?;
        Ok(())
    }

    pub async fn pending(&self) -> Result<Vec<PathBuf>> {
        let _guard = self.lock.lock().await;
        let mut entries = std::fs::read_dir(&self.pending_dir)
            .with_context(|| format!("read spool directory {:?}", self.pending_dir))?
            .filter_map(|entry| entry.ok())
            .map(|entry| entry.path())
            .filter(|path| {
                path.extension().and_then(|extension| extension.to_str()) == Some("json")
            })
            .collect::<Vec<_>>();
        entries.sort();
        Ok(entries)
    }

    pub async fn read<T: DeserializeOwned>(&self, path: &Path) -> Result<T> {
        let payload = tokio::fs::read(path)
            .await
            .with_context(|| format!("read spool record {path:?}"))?;
        serde_json::from_slice(&payload).context("decode spool record")
    }

    pub async fn remove(&self, path: &Path) -> Result<()> {
        let _guard = self.lock.lock().await;
        tokio::fs::remove_file(path)
            .await
            .with_context(|| format!("remove spool record {path:?}"))
    }

    #[cfg(test)]
    pub fn directory(&self) -> &Path {
        &self.pending_dir
    }
}

fn directory_bytes(directory: &Path) -> Result<u64> {
    std::fs::read_dir(directory)
        .with_context(|| format!("read spool directory {directory:?}"))?
        .filter_map(|entry| entry.ok())
        .try_fold(0_u64, |total, entry| {
            let metadata = entry
                .metadata()
                .with_context(|| format!("stat spool record {:?}", entry.path()))?;
            total
                .checked_add(metadata.len())
                .ok_or_else(|| anyhow!("spool byte count overflow"))
        })
}

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn fsyncs_records_and_enforces_an_independent_capacity() {
        let root = std::env::temp_dir().join(format!("json-spool-{}", Uuid::new_v4()));
        let spool = JsonSpool::new(root.clone(), 32).unwrap();
        spool.append(&serde_json::json!({"id": 1})).await.unwrap();
        let pending = spool.pending().await.unwrap();
        assert_eq!(pending.len(), 1);
        assert_eq!(
            spool.read::<serde_json::Value>(&pending[0]).await.unwrap(),
            serde_json::json!({"id": 1})
        );
        assert!(spool.append(&"x".repeat(64)).await.is_err());
        spool.remove(&pending[0]).await.unwrap();
        assert!(spool.pending().await.unwrap().is_empty());
        std::fs::remove_dir_all(root).unwrap();
    }
}
