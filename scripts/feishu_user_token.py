#!/usr/bin/env python3
"""
feishu_user_token.py — Get a Feishu user_access_token via local OAuth, no Go CLI needed.

You need a user_access_token (not just a tenant token) to scan YOUR OWN personal
Drive with feishu_biggest.py. This runs the standard Feishu OAuth code flow locally:

  1. Starts a tiny localhost server on http://localhost:8765/callback
  2. Opens the Feishu authorize page in your browser
  3. You scan/approve -> Feishu redirects back with a ?code=...
  4. Exchanges the code for a user_access_token and prints it.

Prereqs:
  - App at open.feishu.cn with redirect URI  http://localhost:8765/callback
    added under "Security settings" / "Redirect URLs".
  - Scope drive:drive:readonly (or drive:drive) granted.
  - export FEISHU_APP_ID=... FEISHU_APP_SECRET=...

Usage:
  python3 feishu_user_token.py
  # then:  export FEISHU_USER_TOKEN="<printed token>"
  #        python3 feishu_biggest.py --top 50

Stdlib only.
"""

import http.server
import json
import os
import sys
import threading
import urllib.parse
import urllib.request
import webbrowser

BASE = os.environ.get("FEISHU_BASE_URL", "https://open.feishu.cn")
REDIRECT = "http://localhost:8765/callback"
SCOPE = os.environ.get("FEISHU_SCOPE", "drive:drive:readonly")

_code_holder = {}


class Handler(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        q = urllib.parse.urlparse(self.path)
        if q.path != "/callback":
            self.send_response(404); self.end_headers(); return
        params = urllib.parse.parse_qs(q.query)
        _code_holder["code"] = (params.get("code") or [""])[0]
        _code_holder["error"] = (params.get("error") or [""])[0]
        self.send_response(200)
        self.send_header("Content-Type", "text/html; charset=utf-8")
        self.end_headers()
        self.wfile.write("<h2>Done. You can close this tab and return to the terminal.</h2>".encode())

    def log_message(self, *a):
        pass


def post_json(url, payload, headers=None):
    body = json.dumps(payload).encode()
    h = {"Content-Type": "application/json; charset=utf-8"}
    h.update(headers or {})
    req = urllib.request.Request(url, data=body, headers=h, method="POST")
    with urllib.request.urlopen(req, timeout=30) as r:
        return json.loads(r.read())


def main():
    app_id = os.environ.get("FEISHU_APP_ID")
    app_secret = os.environ.get("FEISHU_APP_SECRET")
    if not app_id or not app_secret:
        sys.exit("Set FEISHU_APP_ID and FEISHU_APP_SECRET first.")

    # app_access_token (needed to exchange the oauth code)
    app_tok = post_json(f"{BASE}/open-apis/auth/v3/app_access_token/internal",
                        {"app_id": app_id, "app_secret": app_secret})
    if app_tok.get("code") != 0:
        sys.exit(f"app_access_token failed: {app_tok.get('msg')}")
    app_access_token = app_tok["app_access_token"]

    authorize = (f"{BASE}/open-apis/authen/v1/authorize?"
                 + urllib.parse.urlencode({
                     "app_id": app_id,
                     "redirect_uri": REDIRECT,
                     "scope": SCOPE,
                     "state": "feishu_biggest",
                 }))

    srv = http.server.HTTPServer(("localhost", 8765), Handler)
    t = threading.Thread(target=srv.handle_request)  # serve exactly one request
    t.start()
    print(f"Opening browser for authorization...\nIf it doesn't open, visit:\n{authorize}\n")
    try:
        webbrowser.open(authorize)
    except Exception:
        pass
    t.join(timeout=300)
    srv.server_close()

    if _code_holder.get("error"):
        sys.exit(f"authorize error: {_code_holder['error']}")
    code = _code_holder.get("code")
    if not code:
        sys.exit("no code received (timed out). Check that the redirect URI "
                 f"{REDIRECT} is registered in your app's security settings.")

    tok = post_json(f"{BASE}/open-apis/authen/v1/oidc/access_token",
                    {"grant_type": "authorization_code", "code": code},
                    headers={"Authorization": f"Bearer {app_access_token}"})
    if tok.get("code") != 0:
        sys.exit(f"token exchange failed: code={tok.get('code')} msg={tok.get('msg')}")
    data = tok.get("data") or {}
    uat = data.get("access_token")
    if not uat:
        sys.exit(f"no access_token in response: {tok}")
    print("\n=== user_access_token (valid ~2h) ===")
    print(uat)
    print("\nRun:")
    print(f'  export FEISHU_USER_TOKEN="{uat}"')
    print("  python3 feishu_biggest.py --top 50")


if __name__ == "__main__":
    main()
