FROM alpine:3.20 AS certs
RUN apk --no-cache add ca-certificates

FROM scratch

COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY stowry /stowry

ENV STOWRY_STORAGE_PATH=/data

USER 65532:65532

EXPOSE 5708

ENTRYPOINT ["/stowry"]
CMD ["serve"]
