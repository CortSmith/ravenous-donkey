# Course development image: the exact toolchain the course targets.
#
# Debian (bookworm), not Alpine: the Debian image ships gcc, which cgo and
# therefore `go test -race` need. The Alpine image has no C compiler.
FROM golang:1.24.2-bookworm

# Caches live under /cache so docker/compose.base.yaml can keep them in named
# volumes that survive container restarts (fast rebuilds, no re-downloads).
# GOTOOLCHAIN=local stops the go command from downloading a different
# toolchain: inside the container you always get exactly 1.24.2.
# HOME=/tmp because the container runs as your host UID, which has no home
# directory in the image.
ENV GOCACHE=/cache/go-build \
    GOMODCACHE=/cache/mod \
    GOTOOLCHAIN=local \
    HOME=/tmp

# World-writable (sticky) so any UID can use them. Docker copies these
# permissions into a named volume the first time it is mounted. /data holds
# state for services such as the capstone's kvd server.
RUN mkdir -p /cache/go-build /cache/mod /data && chmod 1777 /cache/go-build /cache/mod /data

# A regular user for VS Code dev containers (.devcontainer/), which remaps it
# to your host UID. Compose runs as "${UID}:${GID}" instead and ignores it.
RUN useradd --create-home --uid 1000 --shell /bin/bash gopher

WORKDIR /course
