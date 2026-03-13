use std::sync::Arc;
use ip2proxy::{Database, Columns, Row};
use anyhow::Result;
use std::net::IpAddr;

#[derive(Clone, Debug)]
pub struct IP2ProxyResolver {
    db: Arc<Database>,
}

impl IP2ProxyResolver {
    pub fn new(db_path: &str) -> Result<Self> {
        let db = Database::open(db_path)?;
        Ok(Self {
            db: Arc::new(db),
        })
    }

    pub fn lookup(&self, ip: &str) -> Result<Option<IP2ProxyResult>> {
        let ip_addr: IpAddr = ip.parse()?;
        let row = self.db.query(ip_addr, Columns::all())?;
        
        Ok(row.map(IP2ProxyResult::from))
    }

    pub fn package_version(&self) -> u8 {
        self.db.package_version()
    }

    pub fn database_version(&self) -> String {
        self.db.database_version()
    }
}

#[derive(Debug, Clone)]
pub struct IP2ProxyResult {
    pub proxy_type: Option<String>,
    pub country_short: Option<String>,
    pub country_long: Option<String>,
    // PX2 license does not include these fields
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

impl From<Row> for IP2ProxyResult {
    fn from(row: Row) -> Self {
        Self {
            proxy_type: row.proxy_type,
            country_short: row.country_short,
            country_long: row.country_long,
            // PX2 license does not include these fields
            // region: row.region,
            // city: row.city,
            // isp: row.isp,
            // domain: row.domain,
            // usage_type: row.usage_type,
            // asn: row.asn,
            // as_name: row.as_name,
            // last_seen: row.last_seen,
            // threat: row.threat,
            // provider: row.provider,
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_ip2proxy_resolver() {
        let resolver = IP2ProxyResolver::new("data/IP2PROXY-IP-PROXYTYPE-COUNTRY.BIN").unwrap();
        println!("Resolver: {:?}", resolver.lookup("38.153.15.49").unwrap());

        if let Some(result) = resolver.lookup("38.153.15.49").unwrap() {
            assert_eq!(result.proxy_type, Some(String::from("VPN")));
            assert_eq!(result.country_short, Some(String::from("GB")));
            assert_eq!(result.country_long, Some(String::from("United Kingdom of Great Britain and Northern Ireland")));
            
        } else {
            panic!("Expected Some(IP2ProxyResult), got None");
        }
    }
}