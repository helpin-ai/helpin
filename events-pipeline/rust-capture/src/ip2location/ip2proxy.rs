use crate::ip2location::resolver::{IP2ProxyResolver, IP2ProxyResult};
use anyhow::{anyhow, Result};

#[derive(Default, Debug)]
pub struct Data {
    pub proxy_type: Option<String>,
    pub country_short: Option<String>,
    pub country_long: Option<String>,
    // pub region: Option<String>,
    // pub city: Option<String>,
    // pub isp: Option<String>,
    // pub domain: Option<String>,
    // pub usage_type: Option<String>,
    // pub asn: Option<String>,
    // pub as_name: Option<String>,
    // pub last_seen: Option<String>,
    // pub threat: Option<String>,
    // pub provider: Option<String>,
}

pub struct IP2ProxyWrapper<'a> {
    ip2proxy_resolver: &'a IP2ProxyResolver,
}

impl<'a> IP2ProxyWrapper<'a> {
    pub fn new(ip2proxy_resolver: &'a IP2ProxyResolver) -> Self {
        Self { ip2proxy_resolver }
    }

    pub fn resolve(&self, ip: &str) -> Result<Data> {
        let result: IP2ProxyResult = self
            .ip2proxy_resolver
            .lookup(ip)?
            .ok_or_else(|| anyhow!("IP not found in the database"))?;

        tracing::debug!("IP2Proxy information: {:?}", result);

        Ok(Data {
            proxy_type: result.proxy_type,
            country_short: result.country_short,
            country_long: result.country_long,
            // region: result.region,
            // city: result.city,
            // isp: result.isp,
            // domain: result.domain,
            // usage_type: result.usage_type,
            // asn: result.asn,
            // as_name: result.as_name,
            // last_seen: result.last_seen,
            // threat: result.threat,
            // provider: result.provider,
        })
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::ip2location::resolver::IP2ProxyResolver;

    #[test]
    fn test_ip2proxy_wrapper() {
        let resolver = IP2ProxyResolver::new("data/IP2PROXY-IP-PROXYTYPE-COUNTRY.BIN").unwrap();
        let wrapper = IP2ProxyWrapper::new(&resolver);

        let result = wrapper.resolve("38.153.15.49").unwrap();
        assert_eq!(result.proxy_type, Some(String::from("VPN")));
        assert_eq!(result.country_short, Some(String::from("GB")));
        assert_eq!(
            result.country_long,
            Some(String::from(
                "United Kingdom of Great Britain and Northern Ireland"
            ))
        );
    }
}
