############################
# STEP 1 build executable binary
############################
# golang 1.13.1
FROM golang:1.13.1-alpine3.10 as builder
# Install git + SSL ca certificates.
# Git is required for fetching the dependencies.
# Ca-certificates is required to call HTTPS endpoints.
RUN apk update && apk add --no-cache git ca-certificates tzdata curl && update-ca-certificates

WORKDIR $GOPATH/src/gluent-bit
COPY . .

# Fetch dependencies.
RUN go mod download

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -a -installsuffix cgo -o /go/bin/gluent-bit .

############################
# STEP 2 build a small image
############################
FROM scratch

# Import from builder.
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /etc/passwd /etc/passwd

# Copy our static executable
COPY --from=builder /go/bin/gluent-bit /go/bin/gluent-bit

ENTRYPOINT ["/go/bin/gluent-bit"]
