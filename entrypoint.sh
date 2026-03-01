#!/bin/sh
# MANUAL_MODE=1 手动模式（容器待命），0 或不设 自动运行注册
if [ "${MANUAL_MODE}" = "1" ]; then
    echo "[entrypoint] 手动模式，等待 CMD 执行: python auto.py"
    sleep infinity
else
    echo "[entrypoint] 自动模式，开始注册"
    exec python auto.py
fi
