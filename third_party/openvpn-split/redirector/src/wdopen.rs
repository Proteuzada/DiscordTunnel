//! WinDivertOpen deadlocks if two threads call it at the same time while the
//! driver is loading. The IPv4 divert and the IPv6 drop both open a handle
//! at startup; without this lock neither handle comes back and Discord
//! stays on the physical NIC.

use std::sync::Mutex;

use anyhow::{Context, Result};
use windivert::layer::NetworkLayer;
use windivert::prelude::*;

static OPEN_LOCK: Mutex<()> = Mutex::new(());

pub fn network(filter: &str, priority: i16, flags: WinDivertFlags) -> Result<WinDivert<NetworkLayer>> {
    let _guard = OPEN_LOCK.lock().unwrap_or_else(|e| e.into_inner());
    tracing::info!("WinDivert open priority={priority} filter={filter}");
    let handle = WinDivert::network(filter, priority, flags)
        .with_context(|| format!("WinDivert open failed: {filter}"))?;
    tracing::info!("WinDivert open ok priority={priority}");
    Ok(handle)
}
