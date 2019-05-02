FROM golang:1.12-alpine3.9 as build
ENV GO111MODULE=on
WORKDIR /go/src/app
RUN apk add --no-cache --update build-base git
COPY . .
RUN make linux-amd64 VERSION=

FROM amd64/alpine:3.9
EXPOSE 8080
ENV LISTEN_ADDR 0.0.0.0:8080
RUN apk --no-cache add ca-certificates tzdata
COPY --from=build /go/src/app/miniflux-linux-amd64 /usr/bin/miniflux
USER nobody
CMD ["/usr/bin/miniflux"]