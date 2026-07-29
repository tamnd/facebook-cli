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

# FB_DATA_DIR is where fb keeps the page cache, any store and the session file,
# so mounting /data is what makes all three survive between runs:
#
#   docker run --rm -v ~/data/fb:/data ghcr.io/tamnd/fb page nasa
#
# Reads need nothing at all. A session, if you want one, is two cookies imported
# once into the mounted volume with `fb auth import`.
ENV FB_DATA_DIR=/data
VOLUME ["/data"]

ENTRYPOINT ["/usr/bin/fb"]
