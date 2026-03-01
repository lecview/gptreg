import json
import os
import re
import sys
import time
import secrets
import hashlib
import base64
import logging
import urllib
import urllib.parse
import urllib.request
import urllib.error
from concurrent.futures import ThreadPoolExecutor, as_completed
from dataclasses import dataclass
from typing import Any, Dict

import random
import yaml
import requests as std_requests
from curl_cffi import requests

# ========== 日志 ==========

log = logging.getLogger("auto_register")


def setup_logging(log_to_file: bool):
    log.setLevel(logging.INFO)
    fmt = logging.Formatter("[%(asctime)s] [%(levelname)s] [%(threadName)s] %(message)s", datefmt="%H:%M:%S")

    ch = logging.StreamHandler(sys.stdout)
    ch.setFormatter(fmt)
    log.addHandler(ch)

    if log_to_file:
        os.makedirs("logs", exist_ok=True)
        fh = logging.FileHandler(f"logs/{time.strftime('%Y%m%d_%H%M%S')}.log", encoding="utf-8")
        fh.setFormatter(fmt)
        log.addHandler(fh)


# ========== 临时邮箱 (Cloudflare Worker) ==========

def create_email(cfg: dict) -> dict:
    """通过 OWN_DOMAIN 生成随机邮箱，返回 {"address": ...}"""
    domain = cfg.get("own_domain")
    if not domain:
        raise ValueError("缺少 OWN_DOMAIN 配置")
    login = f"oai_{int(time.time())}_{random.randint(100, 999)}"
    address = f"{login}@{domain}"
    return {"address": address}


def get_oai_code(email: str, cfg: dict) -> str:
    """通过 Cloudflare Worker API 轮询获取验证码"""
    worker_url = cfg.get("worker_url")
    if not worker_url:
        raise ValueError("缺少 WORKER_URL 配置")
    worker_url = worker_url.rstrip("/")

    log.info(f"正在通过 Worker 监听验证码 (目标: {email})...")
    for i in range(60):
        try:
            resp = std_requests.get(worker_url, params={"email": email}, timeout=10)
            if resp.status_code == 200:
                try:
                    data = resp.json()
                    raw_code = (data.get("code") or data.get("code_grok")
                                or data.get("code_digit") or data.get("code_alpha"))
                    if raw_code:
                        clean_code = str(raw_code).replace("-", "").strip()
                        log.info(f"Worker 返回验证码: {clean_code} (原始: {raw_code})")
                        return clean_code
                except Exception:
                    if len(resp.text) < 20:
                        clean_code = resp.text.replace("-", "").strip()
                        if clean_code:
                            return clean_code
        except Exception as e:
            log.warning(f"Worker 请求异常: {e}")
        time.sleep(3)
    return None


# ========== OAuth ==========

AUTH_URL = "https://auth.openai.com/oauth/authorize"
TOKEN_URL = "https://auth.openai.com/oauth/token"
CLIENT_ID = "app_EMoamEEZ73f0CkXaXp7hrann"
DEFAULT_REDIRECT_URI = "http://localhost:1455/auth/callback"
DEFAULT_SCOPE = "openid email profile offline_access"


def _b64url_no_pad(raw: bytes) -> str:
    return base64.urlsafe_b64encode(raw).decode("ascii").rstrip("=")


def _sha256_b64url_no_pad(s: str) -> str:
    return _b64url_no_pad(hashlib.sha256(s.encode("ascii")).digest())


def _random_state(nbytes: int = 16) -> str:
    return secrets.token_urlsafe(nbytes)


def _pkce_verifier() -> str:
    return secrets.token_urlsafe(64)


def _parse_callback_url(callback_url: str) -> Dict[str, str]:
    candidate = callback_url.strip()
    if not candidate:
        return {"code": "", "state": "", "error": "", "error_description": ""}

    if "://" not in candidate:
        if candidate.startswith("?"):
            candidate = f"http://localhost{candidate}"
        elif any(ch in candidate for ch in "/?#") or ":" in candidate:
            candidate = f"http://{candidate}"
        elif "=" in candidate:
            candidate = f"http://localhost/?{candidate}"

    parsed = urllib.parse.urlparse(candidate)
    query = urllib.parse.parse_qs(parsed.query, keep_blank_values=True)
    fragment = urllib.parse.parse_qs(parsed.fragment, keep_blank_values=True)

    for key, values in fragment.items():
        if key not in query or not query[key] or not (query[key][0] or "").strip():
            query[key] = values

    def get1(k: str) -> str:
        v = query.get(k, [""])
        return (v[0] or "").strip()

    code = get1("code")
    state = get1("state")
    error = get1("error")
    error_description = get1("error_description")

    if code and not state and "#" in code:
        code, state = code.split("#", 1)
    if not error and error_description:
        error, error_description = error_description, ""

    return {"code": code, "state": state, "error": error, "error_description": error_description}


def _jwt_claims_no_verify(id_token: str) -> Dict[str, Any]:
    if not id_token or id_token.count(".") < 2:
        return {}
    payload_b64 = id_token.split(".")[1]
    pad = "=" * ((4 - (len(payload_b64) % 4)) % 4)
    try:
        payload = base64.urlsafe_b64decode((payload_b64 + pad).encode("ascii"))
        return json.loads(payload.decode("utf-8"))
    except Exception:
        return {}


def _to_int(v: Any) -> int:
    try:
        return int(v)
    except (TypeError, ValueError):
        return 0


def _post_form(url: str, data: Dict[str, str], timeout: int = 30) -> Dict[str, Any]:
    body = urllib.parse.urlencode(data).encode("utf-8")
    req = urllib.request.Request(url, data=body, method="POST", headers={
        "Content-Type": "application/x-www-form-urlencoded",
        "Accept": "application/json",
    })
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            raw = resp.read()
            if resp.status != 200:
                raise RuntimeError(f"token exchange failed: {resp.status}: {raw.decode('utf-8', 'replace')}")
            return json.loads(raw.decode("utf-8"))
    except urllib.error.HTTPError as exc:
        raw = exc.read()
        raise RuntimeError(f"token exchange failed: {exc.code}: {raw.decode('utf-8', 'replace')}") from exc


@dataclass(frozen=True)
class OAuthStart:
    auth_url: str
    state: str
    code_verifier: str
    redirect_uri: str


def generate_oauth_url(*, redirect_uri: str = DEFAULT_REDIRECT_URI, scope: str = DEFAULT_SCOPE) -> OAuthStart:
    state = _random_state()
    code_verifier = _pkce_verifier()
    code_challenge = _sha256_b64url_no_pad(code_verifier)

    params = {
        "client_id": CLIENT_ID, "response_type": "code", "redirect_uri": redirect_uri,
        "scope": scope, "state": state, "code_challenge": code_challenge,
        "code_challenge_method": "S256", "prompt": "login",
        "id_token_add_organizations": "true", "codex_cli_simplified_flow": "true",
    }
    auth_url = f"{AUTH_URL}?{urllib.parse.urlencode(params)}"
    return OAuthStart(auth_url=auth_url, state=state, code_verifier=code_verifier, redirect_uri=redirect_uri)


def submit_callback_url(*, callback_url: str, expected_state: str, code_verifier: str,
                        redirect_uri: str = DEFAULT_REDIRECT_URI) -> str:
    cb = _parse_callback_url(callback_url)
    if cb["error"]:
        raise RuntimeError(f"oauth error: {cb['error']}: {cb['error_description']}".strip())
    if not cb["code"]:
        raise ValueError("callback url missing ?code=")
    if not cb["state"]:
        raise ValueError("callback url missing ?state=")
    if cb["state"] != expected_state:
        raise ValueError("state mismatch")

    token_resp = _post_form(TOKEN_URL, {
        "grant_type": "authorization_code", "client_id": CLIENT_ID,
        "code": cb["code"], "redirect_uri": redirect_uri, "code_verifier": code_verifier,
    })

    access_token = (token_resp.get("access_token") or "").strip()
    refresh_token = (token_resp.get("refresh_token") or "").strip()
    id_token = (token_resp.get("id_token") or "").strip()
    expires_in = _to_int(token_resp.get("expires_in"))

    claims = _jwt_claims_no_verify(id_token)
    email = str(claims.get("email") or "").strip()
    auth_claims = claims.get("https://api.openai.com/auth") or {}
    account_id = str(auth_claims.get("chatgpt_account_id") or "").strip()

    now = int(time.time())
    expired_rfc3339 = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime(now + max(expires_in, 0)))
    now_rfc3339 = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime(now))

    config = {
        "id_token": id_token, "access_token": access_token, "refresh_token": refresh_token,
        "account_id": account_id, "last_refresh": now_rfc3339, "email": email,
        "type": "codex", "expired": expired_rfc3339,
    }
    return json.dumps(config, ensure_ascii=False, separators=(",", ":"))


# ========== 上传 ==========

def upload_auth_file(filepath: str, upload_url: str, upload_token: str):
    """上传 auth 文件到 CLIProxyAPI Management API"""
    upload_url = upload_url.rstrip("/")
    url = f"{upload_url}/v0/management/auth-files"
    boundary = f"----WebKitFormBoundary{secrets.token_hex(8)}"
    filename = os.path.basename(filepath)
    with open(filepath, "rb") as f:
        file_data = f.read()
    body = (
        f"--{boundary}\r\n"
        f'Content-Disposition: form-data; name="file"; filename="{filename}"\r\n'
        f"Content-Type: application/json\r\n\r\n"
    ).encode() + file_data + f"\r\n--{boundary}--\r\n".encode()
    req = urllib.request.Request(url, data=body, method="POST", headers={
        "Authorization": f"Bearer {upload_token}",
        "Content-Type": f"multipart/form-data; boundary={boundary}",
        "Accept": "application/json, text/plain, */*",
    })
    with urllib.request.urlopen(req, timeout=15) as resp:
        return resp.status, resp.read().decode("utf-8", "replace")


# ========== 注册流程 ==========

def run(cfg: dict) -> str:
    s = requests.Session(impersonate="chrome")
    mail = create_email(cfg)
    email = mail["address"]
    log.info(f"邮箱: {email}")
    oauth = generate_oauth_url()
    s.get(oauth.auth_url)
    did = s.cookies.get("oai-did")
    log.info(f"oai-did: {did}")

    signup_body = f'{{"username":{{"value":"{email}","kind":"email"}},"screen_hint":"signup"}}'
    sen_req_body = f'{{"p":"","id":"{did}","flow":"authorize_continue"}}'
    sen_resp = requests.post("https://sentinel.openai.com/backend-api/sentinel/req",
                             headers={"origin": "https://sentinel.openai.com",
                                      "referer": "https://sentinel.openai.com/backend-api/sentinel/frame.html?sv=20260219f9f6",
                                      "content-type": "text/plain;charset=UTF-8"}, data=sen_req_body)
    log.info(f"sentinel: {sen_resp.status_code}")
    sen_token = sen_resp.json()["token"]
    sentinel = f'{{"p": "", "t": "", "c": "{sen_token}", "id": "{did}", "flow": "authorize_continue"}}'

    signup_resp = s.post("https://auth.openai.com/api/accounts/authorize/continue",
                         headers={"referer": "https://auth.openai.com/create-account", "accept": "application/json",
                                  "content-type": "application/json", "openai-sentinel-token": sentinel},
                         data=signup_body)
    log.info(f"signup: {signup_resp.status_code}")
    if signup_resp.status_code != 200:
        log.error(f"signup 失败: {signup_resp.text}")
        return None

    otp_resp = s.post("https://auth.openai.com/api/accounts/passwordless/send-otp",
                      headers={"referer": "https://auth.openai.com/create-account/password",
                               "accept": "application/json", "content-type": "application/json"})
    log.info(f"send-otp: {otp_resp.status_code}")
    if otp_resp.status_code != 200:
        log.error(f"send-otp 失败: {otp_resp.text}")
        return None

    code = get_oai_code(email, cfg)
    if not code:
        log.error("获取验证码超时")
        return None
    log.info(f"验证码: {code}")

    code_body = f'{{"code":"{code}"}}'
    code_resp = s.post("https://auth.openai.com/api/accounts/email-otp/validate",
                       headers={"referer": "https://auth.openai.com/email-verification", "accept": "application/json",
                                "content-type": "application/json"}, data=code_body)
    log.info(f"validate-otp: {code_resp.status_code}")

    create_account_body = '{"name":"Neo","birthdate":"2000-02-20"}'
    create_resp = s.post("https://auth.openai.com/api/accounts/create_account",
                         headers={"referer": "https://auth.openai.com/about-you", "accept": "application/json",
                                  "content-type": "application/json"}, data=create_account_body)
    log.info(f"create_account: {create_resp.status_code}")
    if create_resp.status_code != 200:
        log.error(f"创建账号失败: {create_resp.text}")
        return None

    auth = s.cookies.get("oai-client-auth-session")
    auth = json.loads(base64.b64decode(auth.split(".")[0]))
    workspace_id = auth["workspaces"][0]["id"]
    log.info(f"workspace: {workspace_id}")

    select_body = f'{{"workspace_id":"{workspace_id}"}}'
    select_resp = s.post("https://auth.openai.com/api/accounts/workspace/select",
                         headers={"referer": "https://auth.openai.com/sign-in-with-chatgpt/codex/consent",
                                  "content-type": "application/json"}, data=select_body)
    log.info(f"workspace/select: {select_resp.status_code}")

    continue_url = select_resp.json()["continue_url"]
    final_resp = s.get(continue_url, allow_redirects=False)
    final_resp = s.get(final_resp.headers.get("Location"), allow_redirects=False)
    final_resp = s.get(final_resp.headers.get("Location"), allow_redirects=False)
    cbk = final_resp.headers.get("Location")
    return submit_callback_url(callback_url=cbk, code_verifier=oauth.code_verifier,
                               redirect_uri=oauth.redirect_uri, expected_state=oauth.state)


# ========== 单个任务 ==========

def register_one(index: int, total: int, cfg: dict) -> bool:
    log.info(f"===== 第 {index}/{total} 个账号 =====")
    try:
        result = run(cfg)
        if not result:
            log.warning(f"[{index}/{total}] 失败: 未返回结果")
            return False

        data = json.loads(result)
        email = data.get("email", f"unknown_{index}")
        email = email.strip().strip("'\"")
        filename = f"files/{email}.json"
        with open(filename, "w", encoding="utf-8") as f:
            json.dump(data, f, ensure_ascii=False, indent=2)
        log.info(f"[{index}/{total}] 成功, 已保存到 {filename}")

        if cfg["upload"]:
            try:
                status, resp = upload_auth_file(filename, cfg["upload_url"], cfg["upload_token"])
                log.info(f"[{index}/{total}] 上传结果: {status} {resp}")
            except Exception as ue:
                log.error(f"[{index}/{total}] 上传失败: {ue}")
        return True

    except Exception as e:
        log.error(f"[{index}/{total}] 失败: {e}")
        return False


# ========== 入口 ==========

def _env_override(cfg: dict):
    """环境变量优先覆盖 config.yaml 的值"""
    env_map = {
        "count":        ("COUNT",        int),
        "max_workers":  ("MAX_WORKERS",  int),
        "upload":       ("UPLOAD",       int),
        "upload_url":   ("UPLOAD_URL",   str),
        "upload_token": ("UPLOAD_TOKEN", str),
        "worker_url":   ("WORKER_URL",   str),
        "own_domain":   ("OWN_DOMAIN",   str),
        "log_to_file":  ("LOG_TO_FILE",  int),
    }
    for key, (env_name, cast) in env_map.items():
        val = os.environ.get(env_name)
        if val is not None and val != "":
            cfg[key] = cast(val)
    return cfg


def _run_batch(cfg: dict):
    """执行一轮注册"""
    count = cfg.get("count", 1)
    max_workers = cfg.get("max_workers", 1)

    log.info(f"开始注册 {count} 个账号, 并发数 {max_workers}")

    if max_workers <= 1:
        success = 0
        for i in range(1, count + 1):
            if register_one(i, count, cfg):
                success += 1
    else:
        success = 0
        futures = []
        with ThreadPoolExecutor(max_workers=max_workers, thread_name_prefix="worker") as pool:
            for i in range(1, count + 1):
                futures.append(pool.submit(register_one, i, count, cfg))
            for fut in as_completed(futures):
                if fut.result():
                    success += 1

    log.info(f"完成: 成功 {success}/{count}")


def main():
    config_path = os.path.join(os.path.dirname(os.path.abspath(__file__)), "config.yaml")
    if os.path.exists(config_path):
        with open(config_path, "r", encoding="utf-8") as f:
            cfg = yaml.safe_load(f) or {}
    else:
        cfg = {}

    cfg = _env_override(cfg)

    setup_logging(cfg.get("log_to_file", 0))
    os.makedirs("files", exist_ok=True)

    # LOOP_INTERVAL > 0 时持续循环运行（单位：秒），适合容器常驻
    loop_interval = int(os.environ.get("LOOP_INTERVAL", "0"))

    if loop_interval > 0:
        log.info(f"容器模式: 每 {loop_interval} 秒执行一轮")
        while True:
            try:
                _run_batch(cfg)
            except Exception as e:
                log.error(f"本轮执行异常: {e}")
            log.info(f"等待 {loop_interval} 秒后开始下一轮...")
            time.sleep(loop_interval)
    else:
        _run_batch(cfg)


if __name__ == "__main__":
    main()
