# KD Helpdesk (Libredesk) production image, built from this repo's libredesk/ source
# so KD's code changes ship with it. Used by render.yaml; works on any Docker host.

# ---- Build: Vue frontend + Go binary with static files stuffed in ----
FROM node:22-bookworm AS build
COPY --from=golang:1.27-bookworm /usr/local/go /usr/local/go
ENV PATH="/usr/local/go/bin:/root/go/bin:${PATH}" \
    GOTOOLCHAIN=auto \
    CGO_ENABLED=0 \
    COREPACK_ENABLE_DOWNLOAD_PROMPT=0 \
    CYPRESS_INSTALL_BINARY=0
RUN corepack enable

WORKDIR /src
# Dependency layers first so code-only changes rebuild faster
COPY libredesk/go.mod libredesk/go.sum ./
RUN go mod download
COPY libredesk/frontend/package.json libredesk/frontend/pnpm-lock.yaml libredesk/frontend/pnpm-workspace.yaml ./frontend/
COPY libredesk/frontend/apps ./frontend/apps
COPY libredesk/frontend/shared-ui ./frontend/shared-ui
RUN cd frontend && pnpm install --frozen-lockfile

COPY libredesk/ ./
ARG LIBREDESK_VERSION=v2.9.2-kd
RUN make build LIBREDESK_VERSION=${LIBREDESK_VERSION}

# ---- Runtime: small Alpine image with just the binary ----
FROM alpine:3.20
RUN apk --no-cache add ca-certificates tzdata postgresql16-client && adduser -D -h /libredesk libredesk
WORKDIR /libredesk
COPY --from=build /src/libredesk ./libredesk
COPY --from=build /src/config.sample.toml ./config.toml
COPY deploy/start.sh ./start.sh
# Zendesk-style views + fictional KD sample data, loaded on first boot when
# KD_LOAD_SAMPLE_DATA=true (see start.sh)
COPY dev/seed.sql ./seed.sql
COPY dev/seed-customers.sql ./seed-customers.sql
COPY dev/seed-rmas.sql ./seed-rmas.sql
COPY dev/seed-calls.sql ./seed-calls.sql
RUN mkdir -p uploads && chown -R libredesk:libredesk /libredesk && chmod +x start.sh
USER libredesk

# Production defaults; secrets and connection details come from env vars
ENV LIBREDESK_APP__ENV=prod \
    LIBREDESK_APP__LOG_LEVEL=info \
    LIBREDESK_APP__CHECK_UPDATES=false \
    LIBREDESK_UPLOAD__FS__UPLOAD_PATH=/libredesk/uploads
EXPOSE 9000
CMD ["./start.sh"]
