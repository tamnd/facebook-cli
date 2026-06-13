---
title: "Installation"
description: "Install fb with go install, a prebuilt binary, a package manager, or Docker."
weight: 20
---

fb is a single static binary with no runtime dependencies. Pick whichever
install path fits.

## go install

```sh
go install github.com/tamnd/facebook-cli/cmd/fb@latest
```

This puts `fb` in `$(go env GOPATH)/bin`. Make sure that is on your `PATH`.

## Prebuilt binary

Download an archive for your OS and architecture from the
[releases page](https://github.com/tamnd/facebook-cli/releases), unpack it, and
move `fb` somewhere on your `PATH`. Each release ships archives for Linux, macOS,
Windows, and FreeBSD on amd64 and arm64, plus `.deb`, `.rpm`, and `.apk`
packages, checksums, SBOMs, and a cosign signature.

## Package managers

Homebrew (once the tap is published):

```sh
brew install tamnd/tap/fb
```

Scoop on Windows:

```sh
scoop bucket add tamnd https://github.com/tamnd/scoop-bucket
scoop install fb
```

Debian/Ubuntu and Fedora/RHEL: download the `.deb` or `.rpm` from the releases
page and install it with `dpkg -i` or `rpm -i`.

## Docker

```sh
docker run --rm -e FACEBOOK_COOKIE ghcr.io/tamnd/fb page nasa
```

Mount a volume to keep the cache and any datasets between runs:

```sh
docker run --rm -e FACEBOOK_COOKIE -v ~/data/fb:/data ghcr.io/tamnd/fb \
  page nasa --posts --limit 50 -o jsonl
```

## Build from source

```sh
git clone https://github.com/tamnd/facebook-cli
cd facebook-cli
make build      # produces ./bin/fb
```

## Verify

```sh
fb version
fb id nasa
```

`fb id` works with no network and no login, so it is the quickest way to confirm
the binary runs.
