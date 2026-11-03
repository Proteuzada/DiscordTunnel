//! Discord IPv6 must not leave the physical NIC. Riseup's tunnel here is IPv4,
//! so an IPv6 path would keep Discord's region on the ISP. Packets owned by a
//! tunneled process are dropped; everything else is reinjected unchanged.

use std::sync::Arc;

use anyhow::Result;
use windivert::prelude::WinDivertFlags;

use crate::pidlookup;
use crate::policy::{self, PolicyState};
use crate::process::Resolver;
use crate::tunneled::{self, TunneledPaths};

pub fn run(policy_state: PolicyState, resolver: Arc<Resolver>, tunneled_paths: TunneledPaths) -> Result<()> {
    let filter = "outbound and ipv6 and (tcp or udp) and !loopback";
    let capture = crate::wdopen::network(filter, 1041, WinDivertFlags::new())?;
    tracing::info!("ipv6 block open for tunneled apps");

    let mut buffer = vec![0u8; 65535];
    loop {
        let Ok(Some(packet)) = capture.recv_wait(&mut buffer, 50) else {
            continue;
        };
        let drop_it = match parse_src_port(&packet.data) {
            Some((port, proto)) => match pidlookup::pid_for_local_v6(port, proto) {
                Some(pid) if policy::contains(&policy_state, pid) => true,
                Some(pid) => resolver.resolve(pid).is_some_and(|info| {
                    tunneled::matches(&tunneled_paths, std::path::Path::new(&info.exe_path))
                }),
                None => false,
            },
            None => false,
        };
        if drop_it {
            continue;
        }
        let _ = capture.send(&packet);
    }
}

fn parse_src_port(data: &[u8]) -> Option<(u16, u8)> {
    if data.len() < 40 || data[0] >> 4 != 6 {
        return None;
    }
    let mut next = data[6];
    let mut off = 40usize;
    for _ in 0..6 {
        if next == 6 || next == 17 {
            if data.len() < off + 4 {
                return None;
            }
            let port = u16::from_be_bytes([data[off], data[off + 1]]);
            return Some((port, next));
        }
        if data.len() < off + 2 {
            return None;
        }
        let hdr_len = match next {
            0 | 43 | 60 => (data[off + 1] as usize + 1) * 8,
            44 => 8,
            _ => return None,
        };
        next = data[off];
        off += hdr_len;
    }
    None
}
