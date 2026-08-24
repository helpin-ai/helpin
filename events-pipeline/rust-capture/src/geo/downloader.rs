use flate2::read::GzDecoder;
use reqwest::Client;
use std::fs::{self, File};
use std::io::{Cursor, Read};
use std::path::{Path, PathBuf};
use tar::Archive;
use tokio::fs as async_fs;
use walkdir::WalkDir;

const DEFAULT_MAXMIND_URL: &str =
    "https://download.maxmind.com/geoip/databases/GeoLite2-City/download?suffix=tar.gz";
const DEFAULT_TARGET_PATH: &str = "data/GeoLite2-City.mmdb";
const TEMP_DIR: &str = "temp_data/";
const TEMP_TARGET_PATH: &str = "data/GeoLite2-City.mmdb.temp";

pub fn target_path() -> String {
    std::env::var("MAXMIND_DB_PATH").unwrap_or_else(|_| DEFAULT_TARGET_PATH.to_string())
}

pub async fn download_and_save() -> Result<(), Box<dyn std::error::Error>> {
    let account_id = std::env::var("MAXMIND_ACCOUNT_ID")
        .map_err(|_| "MAXMIND_ACCOUNT_ID env var must be set")?;
    let license_key = std::env::var("MAXMIND_LICENSE_KEY")
        .map_err(|_| "MAXMIND_LICENSE_KEY env var must be set")?;
    let url =
        std::env::var("MAXMIND_DOWNLOAD_URL").unwrap_or_else(|_| DEFAULT_MAXMIND_URL.to_string());
    let target = target_path();

    let client = Client::new();
    let response = client
        .get(&url)
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

    // Ensure the temp directory exists.
    if let Some(parent) = Path::new(TEMP_DIR).parent() {
        async_fs::create_dir_all(parent).await?;
    }

    // Extract the tar files to temp directory.
    archive.unpack(TEMP_DIR)?;

    // Walk through the temp directory and find the .mmdb file.
    let db_file = find_db_file(TEMP_DIR)?;

    // Ensure the target directory exists.
    if let Some(parent) = Path::new(TEMP_TARGET_PATH).parent() {
        async_fs::create_dir_all(parent).await?;
    }

    // Move the .mmdb file to the temp target path.
    fs::rename(&db_file, TEMP_TARGET_PATH)?;

    // Check if the temp target file exists.
    if !Path::new(TEMP_TARGET_PATH).exists() {
        return Err(Box::from("Failed to create the temporary file."));
    }

    // Atomically replace the old file with the new one.
    fs::rename(TEMP_TARGET_PATH, &target)?;

    // Delete the temporary directory.
    fs::remove_dir_all(TEMP_DIR)?;

    tracing::info!("✅ Successfully downloaded and extracted to {}", target);
    Ok(())
}

fn find_db_file(dir: &str) -> Result<PathBuf, Box<dyn std::error::Error>> {
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
    use std::fs;

    #[tokio::test]
    #[ignore = "requires MaxMind credentials and network access"]
    async fn test_download_and_save() {
        download_and_save().await.unwrap();

        // Check that the file exists.
        // assert!(Path::new(FILE_PATH).exists());

        // Clean up after test.
        // fs::remove_file(FILE_PATH).unwrap();
    }
}
