# --- Frontend build stage ---
FROM node:22-alpine AS frontend-builder

RUN corepack enable pnpm

WORKDIR /src

# Cache dependencies
COPY pnpm-workspace.yaml package.json pnpm-lock.yaml ./
COPY apps/web/package.json ./apps/web/
RUN pnpm install --frozen-lockfile

# Copy frontend source
COPY apps/web/ ./apps/web/

# Build frontend (standalone output)
RUN pnpm --filter @multicode/web build

# --- Go build stage ---
FROM golang:1.26-alpine AS go-builder

RUN apk add --no-cache git

WORKDIR /src

# Cache dependencies
COPY server/go.mod server/go.sum ./server/
RUN cd server && go mod download

# Copy server source
COPY server/ ./server/

# Build binaries
ARG VERSION=dev
ARG COMMIT=unknown
RUN cd server && CGO_ENABLED=0 go build -ldflags "-s -w" -o bin/server ./cmd/server
RUN cd server && CGO_ENABLED=0 go build -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" -o bin/multicode ./cmd/multicode
RUN cd server && CGO_ENABLED=0 go build -ldflags "-s -w" -o bin/migrate ./cmd/migrate

# --- Runtime stage ---
FROM alpine:3.21

# Node.js is required for Next.js standalone server
RUN apk add --no-cache ca-certificates tzdata nodejs

WORKDIR /app

# Go binaries
COPY --from=go-builder /src/server/bin/server .
COPY --from=go-builder /src/server/bin/multicode .
COPY --from=go-builder /src/server/bin/migrate .
COPY server/migrations/ ./migrations/

# Frontend standalone output
# Next.js standalone produces: server.js at root + node_modules/ + apps/web/
COPY --from=frontend-builder /src/apps/web/.next/standalone ./
COPY --from=frontend-builder /src/apps/web/.next/static ./apps/web/.next/static
COPY --from=frontend-builder /src/apps/web/public ./apps/web/public

# Entrypoint script
COPY scripts/docker-entrypoint.sh ./docker-entrypoint.sh
RUN chmod +x ./docker-entrypoint.sh

EXPOSE 8080 3000

ENTRYPOINT ["./docker-entrypoint.sh"]
