# ---- Build stage ----
FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# CGO=0: binary statik, aman di alpine/tanpa glibc
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/snjsync .

# ---- Runtime stage ----
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
# Aset web + template
COPY *.html *.js /app/
COPY bg-dashboard.jpeg template_output.xlsx /app/
# Binary
COPY --from=build /out/snjsync /app/snjsync

# Data runtime (data/), config.json, client_secret.json, .env
# dipasang via volume/ mount saat deploy — jangan di-bake ke image.
ENV CTF_ROOT=/app \
    CTF_NO_PG_AUTOSTART=1
EXPOSE 8877
CMD ["/app/snjsync"]