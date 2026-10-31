#!/usr/bin/env bash
# Create a Gitee Release and attach the built artifacts.
# Usage: GITEE_TOKEN=xxxx ./scripts/gitee-release.sh v0.2.0
set -euo pipefail
cd "$(dirname "$0")/.."

TAG="${1:-v0.2.0}"
OWNER="hgy_imust"
REPO="mysql_tools"
API="https://gitee.com/api/v5"
TOKEN="${GITEE_TOKEN:-}"
NOTES="docs/release-notes-v0.2.0.md"
ASSETS=(
  "dist/release/NavMini-0.2.0-macos-arm64.dmg"
  "dist/release/NavMini-0.2.0-windows-amd64.exe"
  "dist/release/NavMini-0.2.0-windows-amd64.zip"
  "dist/release/SHA256SUMS"
)

if [ -z "$TOKEN" ]; then
  echo "缺少 GITEE_TOKEN。请在 Gitee → 设置 → 私人令牌 生成（需要 projects 权限），然后：" >&2
  echo "  GITEE_TOKEN=xxxx ./scripts/gitee-release.sh $TAG" >&2
  exit 2
fi
if [ ! -f "$NOTES" ]; then echo "缺少 $NOTES" >&2; exit 2; fi
for f in "${ASSETS[@]}"; do
  [ -f "$f" ] || { echo "缺少构建产物：$f" >&2; exit 2; }
done

echo "→ 校验 token"
curl -fsS "$API/user?access_token=$TOKEN" >/dev/null || {
  echo "token 无效或没有权限" >&2; exit 1;
}

echo "→ 查找已有 release ($TAG)"
RID=$(curl -sS "$API/repos/$OWNER/$REPO/releases?access_token=$TOKEN&per_page=100" | python3 -c '
import json,sys
tag=sys.argv[1]
try: data=json.load(sys.stdin)
except Exception: data=[]
for r in data if isinstance(data,list) else []:
    if r.get("tag_name")==tag:
        print(r["id"]); break
' "$TAG")

if [ -z "$RID" ]; then
  echo "→ 创建 release $TAG"
  python3 - "$TAG" "$NOTES" > /tmp/gitee-release-body.json <<'PY'
import json,sys
tag, notes = sys.argv[1], open(sys.argv[2], encoding='utf-8').read()
print(json.dumps({
    "tag_name": tag,
    "name": "NavMini " + tag,
    "body": notes,
    "prerelease": False,
    "target_commitish": "master",
}, ensure_ascii=False))
PY
  RESP=$(curl -sS -X POST "$API/repos/$OWNER/$REPO/releases?access_token=$TOKEN" \
    -H 'Content-Type: application/json' --data-binary @/tmp/gitee-release-body.json)
  RID=$(printf '%s' "$RESP" | python3 -c 'import json,sys; d=json.load(sys.stdin); print(d.get("id",""))')
  if [ -z "$RID" ]; then echo "创建失败：$RESP" >&2; exit 1; fi
  echo "  release id=$RID"
else
  echo "  已存在 release id=$RID"
fi

for f in "${ASSETS[@]}"; do
  echo "→ 上传 $(basename "$f")"
  RESP=$(curl -sS -X POST "$API/repos/$OWNER/$REPO/releases/$RID/attach_files?access_token=$TOKEN" \
    -F "file=@$f")
  printf '%s' "$RESP" | python3 -c '
import json,sys
try: d=json.load(sys.stdin)
except Exception: print("  响应解析失败"); raise SystemExit(0)
if d.get("browser_download_url"): print("  ", d["browser_download_url"])
else: print("  ", d)
'
done

echo "完成：https://gitee.com/$OWNER/$REPO/releases/tag/$TAG"
