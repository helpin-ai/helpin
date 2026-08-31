use flate2::read::GzDecoder;
use reqwest::Client;
use std::fs;
use std::io::Cursor;
use std::path::{Path, PathBuf};
use tar::Archive;
use tokio::fs as async_fs;
use walkdir::WalkDir;

const DEFAULT_MAXMIND_URL: &str =
    "https://download.maxmind.com/geoip/databases/GeoLite2-City/download?suffix=tar.gz";
const DEFAULT_TARGET_PATH: &str = "data/GeoLite2-City.mmdb";
const DEFAULT_TEMP_DIR: &str = "temp_data/maxmind";

pub fn target_path() -> String {
    std::env::var("MAXMIND_DB_PATH").unwrap_or_else(|_| DEFAULT_TARGET_PATH.to_string())
}

fn download_url() -> String {
    std::env::var("MAXMIND_DOWNLOAD_URL")
        .or_else(|_| std::env::var("MAXMIND_URL"))
        .unwrap_or_else(|_| DEFAULT_MAXMIND_URL.to_string())
}

fn temp_dir() -> String {
    std::env::var("MAXMIND_TEMP_DIR").unwrap_or_else(|_| DEFAULT_TEMP_DIR.to_string())
}

pub async fn download_and_save() -> Result<(), Box<dyn std::error::Error + Send + Sync>> {
    let account_id = std::env::var("MAXMIND_ACCOUNT_ID")
        .map_err(|_| "MAXMIND_ACCOUNT_ID env var must be set")?;
    let license_key = std::env::var("MAXMIND_LICENSE_KEY")
        .map_err(|_| "MAXMIND_LICENSE_KEY env var must be set")?;
    let target = target_path();
    let temp = temp_dir();
    let temp_target = format!("{target}.temp");

    let client = Client::builder()
        .timeout(std::time::Duration::from_secs(30))
        .build()?;
    let response = client
        .get(download_url())
        .basic_auth(&account_id, Some(&license_key))
        .send()
        .await?;

    if !response.status().is_success() {
        return Err(format!("MaxMind API returned status {}", response.status()).into());
    }

    let bytes = response.bytes().await?;

    // Unpack the tar.gz file.
    let tar = GzDecoder::new(Cursor::new(&bytes));
    let mut archive = Archive::new(tar);

    if Path::new(&temp).exists() {
        fs::remove_dir_all(&temp)?;
    }
    async_fs::create_dir_all(&temp).await?;

    archive.unpack(&temp)?;

    // Walk through the temp directory and find the .mmdb file.
    let db_file = find_db_file(&temp)?;

    // Ensure the target directory exists.
    if let Some(parent) = Path::new(&temp_target).parent() {
        if !parent.as_os_str().is_empty() {
            async_fs::create_dir_all(parent).await?;
        }
    }

    // Copy beside the target and rename there so the final replacement is
    // atomic even when the extraction directory is on another filesystem.
    async_fs::copy(&db_file, &temp_target).await?;

    // Check if the temp target file exists.
    if !Path::new(&temp_target).exists() {
        return Err(Box::from("Failed to create the temporary file."));
    }

    // Atomically replace the old file with the new one.
    fs::rename(&temp_target, &target)?;

    // Delete the temporary directory.
    fs::remove_dir_all(&temp)?;

    tracing::info!("✅ Successfully downloaded and extracted to {}", target);
    Ok(())
}

fn find_db_file(dir: &str) -> Result<PathBuf, Box<dyn std::error::Error + Send + Sync>> {
    for entry in WalkDir::new(dir) {
        let entry = entry?;
        if entry.path().extension() == Some(std::ffi::OsStr::new("mmdb")) {
            return Ok(entry.into_path());
        }
    }

    Err("Could not find .mmdb file in extracted contents".into())
}
#[cfg(test)]
mod tests {
    use super::*;
    use flate2::write::GzEncoder;
    use flate2::Compression;
    use std::io::Write;
    use uuid::Uuid;

    static ENV_LOCK: tokio::sync::Mutex<()> = tokio::sync::Mutex::const_new(());

    fn make_tar_gz(contents: &[u8]) -> Vec<u8> {
        let mut tar_buffer = Vec::new();
        {
            let mut builder = tar::Builder::new(&mut tar_buffer);
            let mut header = tar::Header::new_gnu();
            header.set_size(contents.len() as u64);
            header.set_mode(0o644);
            builder
                .append_data(
                    &mut header,
                    "GeoLite2-City_20260825/GeoLite2-City.mmdb",
                    contents,
                )
                .unwrap();
            builder.finish().unwrap();
        }
        let mut encoder = GzEncoder::new(Vec::new(), Compression::default());
        encoder.write_all(&tar_buffer).unwrap();
        encoder.finish().unwrap()
    }

    #[tokio::test]
    async fn atomically_downloads_maxmind_to_the_configured_path() {
        let _guard = ENV_LOCK.lock().await;
        let bytes = b"test-mmdb-payload";
        let mut server = mockito::Server::new_async().await;
        let mock = server
            .mock("GET", "/maxmind")
            .with_status(200)
            .with_body(make_tar_gz(bytes))
            .create_async()
            .await;
        let root = std::env::temp_dir().join(format!("maxmind-download-{}", Uuid::new_v4()));
        let target = root.join("GeoLite2-City.mmdb");
        let temp = root.join("extract");

        std::env::set_var("MAXMIND_DOWNLOAD_URL", format!("{}/maxmind", server.url()));
        std::env::set_var("MAXMIND_ACCOUNT_ID", "account");
        std::env::set_var("MAXMIND_LICENSE_KEY", "license");
        std::env::set_var("MAXMIND_DB_PATH", &target);
        std::env::set_var("MAXMIND_TEMP_DIR", &temp);
        let result = download_and_save().await;
        for name in [
            "MAXMIND_DOWNLOAD_URL",
            "MAXMIND_ACCOUNT_ID",
            "MAXMIND_LICENSE_KEY",
            "MAXMIND_DB_PATH",
            "MAXMIND_TEMP_DIR",
        ] {
            std::env::remove_var(name);
        }

        assert!(result.is_ok(), "download failed: {:?}", result.err());
        assert_eq!(std::fs::read(&target).unwrap(), bytes);
        assert!(!Path::new(&format!("{}.temp", target.display())).exists());
        mock.assert_async().await;
        std::fs::remove_dir_all(root).ok();
    }
}
