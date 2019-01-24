# Start from a Debian image with the latest version of Go installed
# and a workspace (GOPATH) configured at /go.
FROM golang

MAINTAINER Josh Ellithorpe <quest@mac.com>

# Copy the local package files to the container's workspace.
ADD . /go/src/github.com/cashshuffle/cashshuffle

# Switch to the correct working directory.
WORKDIR /go/src/github.com/cashshuffle/cashshuffle

# Restore vendored packages.
RUN go get -u github.com/FiloSottile/gvt
RUN gvt restore

# Build the code.
RUN make install

# Set the start command.
ENTRYPOINT ["/go/bin/cashshuffle"]

# Document that the service listens on ports 1337 and 8080.
EXPOSE 1337 1338 8080
