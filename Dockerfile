FROM alpine:latest

COPY protoplex /

USER 999
ENTRYPOINT ["/protoplex"]
EXPOSE 8443/tcp
STOPSIGNAL SIGINT
