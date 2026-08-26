use std::{net::IpAddr, sync::Arc};

use anyhow::Result;
use maxminddb::geoip2;

#[derive(Clone)]
pub struct GeoResolver {
    reader: Arc<maxminddb::Reader<Vec<u8>>>,
}

impl GeoResolver {
    pub fn new(db_path: &str) -> Result<Self> {
        let reader = maxminddb::Reader::open_readfile(db_path)?;
        tracing::info!("✅ Maxmind db loaded");
        Ok(Self {
            reader: Arc::new(reader),
        })
    }

    pub fn lookup_country(
        &self,
        ip: &str,
    ) -> Result<geoip2::Country<'_>, maxminddb::MaxMindDBError> {
        let ip: IpAddr = ip.parse().unwrap();
        self.reader.lookup(ip)
    }
    pub fn lookup_city(&self, ip: &str) -> Result<geoip2::City<'_>, maxminddb::MaxMindDBError> {
        let ip: IpAddr = ip.parse().unwrap();
        self.reader.lookup(ip)
    }

    pub fn database_build_epoch(&self) -> u64 {
        self.reader.metadata.build_epoch
    }
}

#[cfg(test)]
mod tests {
    use crate::geo::maxmind::MaxMindResolver;

    use super::*;

    #[test]
    #[ignore = "requires a licensed MaxMind database fixture"]
    fn test_geo_resolver() {
        // Create a GeoResolver with the test database file

        // Create a GeoResolver with the in-memory database
        let geo_resolver = GeoResolver::new("data/GeoLite2-City.mmdb").unwrap();
        let maxmind_resolver = MaxMindResolver::new(&geo_resolver);
        // Test IP addresses
        let ip_in_eu = "157.90.23.154"; // IP address in the European Union
        let ip_not_in_eu = "95.10.185.235"; // IP address not in the European Union
                                            // Lookup IP in the database
        let country_in_eu = maxmind_resolver.resolve(ip_in_eu).unwrap();
        let country_not_in_eu = maxmind_resolver.resolve(ip_not_in_eu).unwrap();
        println!("{:?}", country_in_eu);
        println!("{:?}", country_not_in_eu);

        // Print region_name if available
        if let Some(region_name) = &country_in_eu.region_name {
            println!("Region name for IP in EU: {}", region_name);
        } else {
            println!("Region name not available for IP in EU");
        }

        if let Some(region_name) = &country_not_in_eu.region_name {
            println!("Region name for IP not in EU: {}", region_name);
        } else {
            println!("Region name not available for IP not in EU");
        }
        // Check if the countries are correctly resolved
        // assert_eq!(country_in_eu.country.unwrap().is_in_european_union.unwrap(), true);
        // assert_eq!(country_not_in_eu.country.unwrap().is_in_european_union.unwrap(), false);
    }

    #[test]
    #[ignore = "requires a licensed MaxMind database fixture"]
    fn test_ipv6_geo_resolver() {
        // Create a GeoResolver with the test database file

        // Create a GeoResolver with the in-memory database
        let geo_resolver = GeoResolver::new("data/GeoLite2-City.mmdb").unwrap();
        let maxmind_resolver = MaxMindResolver::new(&geo_resolver);
        // Test IP addresses
        let ip = "2600:4040:4520:c300:4c59:5c9e:80ba:ab49"; // IP address in the European Union

        let response = maxmind_resolver.resolve(ip).unwrap();
        println!("{:?}", response);
    }
}
