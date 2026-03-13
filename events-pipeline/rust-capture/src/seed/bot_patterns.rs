use regex::Regex;
use serde::{Deserialize, Serialize};
use std::fs::File;
use std::io::Write;
use std::path::Path;

#[derive(Debug, PartialEq, Serialize, Deserialize)]
struct Bot {
    regex: String,
}

fn main() {
    let path = Path::new("data/bots.yaml");
    let bots: Vec<Bot> = serde_yaml::from_reader(File::open(&path).expect("Unable to open file"))
        .expect("Unable to parse YAML");

    let mut regex_patterns = Vec::new();

    for bot in bots {
        match Regex::new(&bot.regex) {
            Ok(_) => {
                regex_patterns.push(bot.regex);
            }

            Err(e) => {
                println!("'{}' is not a valid regex. Error: {}", bot.regex, e);
            }
        }
    }

    let mut output_file = File::create("data/bot_patterns.txt").expect("Unable to create file");

    for pattern in &regex_patterns {
        writeln!(output_file, "{}", pattern).expect("Unable to write to file");
    }
}
