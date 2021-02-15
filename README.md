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

netbootd's configuration consists of a set of manifest, each representing a machine. 
In order to support automation-based workflow, manifests can be managed via a simple HTTP API.

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

TFTP and HTTP content can either be static text (embedded in the manifest),
generated content (using Go's `text/template` templating engine) or proxied to upstream HTTP(S).
This last feature is mainly intended to proxy TFTP to HTTP(S) but very well may be used to
reverse-proxy HTTP in otherwise isolated environments and can use a proxy itself
(`HTTP_PROXY` and `NO_PROXY` is honored automatically by Go).

netbootd cannot serve local files. An exception is a bundled version of [iPXE](https://ipxe.org/),
which allows to download (typically) kernel and initrd over HTTP instead of TFTP.

## Manifests

A manifest represents a machine to be provisioned/served. The behavior of built-in
DHCP, TFTP and HTTP server is specific to a manifest, meaning that it varies based
on source MAC/IP. Each host may see different content at `/something` path.

Note that this is not a security feature and you should not host any sensitive content.
MAC and IPs can be easily spoofed. In fact, netbootd includes a convenience feature to
spoof source IP for troubleshooting purposes. Append `?spoof=<ip-address>` to HTTP request
to see the response for a particular host. There is no TFTP counterpart of this feature.

Example manifests are included in the `examples/` directory.

## HTTP API

TODO.

## Usage

```
Usage:
  netbootd [flags]

Flags:
  -a, --address string     IP address to listen on (DHCP, TFTP, HTTP)
  -r, --api-port int       HTTP API port to listen on (default 8081)
  -d, --debug              enable debug logging
  -h, --help               help for netbootd
  -p, --http-port int      HTTP port to listen on (default 8080)
  -i, --interface string   interface to listen on, e.g. eth0 (DHCP)
  -m, --manifests string   load manifests from directory
      --trace              enable trace logging
```

Run e.g. `./netbootd --trace -m ./examples/`
 
## Roadmap / TODOs

* API TLS & Authentication
* Manifest persistence (currently API-configured manifests live in memory only)
* Pluggable store backends (e.g. Redis, Etcd, files) for Manifests
* Notifications (e.g. long-polling wait to return when a given host actually booted)
* Per-manifest logs available over API

