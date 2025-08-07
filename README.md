# netbootd

netbootd is a lightweight network boot server, designed for maximum flexibility
and with "batteries included" approach in mind, serving as a DHCP, TFTP and HTTP server.
It includes a basic templating functionality, designed to allow generating e.g. preseed
files for unattended OS installation.

It can be compared to [Foreman](https://github.com/theforeman/foreman) or [Cobbler](https://github.com/cobbler/cobbler),
as the goal is to PXE-boot a machine into an operating system or installation environment.

Unlike Foreman and Cobbler, netbootd is actually a DHCP, TFTP and HTTP server.
It does not require any other software to be used.

netbootd aims to provide maximum flexibility and unlike Foreman or Cobbler makes
no attempt to simplify the process of network booting. The results will be only
as good as the configuration (the manifest in this case).

netbootd's configuration consists of a set of manifests, with each manifest representing a machine
to be provisioned. Netbootd also provides a simple HTTP API for managing manifests, so that netbootd
can become part of a larger automation workflow.

**Note: This software is highly experimental, at proof-of-concept stage. It works
but a lot of critical features are missing.**

## DHCP

netbootd includes a DHCP server that will respond ONLY to MAC addresses found in
one of the manifests. It does not implement the concept of leases as IPs are implied
to be statically allocated via manifest configuration.

Multiple options are supported, such as router, hostname, domain, DNS, NTP,
and naturally NBP.

## TFTP and HTTP

netbootd exposes all "mounts" via both TFTP and HTTP simultatenously.
Naturally, it's not a good idea to transfer really large files over TFTP but PXE generally
requires use of TFTP in most cases.

TFTP and HTTP content can either be static text (embedded in the manifest), generated content (using
Go's `text/template` templating engine) or proxied to upstream HTTP(S). This last feature is mainly intended to proxy
TFTP to HTTP(S) but very well may be used to reverse-proxy HTTP in otherwise isolated environments and can use a proxy
itself
(`HTTP_PROXY` and `NO_PROXY` is honored automatically by Go).

netbootd can serve local files using the `path.localDir` configuration option.
netbootd also contains a bundled version of [iPXE](https://ipxe.org/), which allows
downloading (typically) kernel and initrd over HTTP instead of TFTP.

## Manifests

A manifest represents a machine to be provisioned/served. The behavior of built-in DHCP, TFTP and HTTP server is
specific to a manifest, meaning that it varies based on source MAC/IP. Each host may see different content
at `/something` path.

Note that this is not a security feature, and you should not host any sensitive content. MAC and IPs can be easily
spoofed. In fact, netbootd includes a convenience feature to spoof source IP for troubleshooting purposes.
Append `?spoof=<ip-address>` to HTTP request to see the response for a particular host. There is no TFTP counterpart of
this feature.

Example manifests are included in the `examples/` directory.

### Anatomy of a manifest

```yaml
---
# ID can be anything unique, URL-safe, used to identify it for HTTP API
id: ubuntu-1804

### DHCP options - used for DHCP responses from netbootd
# IP address with subnet (CIDR) to give out
ipv4: 192.168.17.101/24
# Hostname (without domain part) (Option 12)
hostname: ubuntu-machine-1804
# Domain part (used for hostname) (Option 15)
domain: test.local
# Lease duration is used as Option 51
# Note that netbootd is a static-assignment server, which does not prevent IP conflicts.
leaseDuration: 1h
# The MAC addresses which map to this manifest
# List multiple for machine with multiple NICs, if not sure which one boots first
mac:
  - 00:15:5d:bd:be:15
  - aa:bb:cc:dd:ee:fc
# Domain name servers (DNS) in the order of preference (Option 6)
dns:
  - 1.2.3.4
  - 3.4.5.6
# Routers in the order of preference (Option 3), more than one is rare
router:
  - 192.168.17.1
# NTP servers in the order of preference (Option 42), IP address required
ntp:
  - 192.168.17.1
# Whether a bundled iPXE bootloader should be served first (before bootFilename).
# When iPXE is loaded, it does DHCP again and netbootd detects its client string
# to break the boot loop and serve bootFilename instead.
ipxe: true
# The name of NBP file name, server over TFTP from "next server",
# which netbootd automatically points to be itself.
# This should map to a "mount" below.
bootFilename: install.ipxe

# Mounts define virtual per-host (per-manifest) paths that are acessible
# over both TFTP and HTTP but only from the IP address of in this manifest.
# Each mount can be either a proxy mount (HTTP/HTTPS proxy) or a content mount (static).
mounts:
  - path: /netboot
    # When true, all paths starting with this prefix use this mount.
    pathIsPrefix: true
    # When proxy is defined, these requests are proxied to a HTTP/HTTPS address.
    proxy: http://archive.ubuntu.com/ubuntu/dists/bionic-updates/main/installer-amd64/current/images/hwe-netboot/ubuntu-installer/amd64/
    # When true, the proxy path defined above gets a suffix to the Path prefix appended to it.
    appendSuffix: true

  - path: /subdir
    # When true, all paths starting with this prefix use this mount.
    pathIsPrefix: true
    # Provides a path on the host to find the files.
    # So that localDir: /tftpboot path: /subdir and client request: /subdir/file.x so that the host
    # path becomes /tfptboot/file.x
    localDir: /tftpboot
    # When true, the localDir path defined above gets a suffix to the Path prefix appended to it.
    appendSuffix: true

  - path: /install.ipxe
    # The templating context provides access to: .LocalIP, .RemoteIP, .HttpBaseUrl, .ApiBaseUrl and .Manifest.
    # Sprig functions are available: masterminds.github.io/sprig
    content: |
      #!ipxe
      # See https://ipxe.org/scripting for iPXE commands/scripting documentation

      set base {{ .HttpBaseUrl }}/netboot

      {{ $hostnameParts := splitList "." .Manifest.Hostname }}
      kernel ${base}/linux gfxpayload=800x600x16,800x600 initrd=initrd.gz auto=true url={{ .HttpBaseUrl.String }}/preseed.txt netcfg/get_ipaddress={{ .Manifest.IPv4.IP }} netcfg/get_netmask={{ .Manifest.IPv4.Netmask }} netcfg/get_gateway={{ first .Manifest.Router }} netcfg/get_nameservers="{{ .Manifest.DNS | join " " }}" netcfg/disable_autoconfig=true hostname={{ first $hostnameParts }} domain={{ rest $hostnameParts | join "." }} DEBCONF_DEBUG=developer
      initrd ${base}/initrd.gz
      boot
```

## HTTP API

In this preview/development version, this HTTP API does not support authentication.

<details>
<summary>GET /api/manifests</summary>
Returns a dictionary of all manifests keyed by their ID.

Supports `Accept` header (if provided) that allows selecting a json output (`Accept: application/json`).
</details>

<details>
<summary>GET /api/manifests/{id}</summary>
Returns a single manifest with ID provided in the URL path.

Supports `Accept` header (if provided) that allows selecting a json output (`Accept: application/json`).

Returns:

* 200 for successful response
* 404 if manifest with provided ID does not exist

</details>

<details>
<summary>PUT /api/manifests/{id}</summary>
Accepts a manifest in either JSON (`Content-type: application/json`) or YAML (default) format.

Returns:

* 201 Created on success
* 400 for malformed request (invalid manifest)

</details>

<details>
<summary>DELETE /api/manifests/{id}</summary>
Ensures that manifest with provided ID does not exist.

Always returns 204, even if manifest already did not exist.
</details>

<details>
<summary>GET|POST /api/self/suspend-boot</summary>
Allows a provisioned host to ask not to be booted again.
This does not block DHCP, TFTP or HTTP requests, it only removes NBP information from DHCP responses.

This operation looks for a manifest matching the IP address of the requester. It is possible to spoof it
with `?spoof=1.2.3.4` query parameter.
</details>

<details>
<summary>GET|POST /api/self/unsuspend-boot</summary>
Re-enables booting for a provisioned host.

This operation looks for a manifest matching the IP address of the requester. It is possible to spoof it
with `?spoof=1.2.3.4` query parameter.
</details>

<details>
<summary>GET /api/self/manifest</summary>
Returns a manifest matching requester's IP Address.

Supports `Accept` header (if provided) that allows selecting a json output (`Accept: application/json`).

This operation looks for a manifest matching the IP address of the requester. It is possible to spoof it
with `?spoof=1.2.3.4` query parameter.
</details>

## Usage

```
Usage:
  netbootd server [flags]

Flags:
  -a, --address string        IP address to listen on (DHCP, TFTP, HTTP)
  -r, --api-port int          HTTP API port to listen on (default 8081)
      --api-tls-cert string   Path to TLS certificate API
      --api-tls-key string    Path to TLS certificate for API
  -h, --help                  help for server
  -p, --http-port int         HTTP port to listen on (default 8080)
  -i, --interface string      interface to listen on, e.g. eth0 (DHCP)
  -m, --manifests string      load manifests from directory

Global Flags:
  -d, --debug                    enable debug logging
      --disable-journal-logger   disable zerolog journald logger
      --trace                    enable trace logging
```

Run e.g. `./netbootd --trace server -m ./examples/`
 
## Roadmap / TODOs

* [x] API TLS & Authentication
* [ ] Manifest persistence (currently API-configured manifests live in memory only)
* [ ] Pluggable store backends (e.g. Redis, Etcd, files) for Manifests
* [ ] Notifications (e.g. long-polling wait to return when a given host actually booted)
* [ ] Per-manifest logs available over API
