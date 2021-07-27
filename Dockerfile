# build
FROM golang:1-alpine AS build

RUN apk add git
RUN mkdir -p /go/src/github.com/Pandentia
COPY ./ /go/src/github.com/zekker6/protoplex
RUN go get github.com/zekker6/protoplex/cmd/protoplex

# deploy
FROM alpine:latest
COPY --from=build /go/bin/protoplex /protoplex

USER 999
ENTRYPOINT ["/protoplex"]
EXPOSE 8443/tcp
STOPSIGNAL SIGINT
