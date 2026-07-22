FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS graft-build
ARG TARGETOS=linux
ARG TARGETARCH=amd64
ARG TARGETVARIANT=""
ARG VERSION=1.0.0
ARG COMMIT=unknown
ARG BUILD_DATE=1970-01-01T00:00:00Z
WORKDIR /src

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal

RUN set -eu; \
    export CGO_ENABLED=0 GOOS="${TARGETOS}" GOARCH="${TARGETARCH}"; \
    if [ "${TARGETARCH}" = "arm" ] && [ -n "${TARGETVARIANT}" ]; then export GOARM="${TARGETVARIANT#v}"; fi; \
    go build -trimpath -ldflags="-s -w" -o /out/coroot-graft ./cmd/coroot-graft

FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS bering-build
ARG TARGETOS=linux
ARG TARGETARCH=amd64
ARG TARGETVARIANT=""
ARG BERING_REPOSITORY=https://github.com/MB3R-Lab/Bering
ARG BERING_REF=d858f09a8cca8edf302646a54b28412d158c0ec2
WORKDIR /src
RUN apk add --no-cache git
RUN git clone "${BERING_REPOSITORY}" bering
WORKDIR /src/bering
RUN git checkout "${BERING_REF}"
RUN go mod download
RUN set -eu; \
    export CGO_ENABLED=0 GOOS="${TARGETOS}" GOARCH="${TARGETARCH}"; \
    if [ "${TARGETARCH}" = "arm" ] && [ -n "${TARGETVARIANT}" ]; then export GOARM="${TARGETVARIANT#v}"; fi; \
    go build -trimpath -ldflags="-s -w" -o /out/bering ./cmd/bering

FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS sheaft-build
ARG TARGETOS=linux
ARG TARGETARCH=amd64
ARG TARGETVARIANT=""
ARG SHEAFT_REPOSITORY=https://github.com/MB3R-Lab/Sheaft
ARG SHEAFT_REF=7f3e8e447762332f0eec5d8cf1938c1250e43302
WORKDIR /src
RUN apk add --no-cache git
RUN git clone "${SHEAFT_REPOSITORY}" sheaft
WORKDIR /src/sheaft
RUN git checkout "${SHEAFT_REF}"
RUN go mod download
RUN set -eu; \
    export CGO_ENABLED=0 GOOS="${TARGETOS}" GOARCH="${TARGETARCH}"; \
    if [ "${TARGETARCH}" = "arm" ] && [ -n "${TARGETVARIANT}" ]; then export GOARM="${TARGETVARIANT#v}"; fi; \
    go build -trimpath -ldflags="-s -w" -o /out/sheaft ./cmd/sheaft

FROM alpine:3.20
ARG VERSION=1.0.0
ARG COMMIT=unknown
ARG BUILD_DATE=1970-01-01T00:00:00Z
LABEL org.opencontainers.image.title="coroot-graft" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${COMMIT}" \
      org.opencontainers.image.created="${BUILD_DATE}"
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=graft-build /out/coroot-graft /usr/local/bin/coroot-graft
COPY --from=bering-build /out/bering /usr/local/bin/bering
COPY --from=sheaft-build /out/sheaft /usr/local/bin/sheaft
ENTRYPOINT ["coroot-graft"]
