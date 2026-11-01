use std::path::{Path, PathBuf};
use std::sync::{Arc, RwLock};

/// Shared, mutable list of exe paths the user has marked as tunneled.
/// Proc-watcher reloads this from disk on its tick; the divert hot path
/// reads it on every PID-miss to decide whether to admit synchronously.
pub type TunneledPaths = Arc<RwLock<TunneledPathsInner>>;

fn excluded_exe(candidate: &Path, exclude: &[String]) -> bool {
    let Some(name) = candidate.file_name().and_then(|n| n.to_str()) else {
        return false;
    };
    let lower = name.to_lowercase();
    exclude.iter().any(|e| e.to_lowercase() == lower)
}

/// True when `candidate` lives under any configured prefix (Discord install dirs).
pub fn matches(paths: &TunneledPaths, candidate: &Path) -> bool {
    let guard = paths.read().unwrap();
    if guard.prefixes.is_empty() {
        return false;
    }
    if excluded_exe(candidate, &guard.exclude) {
        return false;
    }
    let candidate_lower = candidate.to_string_lossy().to_lowercase();
    guard
        .prefixes
        .iter()
        .any(|p| candidate_lower.starts_with(&p.to_string_lossy().to_lowercase()))
}

#[derive(Clone, Default)]
pub struct TunneledConfig {
    pub prefixes: Vec<PathBuf>,
    pub exclude: Vec<String>,
}

pub struct TunneledPathsInner {
    pub prefixes: Vec<PathBuf>,
    pub exclude: Vec<String>,
}

pub fn new() -> TunneledPaths {
    Arc::new(RwLock::new(TunneledPathsInner {
        prefixes: Vec::new(),
        exclude: vec!["update.exe".into()],
    }))
}

pub fn replace(paths: &TunneledPaths, cfg: TunneledConfig) {
    let mut guard = paths.write().unwrap();
    guard.prefixes = cfg.prefixes;
    if cfg.exclude.is_empty() {
        guard.exclude = vec!["update.exe".into()];
    } else {
        guard.exclude = cfg.exclude;
    }
}

pub fn load_from_disk() -> TunneledConfig {
    let Ok(local) = std::env::var("LOCALAPPDATA") else {
        return TunneledConfig::default();
    };
    let config_path = PathBuf::from(&local)
        .join("DiscordTunnel")
        .join("redirector.json");

    let Ok(bytes) = std::fs::read(&config_path) else {
        return TunneledConfig::default();
    };
    let Ok(json) = serde_json::from_slice::<serde_json::Value>(&bytes) else {
        return TunneledConfig::default();
    };

    let prefixes: Vec<PathBuf> = json
        .get("prefixes")
        .and_then(|v| v.as_array())
        .map(|arr| {
            arr.iter()
                .filter_map(|v| v.as_str())
                .filter(|s| !s.is_empty())
                .map(PathBuf::from)
                .collect()
        })
        .unwrap_or_default();

    let exclude: Vec<String> = json
        .get("excludeNames")
        .and_then(|v| v.as_array())
        .map(|arr| {
            arr.iter()
                .filter_map(|v| v.as_str())
                .map(|s| s.to_string())
                .collect()
        })
        .unwrap_or_else(|| vec!["update.exe".into()]);

    TunneledConfig { prefixes, exclude }
}
