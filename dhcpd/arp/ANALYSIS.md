# Is the ARP injection in this package needed?

Research notes on whether `InjectArp`/`InjectArpFd` (used from
`dhcpd/handler.go` to unicast a DHCP reply before the client has an IP) are
necessary, or whether the existing broadcast fallback is sufficient.

**Short answer:** not needed for correctness. The RFC explicitly permits
broadcasting instead, netbootd already contains that exact fallback, and the
fallback is what actually runs today on non-Linux, under `EPERM`, or when the
interface can't be determined. It's a legitimate optimization with real
precedent (dnsmasq does the same trick), but removing it would change nothing
functionally for any known client.

## What the code does and when it runs

`arp.InjectArpFd` issues a raw `ioctl(SIOCSARP)` to pre-seed the kernel ARP
table with `yiaddr → chaddr` before unicasting a DHCP reply. It's called from
exactly one place, the last branch of the reply-addressing chain in
`dhcpd/handler.go` (`HandleMsg4`, around the `response:` label), reached only
when **giaddr = 0, ciaddr = 0, not a NAK, and the client's BROADCAST flag is
clear**. The injection exists because without it, unicasting to `yiaddr` is
impossible: the kernel would ARP for an address the client doesn't have yet,
get silence, and drop the packet.

## What the protocol says

RFC 2131 §4.1 says that with the flag clear, "the server unicasts DHCPOFFER
and DHCPACK messages to the client's hardware address and 'yiaddr' address" —
**but immediately adds: "If unicasting is not possible, the message MAY be
sent as an IP broadcast using an IP broadcast address (preferably
0xffffffff)."** Broadcasting is a sanctioned server behavior, not a
violation. The BROADCAST flag exists to protect clients that *can't* receive
unicast pre-configuration; a clear flag means "unicast is also fine," not "I
reject broadcast" — every real client accepts broadcast replies, since that's
how DHCP bootstraps in the first place.

## Who actually exercises this path

Checked iPXE's source (`src/net/udp/dhcp.c`): it sets `BOOTP_FL_BROADCAST`
only when the link-layer address is name-only or the interface *already has*
an IPv4 address. On a fresh ethernet netboot — netbootd's primary scenario,
including after chainloading `undionly.kpxe`/`ipxe.efi` — **iPXE leaves the
flag clear, so it hits this path on every boot**. Classic Intel PXE ROMs
generally set the flag and take the broadcast branch instead. So the code is
genuinely exercised, by the most important client — but that client also
happily accepts broadcast.

## What everyone else does

- **dnsmasq** — the same `ioctl(SIOCSARP)` injection before unicasting
  (`src/dhcp.c`); this is almost certainly where the technique came from.
- **coredhcp** — which netbootd's handler is derived from — solves it
  differently: it sends a **raw L2 ethernet frame** (`sendEthernet()`)
  addressed to `chaddr`, no ARP table involvement. netbootd swapped that for
  ARP injection.
- **ISC dhcpd / Kea** — raw sockets (BPF/LPF), crafting the L2 header
  themselves.
- **pixiecore's dhcp4 library** — broadcasts server replies by default; its
  own doc comment says implementations that can't set the link-layer
  destination "MAY instead broadcast."

So the industry answer spans all three options; simple netboot-focused
servers lean on broadcast.

## Costs of keeping it

1. **Arch-fragile ABI**: `arpReq` is a hand-built mirror of the kernel's
   `struct arpreq`, split into `arp_linux.go` vs `arp_linux_64.go` purely over
   `RawSockaddr.Data` being `int8` vs `uint8` per arch. A new GOARCH
   (riscv64, etc.) silently falls into whichever build tag matches and may
   miscompile the struct.
2. **Privilege**: `SIOCSARP` needs `CAP_NET_ADMIN`. The commented-out
   hardening line in `netbootd.service` grants only
   `CAP_NET_BIND_SERVICE CAP_NET_RAW` — enabling it would make every
   injection `EPERM` (logged as an error per packet) and silently degrade to
   broadcast anyway. Under least-privilege deployment the feature disables
   itself.
3. **Dead code**: `cmd/arpinject.go` is a debug command that isn't even
   registered (`rootCmd.AddCommand` is commented out).
4. Minor: it writes entries into the host's ARP table (`ATF_COM`, so they age
   out — low pollution risk).

## Verdict

The broadcast fallback four lines below the injection call is RFC-sanctioned,
already battle-tested (it's the only behavior on non-Linux since commit
`03cb277` "Makes ARP magic optional"), and sufficient for iPXE, classic PXE,
and OS DHCP clients alike. Removing `dhcpd/arp` + the injection branch + the
unregistered `arpinject` command would simplify the codebase with no known
client regression — the only observable change is that replies to
flag-clear clients become L2 broadcasts instead of unicasts (marginally more
traffic on the segment; irrelevant at netboot volumes). If unicast fidelity
is ever wanted back, the modern implementations are netlink `RTM_NEWNEIGH`
(clean, no hand-built ioctl structs) or coredhcp's raw-frame approach — both
better than the current ioctl.

## Sources

- [RFC 2131 §4.1](https://www.rfc-editor.org/rfc/rfc2131.txt)
- [iPXE dhcp.c](https://raw.githubusercontent.com/ipxe/ipxe/master/src/net/udp/dhcp.c)
- [dnsmasq src/dhcp.c](https://github.com/imp/dnsmasq/blob/master/src/dhcp.c)
- [coredhcp server/handle.go](https://raw.githubusercontent.com/coredhcp/coredhcp/master/server/handle.go)
- [pixiecore dhcp4/conn.go](https://raw.githubusercontent.com/danderson/netboot/main/dhcp4/conn.go)
- [ARP injection write-up](https://leshow.github.io/post/linux_arp_injection/)
