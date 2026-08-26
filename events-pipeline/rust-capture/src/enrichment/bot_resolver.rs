use isbot::Bots;
use moka::sync::Cache;
use regex::Regex;
use std::fs::File;
use std::io::{self, BufRead};
use std::path::Path;
use std::sync::Arc;

#[derive(Debug, Clone)]
pub struct BotResolver {
    bot: Arc<Bots>,
    cache: Cache<String, bool>,
}
impl BotResolver {
    pub fn new() -> Self {
        // Create a new IsBot instance
        let mut bot = Bots::default();

        // Open the file
        let input = File::open("data/bot_patterns.txt").unwrap();

        // Use a BufReader to read the file line by line
        let buffered = io::BufReader::new(input);

        // Read the bot patterns from the file and append to the IsBot instance
        for line in buffered.lines() {
            let line = line.unwrap();
            // Form a valid regular expression string
            let regex_str = format!(r"^{}$", line);
            bot.append(&[&regex_str]);
        }

        BotResolver {
            bot: Arc::new(bot),
            cache: Cache::new(30_000),
        }
    }

    pub fn check_bot(&self, agent: &str) -> bool {
        if let Some(is_bot) = self.cache.get(agent) {
            return is_bot;
        }
        let is_bot = self.bot.is_bot(agent);
        self.cache.insert(agent.to_string(), is_bot);
        is_bot
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_bot_check() {
        // Create a new instance of BotResolver
        let bot_resolver = BotResolver::new();

        // Check a few user agent strings

        // Googlebot
        assert_eq!(
            bot_resolver.check_bot(
                "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)"
            ),
            true
        );

        // Bingbot
        assert_eq!(
            bot_resolver.check_bot(
                "Mozilla/5.0 (compatible; bingbot/2.0; +http://www.bing.com/bingbot.htm)"
            ),
            true
        );

        // Regular browser
        assert_eq!(bot_resolver.check_bot("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.36"), false);
    }
}
