FROM --platform=${BUILDPLATFORM:-linux/amd64} golang:1.24-alpine AS builder
WORKDIR /workspace
RUN apk update && apk add make
COPY . .
RUN go mod download
RUN make build CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH}

FROM --platform=${BUILDPLATFORM:-linux/amd64} alpine:3.21@sha256:ce64758a109eb420d874a118f87920e625e12d3634e03b4a5573fd9f6e5d3507
LABEL source_repository="https://github.com/sapcc/kube-fip-controller"

WORKDIR /

RUN apk upgrade --no-cache --no-progress \
  && apk add --no-cache ca-certificates curl tini \
  && apk del --no-cache --no-progress apk-tools alpine-keys alpine-release libc-utils

COPY --from=builder /workspace/bin/kube-fip-controller /usr/local/bin/
RUN ["kube-fip-controller", "--version"]

ENTRYPOINT ["tini", "--"]
CMD ["kube-fip-controller"]
