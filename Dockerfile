FROM golang:1.24-alpine AS go-builder
WORKDIR /src
COPY openai-sentinel-go/ ./openai-sentinel-go/
WORKDIR /src/openai-sentinel-go
RUN go build -o /sentinel-cli cmd/cli/main.go

FROM python:3.11-slim

# curl_cffi 需要的系统依赖
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        build-essential \
        libcurl4-openssl-dev \
        libssl-dev \
        curl \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY requirements.txt .
RUN unset ALL_PROXY all_proxy HTTP_PROXY HTTPS_PROXY http_proxy https_proxy && \
    pip install --no-cache-dir -r requirements.txt

COPY . .
COPY --from=go-builder /sentinel-cli /app/sentinel-cli

RUN mkdir -p files logs && chmod +x entrypoint.sh && chmod +x /app/sentinel-cli

ENTRYPOINT ["sh", "entrypoint.sh"]
