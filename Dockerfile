# Default image: scratch. Nothing but the server, the CA bundle and the data
# directory.
#
# The build context is produced by GoReleaser's dockers_v2 pipe, which lays the
# binaries out per platform (linux/amd64/stowry, linux/arm64/stowry), so the
# binary is selected with $TARGETPLATFORM rather than copied from the root.

# Helper stage. Pinned to the *build* platform so the package manager never runs
# under emulation: everything taken from here (the CA bundle, an empty data
# directory) is architecture independent.
FROM --platform=$BUILDPLATFORM alpine:3.21@sha256:c3f8e73fdb79deaebaa2037150150191b9dcbfba68b4a46d70103204c53f4709 AS base
RUN apk --no-cache add ca-certificates
RUN mkdir -p /data

FROM scratch
ARG TARGETPLATFORM

LABEL org.opencontainers.image.source="https://github.com/sagarc03/stowry"
LABEL org.opencontainers.image.description="Object storage server with pluggable metadata backends"
LABEL org.opencontainers.image.licenses="MIT"
LABEL org.opencontainers.image.base.name="scratch"

COPY --from=base /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
# --chown is required: COPY copies the *contents* of a directory, so without it
# /data is recreated as root and the unprivileged user below cannot write to it.
COPY --from=base --chown=65532:65532 /data /data
COPY $TARGETPLATFORM/stowry /stowry
# Apache-2.0 dependencies require their licence to ship with the binary.
COPY THIRD_PARTY_LICENSES /THIRD_PARTY_LICENSES

VOLUME /data
ENV STOWRY_STORAGE_PATH=/data

USER 65532:65532

EXPOSE 5708

ENTRYPOINT ["/stowry"]
CMD ["serve"]
