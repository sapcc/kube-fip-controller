FROM --platform=${BUILDPLATFORM:-linux/amd64} golang:1.23-alpine AS builder
WORKDIR /go/src/github.com/sapcc/kube-fip-controller
RUN apk add --no-cache make
COPY . .
ARG VERSION
RUN make all

FROM --platform=${BUILDPLATFORM:-linux/amd64} alpine:3.21
LABEL source_repository="https://github.com/sapcc/kube-fip-controller"

RUN apk upgrade --no-cache --no-progress \
  && apk add --no-cache ca-certificates curl tini \
  && apk del --no-cache --no-progress apk-tools alpine-keys alpine-release libc-utils

COPY --from=builder /go/src/github.com/sapcc/kube-fip-controller/bin/linux/controller /usr/local/bin/
RUN ["controller", "--version"]

ENTRYPOINT ["tini", "--"]
CMD ["controller"]
