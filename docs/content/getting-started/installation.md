---
title: "Installation"
description: "Install fb with go install, a prebuilt binary, a package manager, or Docker."
weight: 20
---

fb is one static binary with no runtime dependencies.
The SQLite store is pure Go, so there is no cgo and nothing to link against.
Pick whichever path fits.

## go install

```sh
go install github.com/tamnd/facebook-cli/cmd/fb@latest
```

That puts `fb` in `$(go env GOPATH)/bin`, which needs to be on your `PATH`.

## Prebuilt binary

Download an archive for your OS and architecture from the [releases page](https://github.com/tamnd/facebook-cli/releases), unpack it, and move `fb` somewhere on your `PATH`.
Every release ships Linux on amd64, arm64, armv7 and 386, macOS on amd64 and arm64, Windows on amd64 and arm64, and FreeBSD on amd64 and arm64.

Alongside the archives are `.deb`, `.rpm` and `.apk` packages, a sha256 checksum file, a CycloneDX SBOM for each artifact, and a keyless cosign signature over the checksums.

## Package managers

Homebrew, once the tap is published:

```sh
brew install --cask tamnd/tap/fb
```

Scoop on Windows:

```sh
scoop bucket add tamnd https://github.com/tamnd/scoop-bucket
scoop install fb
```

Debian, Ubuntu, Fedora, RHEL and Alpine: download the `.deb`, `.rpm` or `.apk` from the releases page and install it with `dpkg -i`, `rpm -i` or `apk add --allow-untrusted`.

## Docker

```sh
docker run --rm ghcr.io/tamnd/fb page nasa
```

The image is Alpine with the static binary, CA certificates and tzdata, running as an unprivileged user.
`FB_DATA_DIR` is set to `/data`, so mounting a volume there is what makes the page cache, any store and the session file survive between runs:

```sh
docker run --rm -v ~/data/fb:/data ghcr.io/tamnd/fb \
  photos nasa --limit 50 -o jsonl
```

## Build from source

```sh
git clone https://github.com/tamnd/facebook-cli
cd facebook-cli
make build      # writes bin/fb
```

`bin/` is gitignored, and the binary goes there rather than the repo root because the root already holds the `fb/` source package.

## Verify

```sh
fb --version
fb id nasa
fb surfaces
```

None of those three touch the network, so they work behind any firewall and are the quickest way to confirm the binary runs.
Then try a real read:

```sh
fb page nasa
```
