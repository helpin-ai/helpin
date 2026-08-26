use std::collections::{BTreeSet, HashMap, HashSet};
use std::env;

use anyhow::{Context, Result};
use async_nats::jetstream::consumer;
use async_nats::jetstream::stream::Compression;
use events_pipeline::writer::{
    dlq_stream_config, enriched_work_stream_config, raw_archive_stream_config,
    seed_floor_from_metadata, shard_from_subject, writer_consumer_config, writer_consumer_name,
    LEGACY_SHARD_CONSUMER_PREFIX, WRITER_CONSUMER_PREFIX,
};
use futures::TryStreamExt;

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

fn bool_from_env(name: &str, default: bool) -> Result<bool> {
    let Ok(value) = env::var(name) else {
        return Ok(default);
    };
    parse_bool(name, &value)
}

fn positive_usize_from_env(name: &str, default: usize) -> Result<usize> {
    let value = env::var(name)
        .map(|value| {
            value
                .parse::<usize>()
                .with_context(|| format!("parse {name}"))
        })
        .unwrap_or(Ok(default))?;
    anyhow::ensure!(value > 0, "{name} must be positive");
    Ok(value)
}

fn legacy_consumer_name(name: &str) -> bool {
    name.strip_prefix(LEGACY_SHARD_CONSUMER_PREFIX)
        .is_some_and(|suffix| suffix.len() == 3 && suffix.bytes().all(|byte| byte.is_ascii_digit()))
}

fn topology_token(name: &str) -> Option<String> {
    if legacy_consumer_name(name) {
        return Some("legacy".to_string());
    }
    let suffix = name.strip_prefix(WRITER_CONSUMER_PREFIX)?;
    let (replicas, ordinal) = suffix.split_once("-o")?;
    let replicas = replicas.strip_prefix('r')?;
    if replicas.len() == 3
        && ordinal.len() == 3
        && replicas.bytes().all(|byte| byte.is_ascii_digit())
        && ordinal.bytes().all(|byte| byte.is_ascii_digit())
    {
        Some(format!("r{replicas}"))
    } else {
        None
    }
}

fn managed_consumer(name: &str) -> bool {
    topology_token(name).is_some()
}

fn config_shards(filter_subject: &str, filter_subjects: &[String]) -> BTreeSet<u8> {
    std::iter::once(filter_subject)
        .chain(filter_subjects.iter().map(String::as_str))
        .filter_map(shard_from_subject)
        .collect()
}

fn current_topology(infos: &[consumer::Info]) -> String {
    infos
        .iter()
        .filter_map(|info| topology_token(&info.name))
        .collect::<BTreeSet<_>>()
        .into_iter()
        .collect::<Vec<_>>()
        .join(",")
}

fn capture_seed_floors(infos: &[consumer::Info]) -> HashMap<u8, u64> {
    let mut floors = HashMap::<u8, u64>::new();
    for info in infos {
        let shared_floor = info.ack_floor.stream_sequence;
        for shard in config_shards(&info.config.filter_subject, &info.config.filter_subjects) {
            let floor = shared_floor.max(seed_floor_from_metadata(&info.config.metadata, shard));
            floors
                .entry(shard)
                .and_modify(|current| *current = (*current).max(floor))
                .or_insert(floor);
        }
    }
    floors
}

fn parse_bool(name: &str, value: &str) -> Result<bool> {
    match value.to_ascii_lowercase().as_str() {
        "true" | "1" | "yes" | "on" => Ok(true),
        "false" | "0" | "no" | "off" => Ok(false),
        _ => anyhow::bail!("{name} must be true or false, got {value}"),
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

    if bool_from_env("EVENTS_CONSUMERS_ENABLED", true)? {
        let writer_replicas = positive_usize_from_env("WRITER_REPLICAS", 4)?;
        let memory_storage = bool_from_env("EVENTS_CONSUMER_MEMORY_STORAGE", false)?;
        let stream = jetstream
            .get_stream("EVENTS_ENRICHED_V1")
            .await
            .context("get work stream for consumer reconciliation")?;

        let mut listed = stream.consumers();
        let mut existing = Vec::new();
        while let Some(info) = listed
            .try_next()
            .await
            .context("list existing work consumers")?
        {
            if managed_consumer(&info.name) {
                existing.push(info);
            }
        }
        let seed_floors = capture_seed_floors(&existing);
        let mut desired = HashMap::new();
        for ordinal in 0..writer_replicas {
            let config =
                writer_consumer_config(writer_replicas, ordinal, memory_storage, &seed_floors)?;
            desired.insert(writer_consumer_name(writer_replicas, ordinal), config);
        }

        let desired_names = desired.keys().cloned().collect::<HashSet<_>>();
        let requires_rebalance = existing.iter().any(|info| {
            !desired_names.contains(&info.name)
                || desired.get(&info.name).is_some_and(|config| {
                    config_shards(&config.filter_subject, &config.filter_subjects)
                        != config_shards(&info.config.filter_subject, &info.config.filter_subjects)
                })
        });
        if requires_rebalance {
            let current = current_topology(&existing);
            let expected = env::var("EVENTS_CONSUMER_REBALANCE_FROM").unwrap_or_default();
            anyhow::ensure!(
                !current.is_empty() && expected == current,
                "consumer topology rebalance from {current:?} to r{writer_replicas:03} requires stopped writers and EVENTS_CONSUMER_REBALANCE_FROM={current}"
            );
            for info in &existing {
                stream
                    .delete_consumer(&info.name)
                    .await
                    .with_context(|| format!("delete old consumer {}", info.name))?;
            }
            tracing::warn!(
                from = %current,
                to = %format!("r{writer_replicas:03}"),
                consumers = existing.len(),
                "writer consumer topology rebalanced"
            );
        }

        for (durable_name, config) in desired {
            stream
                .get_or_create_consumer(&durable_name, config.clone())
                .await
                .with_context(|| format!("create consumer {durable_name}"))?;
            stream
                .update_consumer(config)
                .await
                .with_context(|| format!("reconcile consumer {durable_name}"))?;
        }
        tracing::info!(
            memory_storage,
            writer_replicas,
            "one durable work consumer per writer reconciled"
        );
    } else {
        tracing::info!("writer consumer reconciliation disabled for capture isolation run");
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

    #[test]
    fn boolean_switches_are_strict() {
        assert!(parse_bool("TEST_SWITCH", "true").unwrap());
        assert!(!parse_bool("TEST_SWITCH", "off").unwrap());
        assert!(parse_bool("TEST_SWITCH", "maybe").is_err());
    }

    #[test]
    fn topology_names_are_strict_and_versioned() {
        assert_eq!(
            topology_token("events-writer-v1-007").as_deref(),
            Some("legacy")
        );
        assert_eq!(
            topology_token("events-writer-v2-r003-o002").as_deref(),
            Some("r003")
        );
        assert!(topology_token("events-writer-v2-r3-o2").is_none());
        assert!(topology_token("unmanaged").is_none());
    }
}
