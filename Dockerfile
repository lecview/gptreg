FROM python:3.11-slim

# curl_cffi 需要的系统依赖
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        build-essential \
        libcurl4-openssl-dev \
        libssl-dev \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY requirements.txt .
RUN unset ALL_PROXY all_proxy HTTP_PROXY HTTPS_PROXY http_proxy https_proxy && \
    pip install --no-cache-dir -r requirements.txt

COPY . .

RUN mkdir -p files logs && chmod +x entrypoint.sh

ENTRYPOINT ["sh", "entrypoint.sh"]
