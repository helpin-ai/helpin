use reqwest::Client;
use std::path::Path;
use std::{env, fs};
use tokio::fs as async_fs;

const DEFAULT_TARGET_PATH: &str = "data/IP2PROXY-IP-PROXYTYPE-COUNTRY.BIN";

pub fn target_path() -> String {
    env::var("IP2PROXY_DB_PATH").unwrap_or_else(|_| DEFAULT_TARGET_PATH.to_string())
}

pub async fn ip2proxy_download_and_save() -> Result<(), Box<dyn std::error::Error + Send + Sync>> {
    let download_url = env::var("IP2PROXY_DOWNLOADER_URL")
        .map_err(|_| "IP2PROXY_DOWNLOADER_URL env var must be set")?;
    let target = target_path();
    let temp_target = format!("{target}.temp");
    let client = Client::builder()
        .timeout(std::time::Duration::from_secs(30))
        .build()?;
    let response = client.get(download_url).send().await?;
    if !response.status().is_success() {
        return Err(format!("IP2Proxy download returned status {}", response.status()).into());
    }
    let bytes = response.bytes().await?;

    // Ensure the target directory exists.
    if let Some(parent) = Path::new(&temp_target).parent() {
        if !parent.as_os_str().is_empty() {
            async_fs::create_dir_all(parent).await?;
        }
    }

    // Write the downloaded content to the temp file.
    async_fs::write(&temp_target, &bytes).await?;

    // Check if the temp target file exists.
    if !Path::new(&temp_target).exists() {
        return Err(Box::from("Failed to create the temporary file."));
    }

    // Atomically replace the old file with the new one.
    fs::rename(&temp_target, &target)?;

    tracing::info!("✅ Successfully downloaded IP2Proxy database to {}", target);
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;
    use uuid::Uuid;

    static ENV_LOCK: tokio::sync::Mutex<()> = tokio::sync::Mutex::const_new(());

    #[tokio::test]
    async fn atomically_downloads_ip2proxy_to_the_configured_path() {
        let _guard = ENV_LOCK.lock().await;
        let bytes = b"test-ip2proxy-payload";
        let mut server = mockito::Server::new_async().await;
        let mock = server
            .mock("GET", "/ip2proxy")
            .with_status(200)
            .with_body(bytes)
            .create_async()
            .await;
        let root = std::env::temp_dir().join(format!("ip2proxy-download-{}", Uuid::new_v4()));
        let target = root.join("IP2PROXY.BIN");
        env::set_var(
            "IP2PROXY_DOWNLOADER_URL",
            format!("{}/ip2proxy", server.url()),
        );
        env::set_var("IP2PROXY_DB_PATH", &target);
        let result = ip2proxy_download_and_save().await;
        env::remove_var("IP2PROXY_DOWNLOADER_URL");
        env::remove_var("IP2PROXY_DB_PATH");

        assert!(result.is_ok(), "download failed: {:?}", result.err());
        assert_eq!(std::fs::read(&target).unwrap(), bytes);
        assert!(!Path::new(&format!("{}.temp", target.display())).exists());
        mock.assert_async().await;
        std::fs::remove_dir_all(root).ok();
    }
}
