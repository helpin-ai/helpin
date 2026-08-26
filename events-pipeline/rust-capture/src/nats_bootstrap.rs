use std::env;

use anyhow::{Context, Result};
use async_nats::jetstream::stream::Compression;
use events_pipeline::pipeline::VISITOR_SHARD_COUNT;
use events_pipeline::writer::{
    dlq_stream_config, enriched_work_stream_config, raw_archive_stream_config,
    shard_consumer_config,
};

fn required_positive_bytes(name: &str) -> Result<i64> {
    let value = env::var(name).with_context(|| format!("{name} must be set"))?;
    let parsed = value
        .parse::<i64>()
        .with_context(|| format!("{name} must be an integer byte count"))?;
    anyhow::ensure!(parsed > 0, "{name} must be positive");
    Ok(parsed)
}

fn work_compression(value: Option<&str>) -> Result<Compression> {
    match value.unwrap_or("s2").to_ascii_lowercase().as_str() {
        "s2" => Ok(Compression::S2),
        "none" => Ok(Compression::None),
        other => anyhow::bail!("EVENTS_WORK_COMPRESSION must be s2 or none, got {other}"),
    }
}

#[tokio::main]
async fn main() -> Result<()> {
    let nats_url = env::var("NATS_URL").context("NATS_URL must be set")?;
    let client = events_pipeline::nats_client::connect(&nats_url).await?;
    let jetstream = async_nats::jetstream::new(client);
    let mut work = enriched_work_stream_config(required_positive_bytes("EVENTS_WORK_MAX_BYTES")?);
    work.compression = Some(work_compression(
        env::var("EVENTS_WORK_COMPRESSION").ok().as_deref(),
    )?);
    let configs = [
        work,
        raw_archive_stream_config(required_positive_bytes("EVENTS_RAW_MAX_BYTES")?),
        dlq_stream_config(required_positive_bytes("EVENTS_DLQ_MAX_BYTES")?),
    ];

    for config in configs {
        let name = config.name.clone();
        jetstream
            .get_or_create_stream(config.clone())
            .await
            .with_context(|| format!("create stream {name}"))?;
        jetstream
            .update_stream(config)
            .await
            .with_context(|| format!("reconcile stream {name}"))?;
        tracing::info!(%name, "JetStream configuration reconciled");
    }

    let stream = jetstream
        .get_stream("EVENTS_ENRICHED_V1")
        .await
        .context("get work stream for consumer reconciliation")?;
    for shard in 0..VISITOR_SHARD_COUNT as u8 {
        let config = shard_consumer_config(shard);
        let durable_name = format!("events-writer-v1-{shard:03}");
        stream
            .get_or_create_consumer(&durable_name, config.clone())
            .await
            .with_context(|| format!("create consumer {durable_name}"))?;
        stream
            .update_consumer(config)
            .await
            .with_context(|| format!("reconcile consumer {durable_name}"))?;
    }

    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn work_compression_supports_an_explicit_ab_switch() {
        assert_eq!(work_compression(None).unwrap(), Compression::S2);
        assert_eq!(work_compression(Some("s2")).unwrap(), Compression::S2);
        assert_eq!(work_compression(Some("none")).unwrap(), Compression::None);
        assert!(work_compression(Some("gzip")).is_err());
    }
}
