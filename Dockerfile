# ---------- build stage ----------
FROM golang:1.25-alpine AS builder

WORKDIR /src

# Dependensi dulu agar layer cache lebih efektif
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-s -w" -o /out/api ./cmd/api \
 && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-s -w" -o /out/migrate ./cmd/migrate \
 && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-s -w" -o /out/seed ./cmd/seed

# ---------- runtime stage ----------
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata wget \
 && adduser -D -u 10001 app

ENV TZ=Asia/Jakarta

WORKDIR /app

COPY --from=builder /out/api /app/api
COPY --from=builder /out/migrate /app/migrate
COPY --from=builder /out/seed /app/seed
# Media pembelajaran (video pelajaran) yang disajikan pada /api/v1/media/*.
COPY --from=builder /src/media /app/media

USER app

# Direktori berkas media yang dilayani backend.
ENV MEDIA_DIR=/app/media

EXPOSE 8080

HEALTHCHECK --interval=15s --timeout=3s --start-period=20s --retries=5 \
  CMD wget -qO- http://127.0.0.1:8080/healthz || exit 1

ENTRYPOINT ["/app/api"]
