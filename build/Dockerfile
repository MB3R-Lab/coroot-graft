FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS graft-build
ARG TARGETOS=linux
ARG TARGETARCH=amd64
ARG TARGETVARIANT=""
ARG VERSION=0.1.2
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
ARG BERING_REF=812b69982293a02b1be46ee2b2161beced2f658a
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
ARG SHEAFT_REF=7a37c5908a721157f9178d6173395f1cbb29d493
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
ARG VERSION=0.1.2
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
