use moka::sync::Cache;
use std::fs::File;
use std::io::{self, BufRead};
use std::path::Path;
use std::sync::Arc;
use uaparser::{Parser, UserAgentParser};

#[derive(Debug, Default, Clone)]
pub struct ResolvedUa {
    pub ua_family: Option<String>,
    pub ua_version: Option<String>,
    pub os_family: Option<String>,
    pub os_version: Option<String>,
    pub device_family: Option<String>,
    pub device_brand: Option<String>,
    pub device_model: Option<String>,
    pub bot: bool,
}

impl ResolvedUa {
    fn is_empty(&self) -> bool {
        self.ua_family.is_none()
            && self.ua_version.is_none()
            && self.os_family.is_none()
            && self.os_version.is_none()
            && self.device_family.is_none()
            && self.device_brand.is_none()
            && self.device_model.is_none()
    }
}
pub struct UserAgentString {
    major: Option<String>,
    minor: Option<String>,
    patch: Option<String>,
}
impl From<&uaparser::UserAgent<'_>> for UserAgentString {
    fn from(agent: &uaparser::UserAgent<'_>) -> Self {
        UserAgentString {
            major: agent.major.as_ref().map(|s| s.to_string()),
            minor: agent.minor.as_ref().map(|s| s.to_string()),
            patch: agent.patch.as_ref().map(|s| s.to_string()),
        }
    }
}

impl UserAgentString {
    pub fn to_version_string(&self) -> Option<String> {
        let mut version = String::new();

        if let Some(ref major) = self.major {
            version.push_str(major);
        }
        if let Some(ref minor) = self.minor {
            if !version.is_empty() {
                version.push('.');
            }
            version.push_str(minor);
        }
        if let Some(ref patch) = self.patch {
            if !version.is_empty() {
                version.push('.');
            }
            version.push_str(patch);
        }

        if version.is_empty() {
            None
        } else {
            Some(version)
        }
    }
}

pub struct OsString {
    family: Option<String>,
    major: Option<String>,
    minor: Option<String>,
    patch: Option<String>,
    patch_minor: Option<String>,
}

impl From<&uaparser::OS<'_>> for OsString {
    fn from(os: &uaparser::OS<'_>) -> Self {
        OsString {
            family: Some(os.family.to_owned().into_owned()),
            major: os.major.as_ref().map(|s| s.to_owned().into_owned()),
            minor: os.minor.as_ref().map(|s| s.to_owned().into_owned()),
            patch: os.patch.as_ref().map(|s| s.to_owned().into_owned()),
            patch_minor: os.patch_minor.as_ref().map(|s| s.to_owned().into_owned()),
        }
    }
}

impl OsString {
    pub fn to_version_string(&self) -> Option<String> {
        let mut version = String::new();

        if let Some(ref major) = self.major {
            version.push_str(major);
        }
        if let Some(ref minor) = self.minor {
            if !version.is_empty() {
                version.push('.');
            }
            version.push_str(minor);
        }
        if let Some(ref patch) = self.patch {
            if !version.is_empty() {
                version.push('.');
            }
            version.push_str(patch);
        }
        if let Some(ref patch_minor) = self.patch_minor {
            if !version.is_empty() {
                version.push('.');
            }
            version.push_str(patch_minor);
        }

        Some(version)
    }
}

#[derive(Debug, Clone)]
pub struct UaResolver {
    parser: Arc<UserAgentParser>,
    cache: Cache<String, ResolvedUa>,
}

impl UaResolver {
    pub fn new() -> Self {
        tracing::info!("Loading UAP parser");
        // let ua_regexes = include_bytes!("../../data/uap-regexes.yaml");

        let parser = UserAgentParser::from_yaml("data/uap-regexes.yaml").expect(
            "Could not create UserAgent. \
                 You are probably using a bad build of regexes ",
        );
        let cache = Cache::new(30000); // A cache with capacity 1024.

        UaResolver {
            parser: Arc::new(parser),
            cache,
        }
    }

    pub fn seed_to_lru_cache(&self) -> io::Result<()> {
        tracing::info!("🔄 Seeding user agents from file to LRU cache started");
        let path = Path::new("data/user_agents_seed.txt");
        let file = File::open(&path)?;
        let reader = io::BufReader::new(file);

        for line in reader.lines() {
            let ua = line?;
            self.resolve(&ua);
        }
        tracing::info!("✅ Seeding user agents from file to LRU cache completed");
        Ok(())
    }

    pub fn resolve(&self, ua: &str) -> Option<ResolvedUa> {
        // Start the timer
        // let start = std::time::Instant::now();
        // First, check if the result is already cached.
        // First, check if the result is already cached.
        if let Some(resolved_ua) = self.cache.get(ua) {
            // If it is, return the cached result immediately,
            // without needing to parse the UA string again.
            return Some(resolved_ua);
        }

        if ua.is_empty() {
            return Some(ResolvedUa::default());
        }
        let mut resolved = ResolvedUa::default();
        let user_agent = self.parser.as_ref().parse_user_agent(ua);
        let device = self.parser.as_ref().parse_device(ua);
        let os = self.parser.as_ref().parse_os(ua);

        // conversion from `&UserAgent` to `UserAgentString`
        let ua_string = UserAgentString::from(&user_agent);
        // now you can use the `to_version_string` method
        resolved.ua_version = ua_string.to_version_string();

        let agent_name = user_agent.family.into_owned();
        // continue your processing with agent_name
        if agent_name.to_lowercase() != "other" {
            resolved.ua_family = Some(agent_name);
        }

        let os_name = os.family.to_owned().into_owned();
        if os_name.to_lowercase() != "other" {
            resolved.os_family = Some(os_name);
        }

        // conversion from `&Os` to `OsString`
        let os_string = OsString::from(&os);
        // now you can use the `to_version_string` method
        resolved.os_version = os_string.to_version_string();

        resolved.device_brand = device.brand.map(|s| s.into_owned());
        resolved.device_family =
            Some(device.family.into_owned()).filter(|family| family.to_lowercase() != "other");
        resolved.device_model = device.model.map(|s| s.into_owned());

        if resolved.is_empty() {
            return Some(ResolvedUa::default());
        }
        // After parsing the UA string, store the result in the cache
        // so that we can just look it up next time.
        self.cache.insert(ua.to_string(), resolved.clone());
        Some(resolved)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_ua_resolver() {
        let ua_resolver = UaResolver::new();

        // Testing with a known User Agent
        let ua = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537";
        let resolved_ua = ua_resolver.resolve(ua).unwrap();

        assert_eq!(resolved_ua.ua_family, Some("Chrome".to_string()));
        assert_eq!(resolved_ua.ua_version, Some("58.0.3029".to_string()));
        assert_eq!(resolved_ua.os_family, Some("Windows".to_string()));
        assert_eq!(resolved_ua.os_version, Some("10".to_string()));
        println!("{:?}", resolved_ua);
        // assert!(resolved_ua.device_family.is_none());
        // assert!(resolved_ua.device_brand.is_none());
        // assert!(resolved_ua.device_model.is_none());
        // assert!(!resolved_ua.bot);

        // Testing with an unknown User Agent
        let ua = "unknown";
        let resolved_ua = ua_resolver.resolve(ua).unwrap();
        println!("{:?}", resolved_ua);

        assert_eq!(resolved_ua.ua_version, None);
        assert!(resolved_ua.os_family.is_none());
        assert_eq!(resolved_ua.os_version, Some("".to_string()));
        assert!(resolved_ua.device_family.is_none());
        assert!(resolved_ua.device_brand.is_none());
        assert!(resolved_ua.device_model.is_none());
        assert_eq!(resolved_ua.bot, false);
    }

    #[test]

    fn test_non_resolved_ua() {
        let ua_resolver = UaResolver::new();
        let resolved_ua = ua_resolver.resolve("").unwrap();
    }
}
