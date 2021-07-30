# build
FROM golang:1-alpine AS build

WORKDIR /app

COPY ./ ./

RUN cd cmd/protoplex  && go build -o /go/bin/protoplex

# deploy
FROM alpine:latest
COPY --from=build /go/bin/protoplex /protoplex

USER 999
ENTRYPOINT ["/protoplex"]
EXPOSE 8443/tcp
STOPSIGNAL SIGINT
