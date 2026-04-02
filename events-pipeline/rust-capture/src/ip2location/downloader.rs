use reqwest::Client;
use std::path::Path;
use std::{env, fs};
use tokio::fs as async_fs;

const TARGET_PATH: &str = "data/IP2PROXY-IP-PROXYTYPE-COUNTRY.BIN";
const TEMP_TARGET_PATH: &str = "data/IP2PROXY-IP-PROXYTYPE-COUNTRY.BIN.temp";

pub async fn ip2proxy_download_and_save() -> Result<(), Box<dyn std::error::Error>> {
    let IP2PROXY_URL =
        env::var("IP2PROXY_DOWNLOADER_URL").expect("IP2PROXY_DOWNLOADER_URL must be set"); //"http://ip2proxy-downloader-svc.eventpipeline.svc/download";
    let client = Client::new();
    let response = client.get(IP2PROXY_URL).send().await?;
    let bytes = response.bytes().await?;

    // Ensure the target directory exists.
    if let Some(parent) = Path::new(TEMP_TARGET_PATH).parent() {
        async_fs::create_dir_all(parent).await?;
    }

    // Write the downloaded content to the temp file.
    async_fs::write(TEMP_TARGET_PATH, &bytes).await?;

    // Check if the temp target file exists.
    if !Path::new(TEMP_TARGET_PATH).exists() {
        return Err(Box::from("Failed to create the temporary file."));
    }

    // Atomically replace the old file with the new one.
    fs::rename(TEMP_TARGET_PATH, TARGET_PATH)?;

    tracing::info!(
        "✅ Successfully downloaded IP2Proxy database to {}",
        TARGET_PATH
    );
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::fs;
    use std::path::Path;

    #[tokio::test]
    async fn test_ip2proxy_download_and_save() {
        ip2proxy_download_and_save().await.unwrap();
        // Check that the file exists.
        assert!(Path::new(TARGET_PATH).exists());
        // Clean up after test.
        fs::remove_file(TARGET_PATH).unwrap();
    }
}
