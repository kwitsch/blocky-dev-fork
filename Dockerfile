FROM ghcr.io/euantorano/zig:master AS zig-env
# prepare build environment
FROM --platform=$BUILDPLATFORM ghcr.io/gythialy/golang-cross-builder:v1.18.6-0 AS build

# required arguments(buildx will set target)
ARG VERSION
ARG BUILD_TIME
ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT

# arguments to environment
ENV CC="zig cc"
ENV CXX="zig c++"
ENV AR="zig ar"
ENV CGO_ENABLED=0
ENV GOOS=$TARGETOS
ENV GOARCH=$TARGETARCH

# create blocky user passwd file
RUN echo "blocky:x:100:65533:Blocky User,,,:/app:/sbin/nologin" > /tmp/blocky_passwd

# set working directory
WORKDIR /go/src

COPY --from=zig-env /usr/local/bin/zig /usr/local/bin/zig

ENV PATH "/usr/local/bin/zig:${PATH}"

COPY ./docker /scripts
RUN chmod +x /scripts/*.sh

# download packages
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg \
    go mod download


# add source
COPY . .
RUN --mount=type=cache,target=/go/pkg \
    go generate ./...

# build binary
RUN --mount=type=cache,target=/root/.cache/go-build \ 
    --mount=type=cache,target=/go/pkg \
    /scripts/printenv.sh && \
    go build \
    -tags static,sqlite_omit_load_extensions,osusergo,netgo \
    -v \
    -ldflags="-linkmode=external -extldflags=-static -s -w -X github.com/0xERR0R/blocky/util.Version=${VERSION} -X github.com/0xERR0R/blocky/util.BuildTime=${BUILD_TIME}" \
    -o /bin/blocky

RUN setcap 'cap_net_bind_service=+ep' /bin/blocky && \
    chown 100 /bin/blocky

# final stage
FROM scratch

LABEL org.opencontainers.image.source="https://github.com/0xERR0R/blocky" \
      org.opencontainers.image.url="https://github.com/0xERR0R/blocky" \
      org.opencontainers.image.title="DNS proxy as ad-blocker for local network"

WORKDIR /app

COPY --from=build /tmp/blocky_passwd /etc/passwd
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /bin/blocky /app/blocky

USER blocky

ENV BLOCKY_CONFIG_FILE=/app/config.yml

ENTRYPOINT ["/app/blocky"]

HEALTHCHECK --interval=1m --timeout=3s CMD ["/app/blocky", "healthcheck"]
