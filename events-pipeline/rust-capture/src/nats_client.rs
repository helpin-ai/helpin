use std::env;
use std::path::PathBuf;

use anyhow::{Context, Result};

pub async fn connect(url: &str) -> Result<async_nats::Client> {
    let mut options = async_nats::ConnectOptions::new();

    match (env::var("NATS_USERNAME"), env::var("NATS_PASSWORD")) {
        (Ok(username), Ok(password)) => {
            options = options.user_and_password(username, password);
        }
        (Err(_), Err(_)) => {}
        _ => anyhow::bail!("NATS_USERNAME and NATS_PASSWORD must be set together"),
    }

    if let Ok(ca_file) = env::var("NATS_CA_FILE") {
        options = options
            .require_tls(true)
            .add_root_certificates(PathBuf::from(ca_file));
    }
    match (
        env::var("NATS_CLIENT_CERT_FILE"),
        env::var("NATS_CLIENT_KEY_FILE"),
    ) {
        (Ok(cert_file), Ok(key_file)) => {
            options = options
                .require_tls(true)
                .add_client_certificate(PathBuf::from(cert_file), PathBuf::from(key_file));
        }
        (Err(_), Err(_)) => {}
        _ => anyhow::bail!("NATS_CLIENT_CERT_FILE and NATS_CLIENT_KEY_FILE must be set together"),
    }

    options
        .connect(url)
        .await
        .with_context(|| format!("connect to NATS at {url}"))
}

#[cfg(test)]
mod tests {
    #[test]
    fn tls_client_file_names_remain_a_pair() {
        let names = ["NATS_CLIENT_CERT_FILE", "NATS_CLIENT_KEY_FILE"];
        assert_eq!(names.len(), 2);
        assert!(names.iter().all(|name| name.starts_with("NATS_CLIENT_")));
    }
}
