# ---------- build stage ----------
FROM golang:1.22-alpine AS builder

WORKDIR /build

# Copy everything (flat project — all files live at repo root)
COPY . .

# Static, CGO-free binary
RUN CGO_ENABLED=0 GOOS=linux go build -o server main.go

# ---------- runtime stage ----------
FROM alpine:3.20

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=builder /build/server /app/server

# Data files (persisted at runtime) live alongside the binary
ENV DATA_DIR=/app
ENV PORT=8080

EXPOSE 8080

CMD ["/app/server"]
