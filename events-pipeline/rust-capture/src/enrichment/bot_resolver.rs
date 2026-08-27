use isbot::Bots;
use moka::sync::Cache;
use regex::Regex;
use serde::Deserialize;
use std::fs::File;
use std::io::{self, BufRead};
use std::sync::{Arc, Mutex};

const CATEGORY_NONE: &str = "none";
const CATEGORY_GENERIC: &str = "generic";
const AI_BOT_PATTERNS_PATH: &str = "data/ai_bot_patterns.yaml";

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct BotClassification {
    pub is_bot: bool,
    pub category: String,
    pub provider: String,
    pub name: String,
}

impl BotClassification {
    pub fn missing_user_agent() -> Self {
        Self {
            is_bot: true,
            category: CATEGORY_GENERIC.to_string(),
            provider: String::new(),
            name: String::new(),
        }
    }

    fn human() -> Self {
        Self {
            is_bot: false,
            category: CATEGORY_NONE.to_string(),
            provider: String::new(),
            name: String::new(),
        }
    }

    fn generic_bot() -> Self {
        Self::missing_user_agent()
    }
}

#[derive(Debug, Deserialize)]
struct AiBotPatternDefinition {
    token: String,
    name: String,
    provider: String,
    category: String,
}

#[derive(Debug)]
struct AiBotPattern {
    matcher: Regex,
    name: String,
    provider: String,
    category: String,
}

impl AiBotPattern {
    fn from_definition(definition: AiBotPatternDefinition) -> Self {
        let escaped_token = regex::escape(definition.token.trim());
        let pattern = format!(r"(?i)(?:^|[\s(;]){}(?:$|[/\s;)])", escaped_token);
        let matcher = Regex::new(&pattern).expect("AI bot token must produce a valid regex");
        Self {
            matcher,
            name: definition.name,
            provider: definition.provider,
            category: definition.category,
        }
    }

    fn classify(&self, agent: &str) -> Option<BotClassification> {
        self.matcher.is_match(agent).then(|| BotClassification {
            is_bot: true,
            category: self.category.clone(),
            provider: self.provider.clone(),
            name: self.name.clone(),
        })
    }
}

#[derive(Debug, Clone)]
pub struct BotResolver {
    bot: Arc<Bots>,
    ai_patterns: Arc<Vec<AiBotPattern>>,
    cache: Cache<String, BotClassification>,
    cold_miss: Arc<Mutex<()>>,
}

impl BotResolver {
    pub fn new() -> Self {
        let mut bot = Bots::default();

        let input = File::open("data/bot_patterns.txt").expect("open generic bot patterns");
        let buffered = io::BufReader::new(input);
        for line in buffered.lines() {
            let line = line.expect("read generic bot pattern");
            let regex_str = format!(r"^{}$", line);
            bot.append(&[&regex_str]);
        }

        let ai_definitions: Vec<AiBotPatternDefinition> = serde_yaml::from_reader(
            File::open(AI_BOT_PATTERNS_PATH).expect("open AI bot patterns"),
        )
        .expect("parse AI bot patterns");
        let ai_patterns = ai_definitions
            .into_iter()
            .map(AiBotPattern::from_definition)
            .collect();

        BotResolver {
            bot: Arc::new(bot),
            ai_patterns: Arc::new(ai_patterns),
            cache: Cache::new(30_000),
            cold_miss: Arc::new(Mutex::new(())),
        }
    }

    pub fn classify(&self, agent: &str) -> BotClassification {
        if agent.trim().is_empty() {
            return BotClassification::missing_user_agent();
        }
        if let Some(classification) = self.cache.get(agent) {
            return classification;
        }
        let _cold_miss = self
            .cold_miss
            .lock()
            .unwrap_or_else(|poisoned| poisoned.into_inner());
        if let Some(classification) = self.cache.get(agent) {
            return classification;
        }

        let classification = self
            .ai_patterns
            .iter()
            .find_map(|pattern| pattern.classify(agent))
            .unwrap_or_else(|| {
                if self.bot.is_bot(agent) {
                    BotClassification::generic_bot()
                } else {
                    BotClassification::human()
                }
            });
        self.cache.insert(agent.to_string(), classification.clone());
        classification
    }

    pub fn check_bot(&self, agent: &str) -> bool {
        self.classify(agent).is_bot
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn classifies_generic_bots_and_browsers() {
        let resolver = BotResolver::new();
        assert!(resolver
            .check_bot("Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)"));
        assert!(resolver
            .check_bot("Mozilla/5.0 (compatible; bingbot/2.0; +http://www.bing.com/bingbot.htm)"));
        assert!(!resolver.check_bot("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.36"));
    }

    #[test]
    fn classifies_official_ai_agent_tokens() {
        let resolver = BotResolver::new();
        let cases = [
            ("GPTBot", "openai", "ai_training"),
            ("ClaudeBot", "anthropic", "ai_training"),
            ("OAI-SearchBot", "openai", "ai_search"),
            ("Claude-SearchBot", "anthropic", "ai_search"),
            ("PerplexityBot", "perplexity", "ai_search"),
            ("ChatGPT-User", "openai", "ai_user_fetcher"),
            ("Claude-User", "anthropic", "ai_user_fetcher"),
            ("Perplexity-User", "perplexity", "ai_user_fetcher"),
            ("Google-CloudVertexBot", "google", "ai_agent"),
            ("OAI-AdsBot", "openai", "ai_other"),
            ("Amazonbot", "amazon", "ai_training"),
            ("Amzn-SearchBot", "amazon", "ai_search"),
            ("Amzn-User", "amazon", "ai_user_fetcher"),
        ];

        for (token, provider, category) in cases {
            let user_agent = format!(
                "Mozilla/5.0 (compatible; {}/1.0; +https://example.com/bot)",
                token
            );
            let classification = resolver.classify(&user_agent);
            assert!(classification.is_bot, "{token} was not classified as a bot");
            assert_eq!(classification.name, token);
            assert_eq!(classification.provider, provider);
            assert_eq!(classification.category, category);
        }
    }

    #[test]
    fn ai_agent_matching_is_case_insensitive_and_token_bounded() {
        let resolver = BotResolver::new();
        let classification = resolver.classify("Mozilla/5.0 (compatible; chatgpt-user/1.0)");
        assert_eq!(classification.category, "ai_user_fetcher");
        assert_eq!(classification.name, "ChatGPT-User");

        let classification = resolver.classify("Mozilla/5.0 ClaudeBotanical/1.0");
        assert_eq!(classification.category, "generic");
        assert!(classification.provider.is_empty());
        assert!(classification.name.is_empty());
    }

    #[test]
    fn missing_or_empty_user_agents_are_generic_bots() {
        let missing = BotClassification::missing_user_agent();
        assert!(missing.is_bot);
        assert_eq!(missing.category, "generic");

        let resolver = BotResolver::new();
        assert_eq!(resolver.classify("   "), missing);
    }
}
