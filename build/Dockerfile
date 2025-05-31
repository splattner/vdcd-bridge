FROM alpine:3

ARG VDCD_BRIDGE_BIN=vdcd-bridge

LABEL maintainer "Sebastian Plattner"
LABEL org.opencontainers.image.title "vdcd Bridge"
LABEL org.opencontainers.image.description "Bridge to multiple Home Automation System to Digitalstrom using https://github.com/plan44/vdcd"
LABEL org.opencontainers.image.authors "Sebastian Plattner"
LABEL org.opencontainers.image.source https://github.com/splattner/vdcd-bridge/
LABEL org.opencontainers.image.licenses MPL-2.0

RUN apk --no-cache add libc6-compat

COPY ./$VDCD_BRIDGE_BIN /app/vdcd-bridge

WORKDIR /app

RUN adduser -D nonroot
USER nonroot

ENTRYPOINT [ "/app/vdcd-bridge" ]
CMD [ "" ]
