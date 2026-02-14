FROM alpine:3.21@sha256:c3f8e73fdb79deaebaa2037150150191b9dcbfba68b4a46d70103204c53f4709 AS base
RUN apk --no-cache add ca-certificates
RUN mkdir -p /data && chown 65532:65532 /data

FROM scratch

LABEL org.opencontainers.image.source="https://github.com/sagarc03/stowry"
LABEL org.opencontainers.image.description="Object storage server with pluggable metadata backends"
LABEL org.opencontainers.image.licenses="MIT"

COPY --from=base /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=base /data /data
COPY stowry /stowry

VOLUME /data
ENV STOWRY_STORAGE_PATH=/data

USER 65532:65532

EXPOSE 5708

ENTRYPOINT ["/stowry"]
CMD ["serve"]
