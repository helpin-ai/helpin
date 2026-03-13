use crate::geo::resolver::GeoResolver;
use maxminddb::geoip2;
use std::net::IpAddr;

#[derive(Default, Debug)]
pub struct Data {
    pub continent: Option<String>,
    pub country: Option<String>,
    pub country_name: Option<String>,
    pub city: Option<String>,
    pub lat: Option<f64>,
    pub lon: Option<f64>,
    pub zip: Option<String>,
    pub region: Option<String>,
    pub region_name: Option<String>,
    pub is_in_european_union: Option<bool>,
    // NOTE: We do not need for now. Maybe later down the road.
    
    // asn: Option<u32>,
    // aso: Option<String>,
    // isp: Option<String>,
    // organization: Option<String>,
    // domain: Option<String>,
}

pub struct MaxMindResolver<'a> {
    geo_resolver: &'a GeoResolver,
    // other fields
}

impl<'a> MaxMindResolver<'a> {
    pub fn new(geo_resolver: &'a GeoResolver) -> Self {
        Self { geo_resolver }
    }

    pub fn resolve(&self, ip: &str) -> Result<Data, Box<dyn std::error::Error>> {
        let country: geoip2::Country = self.geo_resolver.lookup_country(ip)?;
        let city: geoip2::City = self.geo_resolver.lookup_city(ip)?;
        tracing::debug!(
            "Country information: {:?}, city information: {:?}",
            country,
            city
        );
        let mut data = Data::default();

        if let Some(continent) = country.continent {
            if let Some(names) = continent.names {
                if let Some(name) = names.get("en") {
                    data.continent = Some(name.to_string());
                }
            }
        }

        if let Some(country) = country.country {
            if let Some(iso_code) = country.iso_code {
                data.country = Some(iso_code.to_string());
            }
            if let Some(names) = country.names {
                if let Some(name) = names.get("en") {
                    data.country_name = Some(name.to_string());
                }
            }
            if let Some(is_in_european_union) = country.is_in_european_union {
                data.is_in_european_union = Some(is_in_european_union);
            }
        }

        if let Some(city) = city.city {
            if let Some(names) = city.names {
                if let Some(name) = names.get("en") {
                    data.city = Some(name.to_string());
                }
            }
        }

        if let Some(latitude) = city.location.clone() {
            if let Some(latitude) = latitude.latitude {
                data.lat = Some(latitude);
            }
        }

        if let Some(longitude) = city.location.clone() {
            if let Some(longitude) = longitude.longitude {
                data.lon = Some(longitude);
            }
        }
        if let Some(postal_code) = city.postal {
            if let Some(code) = postal_code.code {
                data.zip = Some(code.to_string());
            }
        }

        if let Some(subdivisions) = &city.subdivisions {
            if let Some(country) = &city.country {
                if country.iso_code == Some("RU") {
                    data.region = Some(format!(
                        "{}-{}",
                        country.iso_code.as_ref().unwrap(),
                        subdivisions[0].iso_code.as_ref().unwrap()
                    ));
                } else {
                    data.region = Some(format!("{}", subdivisions[0].iso_code.as_ref().unwrap()));
                }

                if let Some(names) = &subdivisions[0].names {
                    if let Some(name) = names.get("en") {
                        data.region_name = Some(name.to_string());
                    }
                }
            } else {
                data.region = Some(format!("{}", subdivisions[0].iso_code.as_ref().unwrap()));

                if let Some(names) = &subdivisions[0].names {
                    if let Some(name) = names.get("en") {
                        data.region_name = Some(name.to_string());
                    }
                }
            }
        }

        Ok(data)
    }
}
