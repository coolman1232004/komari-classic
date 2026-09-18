FROM alpine:3.24

# Upgrade packages already present in the base; reject mirrors missing the security fix.
RUN apk upgrade --no-cache \
    && apk add --no-cache 'libcrypto3>=3.5.8-r0' 'libssl3>=3.5.8-r0' ca-certificates tzdata

WORKDIR /app

# Docker buildx 会在构建时自动填充这些变量
ARG TARGETOS
ARG TARGETARCH

COPY komari-agent-${TARGETOS}-${TARGETARCH} /app/komari-agent

RUN chmod +x /app/komari-agent

RUN touch /.komari-agent-container

ENTRYPOINT ["/app/komari-agent"]
# 运行时请指定参数
# Please specify parameters at runtime.
# eg: docker run komari-agent -e example.com -t token
CMD ["--help"]