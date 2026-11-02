use std::path::PathBuf;
use serde::Deserialize;
use crate::error::{Result, ValtError};

#[derive(Debug, Deserialize, Clone)]
pub struct ValtConfig {
    #[serde(default = "default_api_url")]
    pub api_url: String,

    #[serde(default)]
    pub project_id: Option<String>,
}

fn default_api_url() -> String {
    "http://localhost:8080/api/v1".to_string()
}

impl Default for ValtConfig {
    fn default() -> Self {
        Self { api_url: default_api_url(), project_id: None }
    }
}

pub fn config_path() -> PathBuf {
    let home = dirs::home_dir().unwrap_or_else(|| PathBuf::from("."));
    home.join(".valtag").join("config.toml")
}

pub fn load_config() -> Result<ValtConfig> {
    let path = config_path();
    let mut cfg = if path.exists() {
        let content = std::fs::read_to_string(&path)
            .map_err(|e| ValtError::Config(format!("read config: {e}")))?;
        toml::from_str(&content)
            .map_err(|e| ValtError::Config(format!("parse config: {e}")))?
    } else {
        ValtConfig::default()
    };

    // Env overrides
    if let Ok(url) = std::env::var("VALTAG_API_URL") {
        cfg.api_url = url;
    }
    if let Ok(pid) = std::env::var("VALT_PROJECT_ID") {
        cfg.project_id = Some(pid);
    }

    Ok(cfg)
}
