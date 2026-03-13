use serde::Serialize;
use time::{format_description::well_known::Rfc3339, OffsetDateTime};

pub trait TimeSource {
    // Return an ISO timestamp
    fn current_time(&self) -> String;
}

#[derive(Clone)]
pub struct SystemTime {}

impl TimeSource for SystemTime {
    fn current_time(&self) -> String {
        let time = time::OffsetDateTime::now_utc();

        time.format(&time::format_description::well_known::Iso8601::DEFAULT)
            .expect("failed to iso8601 format timestamp")
    }
}

#[derive(Debug, Serialize)]
pub struct MyTimeError {
    details: String,
}

pub fn get_current_time() -> String {
    let time = OffsetDateTime::now_utc();

    match time.format(&time::format_description::well_known::Iso8601::DEFAULT) {
        Ok(formatted_time) => formatted_time,
        Err(e) => {
            // Handle the error within the function.
            // For example, log the error and return a default string.
            eprintln!("Failed to format timestamp: {}", e);
            String::from("")
        }
    }
}
