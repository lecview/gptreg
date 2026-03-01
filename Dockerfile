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
RUN pip install --no-cache-dir -r requirements.txt

COPY . .

RUN mkdir -p files logs

# 容器保持运行，不自动执行注册
CMD ["sleep", "infinity"]
