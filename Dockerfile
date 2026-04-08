# fore-cast API (web server)
FROM golang:1.24-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/web ./cmd/web

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates wget \
  && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/web /usr/local/bin/web
ENV APP_ENV=production
EXPOSE 8080
HEALTHCHECK --interval=5s --timeout=5s --retries=30 \
  CMD wget -qO- http://127.0.0.1:8080/health || exit 1
CMD ["web"]
