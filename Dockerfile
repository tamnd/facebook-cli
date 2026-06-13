# Consumed by GoReleaser: it copies the already cross-compiled binary out of the
# build context rather than compiling, so the image build is fast and uses the
# same static binary every other artifact ships.
#
# GoReleaser builds one multi-platform image with buildx and stages each
# platform's binary under a $TARGETPLATFORM directory (e.g. linux/amd64/) in the
# build context, so the COPY line selects the right one through the automatic
# TARGETPLATFORM build arg.
FROM alpine:3.21

ARG TARGETPLATFORM

# ca-certificates for HTTPS to facebook.com; tzdata for sane timestamps.
RUN apk add --no-cache ca-certificates tzdata \
 && adduser -D -H -u 10001 fb \
 && mkdir -p /data \
 && chown fb:fb /data

COPY $TARGETPLATFORM/fb /usr/bin/fb

USER fb
WORKDIR /data

# Pass your session cookie through the environment to unlock authenticated
# reads, and keep the cache and any datasets under a mounted volume:
#
#   docker run -e FACEBOOK_COOKIE -v ~/data/fb:/data ghcr.io/tamnd/fb page nasa
ENV XDG_CACHE_HOME=/data/cache \
    XDG_DATA_HOME=/data/share
VOLUME ["/data"]

ENTRYPOINT ["/usr/bin/fb"]
