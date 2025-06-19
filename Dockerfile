FROM golang:1.24 as build-env

WORKDIR /go/src/app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /go/bin/app

FROM debian:bookworm-slim as cert-bundler
RUN apt-get update && apt-get install ca-certificates --yes

FROM gcr.io/distroless/static:latest
USER 1001:1001
WORKDIR /app
COPY --from=build-env /go/bin/app /app
COPY --from=cert-bundler /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
ENV APP_ADDR ":8080"
EXPOSE 8080
ENTRYPOINT ["./app"]
