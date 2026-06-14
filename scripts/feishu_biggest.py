#!/usr/bin/env python3
"""
feishu_biggest.py — Find the LARGEST files in your Feishu Drive, sorted by size.

Why this exists:
  Feishu's web UI hides per-file size / sort-by-size behind a paid plan, and the
  open Drive "list files" API returns NO size field and only supports sorting by
  EditedTime / CreatedTime. This script reconstructs the missing capability from
  APIs every account already has:

    1. Recursively walk all folders via  GET /drive/v1/files
    2. Collect every item of type "file" (docs/sheets/bitables have no size and
       count ~0 toward storage, so they are skipped for the size ranking).
    3. For each file, read its byte size WITHOUT downloading it:
         a. GET /drive/v1/medias/batch_get_tmp_download_url  -> temp URL
         b. HTTP HEAD (or 1-byte Range GET) the temp URL -> Content-Length
    4. Sort descending and print a table.

READ-ONLY BY DESIGN. This script never deletes, moves, trashes, uploads, or
empties anything. It only issues GET / HEAD requests. The largest entries it
surfaces are the ones worth reviewing (and manually deleting yourself).

Auth: uses a tenant_access_token built from your app credentials.
  export FEISHU_APP_ID="cli_xxx"
  export FEISHU_APP_SECRET="xxx"
Required app scope (read is enough): drive:drive:readonly  (or drive:drive)

Usage:
  python3 feishu_biggest.py                  # scan from My Space root
  python3 feishu_biggest.py --folder TOKEN   # scan a specific folder subtree
  python3 feishu_biggest.py --top 50         # show top 50 (default 30)
  python3 feishu_biggest.py --json out.json  # also dump full results as JSON
  python3 feishu_biggest.py --user-token T   # use a user_access_token instead
                                             # (needed to see YOUR personal space;
                                             #  tenant token only sees app-owned files)

Stdlib only. No pip installs.
"""

import argparse
import json
import os
import sys
import time
import urllib.error
import urllib.parse
import urllib.request

BASE = os.environ.get("FEISHU_BASE_URL", "https://open.feishu.cn")
TOKEN_URL = f"{BASE}/open-apis/auth/v3/tenant_access_token/internal"
LIST_URL = f"{BASE}/open-apis/drive/v1/files"
TMP_URL = f"{BASE}/open-apis/drive/v1/medias/batch_get_tmp_download_url"

# These types carry no byte size in Feishu and effectively cost 0 storage.
ZERO_SIZE_TYPES = {"docx", "doc", "sheet", "bitable", "mindnote", "slides", "shortcut"}


def _http(method, url, headers=None, data=None, timeout=30, retries=5):
    """Minimal HTTP with exponential backoff on 429/5xx (per CLAUDE.md API rules)."""
    headers = dict(headers or {})
    body = None
    if data is not None:
        body = json.dumps(data).encode("utf-8")
        headers.setdefault("Content-Type", "application/json; charset=utf-8")
    delay = 1.0
    last_err = None
    for attempt in range(retries):
        req = urllib.request.Request(url, data=body, headers=headers, method=method)
        try:
            with urllib.request.urlopen(req, timeout=timeout) as resp:
                return resp.status, dict(resp.headers), resp.read()
        except urllib.error.HTTPError as e:
            status = e.code
            payload = e.read()
            if status == 429 or 500 <= status < 600:
                last_err = (status, payload)
                time.sleep(delay)
                delay = min(delay * 2, 60)
                continue
            return status, dict(e.headers or {}), payload
        except urllib.error.URLError as e:
            last_err = (None, str(e).encode())
            time.sleep(delay)
            delay = min(delay * 2, 60)
            continue
    status = last_err[0] if last_err else None
    payload = last_err[1] if last_err else b"unknown error"
    return status or 0, {}, payload


def get_tenant_token(app_id, app_secret):
    status, _, body = _http("POST", TOKEN_URL, data={"app_id": app_id, "app_secret": app_secret})
    obj = json.loads(body or b"{}")
    if obj.get("code") != 0 or not obj.get("tenant_access_token"):
        sys.exit(f"[auth] failed to get tenant_access_token: code={obj.get('code')} msg={obj.get('msg')}")
    return obj["tenant_access_token"]


def list_folder(token, folder_token, page_token=""):
    params = {"page_size": "200"}
    if folder_token:
        params["folder_token"] = folder_token
    if page_token:
        params["page_token"] = page_token
    url = LIST_URL + "?" + urllib.parse.urlencode(params)
    status, _, body = _http("GET", url, headers={"Authorization": f"Bearer {token}"})
    obj = json.loads(body or b"{}")
    if obj.get("code") != 0:
        # 1062009 etc. -> typically a scope/permission issue.
        sys.stderr.write(f"[list] folder={folder_token or 'ROOT'} code={obj.get('code')} msg={obj.get('msg')}\n")
        return [], "", False
    data = obj.get("data") or {}
    return data.get("files") or [], data.get("next_page_token") or "", bool(data.get("has_more"))


def walk(token, root_folder):
    """Recursively yield (file_item, relpath) for every type=='file' item."""
    # stack of (folder_token, relpath_prefix)
    stack = [(root_folder, "")]
    seen_folders = set()
    while stack:
        folder_token, prefix = stack.pop()
        if folder_token in seen_folders:
            continue
        seen_folders.add(folder_token)
        page = ""
        while True:
            files, nxt, has_more = list_folder(token, folder_token, page)
            for f in files:
                name = f.get("name", "")
                rel = f"{prefix}/{name}" if prefix else name
                ftype = f.get("type", "")
                if ftype == "folder":
                    stack.append((f.get("token"), rel))
                else:
                    yield f, rel
            if has_more and nxt:
                page = nxt
            else:
                break


def get_size(token, file_token):
    """Return byte size of an uploaded file without downloading its body.
    Step 1: get a temp download URL. Step 2: HEAD it for Content-Length.
    Returns int bytes, or None if size can't be determined."""
    url = TMP_URL + "?" + urllib.parse.urlencode([("file_tokens", file_token)])
    status, _, body = _http("GET", url, headers={"Authorization": f"Bearer {token}"})
    obj = json.loads(body or b"{}")
    if obj.get("code") != 0:
        return None, obj.get("msg")
    urls = (obj.get("data") or {}).get("tmp_download_urls") or []
    if not urls:
        return None, "no tmp url (not a downloadable media file)"
    tmp = urls[0].get("tmp_download_url")
    # Some responses already include size:
    if "size" in urls[0]:
        try:
            return int(urls[0]["size"]), None
        except (TypeError, ValueError):
            pass
    if not tmp:
        return None, "empty tmp url"
    # HEAD first (no body). Fall back to a 1-byte ranged GET if HEAD is unsupported.
    st, hdrs, _ = _http("HEAD", tmp)
    cl = hdrs.get("Content-Length") or hdrs.get("content-length")
    if cl and cl.isdigit():
        return int(cl), None
    st, hdrs, _ = _http("GET", tmp, headers={"Range": "bytes=0-0"})
    cr = hdrs.get("Content-Range") or hdrs.get("content-range")  # e.g. "bytes 0-0/12345"
    if cr and "/" in cr:
        total = cr.rsplit("/", 1)[-1]
        if total.isdigit():
            return int(total), None
    cl = hdrs.get("Content-Length") or hdrs.get("content-length")
    if cl and cl.isdigit():
        return int(cl), None
    return None, "no content-length"


def human(n):
    if n is None:
        return "      ?"
    for unit in ("B", "KB", "MB", "GB", "TB"):
        if n < 1024 or unit == "TB":
            return f"{n:6.1f}{unit}" if unit != "B" else f"{n:6d}B"
        n /= 1024.0


def main():
    ap = argparse.ArgumentParser(description="Find largest Feishu Drive files (read-only).")
    ap.add_argument("--folder", default="", help="folder token to start from (default: My Space root)")
    ap.add_argument("--top", type=int, default=30, help="how many largest to print (default 30)")
    ap.add_argument("--json", default="", help="write full results to this JSON file")
    ap.add_argument("--user-token", default=os.environ.get("FEISHU_USER_TOKEN", ""),
                    help="user_access_token to scan YOUR personal space (else tenant token)")
    args = ap.parse_args()

    if args.user_token:
        token = args.user_token
        print("[auth] using provided user_access_token", file=sys.stderr)
    else:
        app_id = os.environ.get("FEISHU_APP_ID")
        app_secret = os.environ.get("FEISHU_APP_SECRET")
        if not app_id or not app_secret:
            sys.exit("Set FEISHU_APP_ID and FEISHU_APP_SECRET (or pass --user-token). "
                     "Note: a tenant token only sees app-owned files; to scan your own "
                     "personal Drive you usually need --user-token from `feishu-cli auth login`.")
        token = get_tenant_token(app_id, app_secret)
        print("[auth] tenant_access_token acquired", file=sys.stderr)

    print(f"[scan] walking folders from {args.folder or 'ROOT (My Space)'} ...", file=sys.stderr)
    files = list(walk(token, args.folder))
    real_files = [(f, rel) for (f, rel) in files if f.get("type") not in ZERO_SIZE_TYPES]
    print(f"[scan] found {len(files)} items, {len(real_files)} uploaded files to size", file=sys.stderr)

    results = []
    for i, (f, rel) in enumerate(real_files, 1):
        size, err = get_size(token, f.get("token"))
        results.append({
            "size": size, "name": f.get("name"), "type": f.get("type"),
            "path": rel, "token": f.get("token"), "url": f.get("url"),
            "modified_time": f.get("modified_time"), "error": err,
        })
        if i % 25 == 0:
            print(f"[size] {i}/{len(real_files)} sized ...", file=sys.stderr)

    results.sort(key=lambda r: (r["size"] is not None, r["size"] or 0), reverse=True)

    print(f"\n{'SIZE':>9}  {'TYPE':<8}  NAME / PATH")
    print("-" * 78)
    for r in results[: args.top]:
        note = "" if not r["error"] else f"  ({r['error']})"
        print(f"{human(r['size']):>9}  {r['type']:<8}  {r['path']}{note}")
    if not results:
        print("(no uploaded files found — your storage may be docs-only, or the token "
              "lacks access to the space. Try --user-token.)")

    if args.json:
        with open(args.json, "w") as fh:
            json.dump(results, fh, ensure_ascii=False, indent=2)
        print(f"\n[out] full results written to {args.json}", file=sys.stderr)


if __name__ == "__main__":
    main()
