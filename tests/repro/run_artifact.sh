#!/usr/bin/env bash
# proactive-workbench Bug 复现（Notion 开发看板 #13）
#
# 从某个 commit 的 GitHub Actions 产物下载 Linux 包、解压、按 CI 同样的方式跑一遍，
# 并把 stdout/stderr、退出码、环境信息和 run URL 打进一个目录，再打成可挂到 issue 的 tar.gz。
#
# 对应 workflow：.github/workflows/linux-build.yml
#   - 产物名：proactive-workbench-0.1.0-linux-amd64.tar.gz
#   - 包内二进制：proactive-workbench-0.1.0-linux-amd64/workbench
#   - CI 验收命令：workbench version / workbench tools
#   - 带伞演示（与 workflow 一致）：
#       workbench demo --weather=rain  --tz=Asia/Shanghai --now=2026-10-09T13:00:00+08:00
#       workbench demo --weather=clear --tz=Asia/Shanghai --now=2026-10-09T13:00:00+08:00
#
# 用法：
#   ./tests/repro/run_artifact.sh <commit-sha>
#   ./tests/repro/run_artifact.sh <commit-sha> --workflow "Linux build" --artifact NAME
#   ./tests/repro/run_artifact.sh <commit-sha> -- demo --weather=cloudy
#
# 退出码：0 = 下载并跑完且命令都成功；1 = 产物已跑但有命令失败（仍会打出打包路径）；
#         2 = 用法 / 缺 gh / 该 commit 没有 run / 没有产物等，无法复现
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

DEFAULT_WORKFLOW="Linux build"
DEFAULT_ARTIFACT="proactive-workbench-0.1.0-linux-amd64.tar.gz"
DEMO_NOW="2026-10-09T13:00:00+08:00"
DEMO_TZ="Asia/Shanghai"

usage() {
  cat <<EOF
Usage: $0 <commit-sha> [options] [-- extra workbench args]

Download the Linux CI artifact for a commit, unpack it, run the same
commands CI uses, and pack logs for a bug report.

Options:
  --workflow NAME   GitHub Actions workflow name or file (default: ${DEFAULT_WORKFLOW})
  --artifact NAME   Artifact name (default: ${DEFAULT_ARTIFACT})
  --run-id ID       Skip lookup; download this run id
  --out DIR         Output directory (default: tests/repro/.repro/<sha>-<utc>)
  --repo OWNER/NAME Override repo (default: gh repo view)
  -h, --help        Show this help

Anything after -- is passed to the unpacked ./workbench as an extra command
(in addition to version / tools / the two CI demos).

Examples:
  $0 c6d71d3
  $0 c6d71d3eeb137be20a7f18592e4c94c072d637cc --workflow linux-build.yml
  $0 HEAD -- plan "bring an umbrella tomorrow 8am" --weather=rain
EOF
}

die() {
  echo "ERROR: $*" >&2
  exit 2
}

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "required command not found: $1"
}

# Capture stdout; on failure, surface the raw stderr (and leftover stdout).
run_capture() {
  local dest="$1"
  shift
  local err
  err="$(mktemp)"
  if ! "$@" >"$dest" 2>"$err"; then
    local raw
    raw="$(cat "$err"; echo '----- stdout -----'; cat "$dest")"
    rm -f "$err"
    printf '%s\n' "$raw"
    return 1
  fi
  rm -f "$err"
  return 0
}

# ---------- parse args ----------
COMMIT_ARG=""
WORKFLOW="$DEFAULT_WORKFLOW"
ARTIFACT_NAME=""
RUN_ID=""
OUT_DIR=""
REPO=""
EXTRA_ARGS=()
SEEN_DASHDASH=0

if [ "$#" -eq 0 ]; then
  usage >&2
  die "commit SHA is required"
fi

while [ "$#" -gt 0 ]; do
  if [ "$SEEN_DASHDASH" -eq 1 ]; then
    EXTRA_ARGS+=("$1")
    shift
    continue
  fi
  case "$1" in
    -h|--help)
      usage
      exit 0
      ;;
    --workflow)
      [ "$#" -ge 2 ] || die "--workflow requires a value"
      WORKFLOW="$2"
      shift 2
      ;;
    --artifact)
      [ "$#" -ge 2 ] || die "--artifact requires a value"
      ARTIFACT_NAME="$2"
      shift 2
      ;;
    --run-id)
      [ "$#" -ge 2 ] || die "--run-id requires a value"
      RUN_ID="$2"
      shift 2
      ;;
    --out)
      [ "$#" -ge 2 ] || die "--out requires a value"
      OUT_DIR="$2"
      shift 2
      ;;
    --repo)
      [ "$#" -ge 2 ] || die "--repo requires a value"
      REPO="$2"
      shift 2
      ;;
    --)
      SEEN_DASHDASH=1
      shift
      ;;
    -*)
      die "unknown option: $1 (see --help)"
      ;;
    *)
      if [ -n "$COMMIT_ARG" ]; then
        die "unexpected extra argument: $1 (put workbench args after --)"
      fi
      COMMIT_ARG="$1"
      shift
      ;;
  esac
done

[ -n "$COMMIT_ARG" ] || die "commit SHA is required"

# ---------- host / tools ----------
OS_NAME="$(uname -s)"
if [ "$OS_NAME" != "Linux" ]; then
  die "this script only runs on Linux (uname -s reported: ${OS_NAME}). Product artifact is linux-amd64."
fi

need_cmd gh
need_cmd jq
need_cmd tar
need_cmd find
need_cmd date

AUTH_OUT=""
if ! AUTH_OUT="$(gh auth status 2>&1)"; then
  die "gh is not logged in (run: gh auth login). Raw error:
${AUTH_OUT}"
fi

if [ -z "$REPO" ]; then
  REPO_OUT=""
  if ! REPO_OUT="$(gh repo view --json nameWithOwner --jq '.nameWithOwner' 2>&1)"; then
    die "could not detect GitHub repo (pass --repo OWNER/NAME). Raw error:
${REPO_OUT}"
  fi
  REPO="$REPO_OUT"
fi

# ---------- resolve commit ----------
FULL_SHA=""
if git -C "$REPO_ROOT" rev-parse --verify "${COMMIT_ARG}^{commit}" >/dev/null 2>&1; then
  FULL_SHA="$(git -C "$REPO_ROOT" rev-parse --verify "${COMMIT_ARG}^{commit}")"
else
  RESOLVE_OUT=""
  if ! RESOLVE_OUT="$(gh api "repos/${REPO}/commits/${COMMIT_ARG}" --jq '.sha' 2>&1)"; then
    die "could not resolve commit ${COMMIT_ARG} in ${REPO}. Raw error:
${RESOLVE_OUT}"
  fi
  FULL_SHA="$RESOLVE_OUT"
fi
SHORT_SHA="${FULL_SHA:0:12}"

STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
if [ -z "$OUT_DIR" ]; then
  OUT_DIR="${SCRIPT_DIR}/.repro/${SHORT_SHA}-${STAMP}"
fi
mkdir -p "$OUT_DIR"
OUT_DIR="$(cd "$OUT_DIR" && pwd)"
LOG_DIR="${OUT_DIR}/logs"
DOWNLOAD_DIR="${OUT_DIR}/download"
UNPACK_DIR="${OUT_DIR}/unpacked"
mkdir -p "$LOG_DIR" "$DOWNLOAD_DIR" "$UNPACK_DIR"

ENV_FILE="${OUT_DIR}/environment.txt"
SUMMARY="${OUT_DIR}/summary.txt"
RUN_JSON="${OUT_DIR}/run.json"
COMMANDS="${OUT_DIR}/commands.tsv"

{
  echo "os=$(uname -s)"
  echo "kernel=$(uname -r)"
  echo "arch=$(uname -m)"
  if [ -r /etc/os-release ]; then
    # shellcheck disable=SC1091
    . /etc/os-release
    echo "distro=${PRETTY_NAME:-unknown}"
  else
    echo "distro=unknown"
  fi
  echo "hostname=$(hostname)"
  echo "utc_start=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  echo "local_start=$(date '+%Y-%m-%dT%H:%M:%S %Z')"
  echo "commit=${FULL_SHA}"
  echo "commit_arg=${COMMIT_ARG}"
  echo "repo=${REPO}"
  echo "workflow=${WORKFLOW}"
  echo "gh=$(gh --version | head -n 1)"
} >"$ENV_FILE"

echo "== proactive-workbench repro =="
echo "repo:     $REPO"
echo "commit:   $FULL_SHA"
echo "workflow: $WORKFLOW"
echo "out:      $OUT_DIR"
echo

# ---------- find Actions run ----------
if [ -z "$RUN_ID" ]; then
  LIST_RAW=""
  if ! LIST_RAW="$(run_capture "${OUT_DIR}/runs.json" gh run list --repo "$REPO" --commit "$FULL_SHA" --workflow "$WORKFLOW" --limit 20 \
      --json databaseId,url,conclusion,status,displayTitle,workflowName,event,createdAt,updatedAt,headSha)"; then
    die "gh run list failed for commit ${FULL_SHA} workflow ${WORKFLOW}. Raw error:
${LIST_RAW}"
  fi
  if [ "$(jq -r 'if type == "array" and length == 0 then "true" else "false" end' "${OUT_DIR}/runs.json")" = "true" ]; then
    ALL_ERR=""
    ALL_ERR="$(gh run list --repo "$REPO" --commit "$FULL_SHA" --limit 20 \
      --json databaseId,url,conclusion,status,workflowName,event,createdAt 2>/dev/null || true)"
    die "no GitHub Actions run for commit ${FULL_SHA} in workflow ${WORKFLOW} (repo ${REPO}).
Other runs on this commit: ${ALL_ERR:-none}"
  fi

  PICK_JQ='(map(select(.conclusion == "success")) | sort_by(.createdAt) | reverse | .[0]) // (map(select(.status == "completed")) | sort_by(.createdAt) | reverse | .[0]) // .[0]'
  if ! jq -e "$PICK_JQ" "${OUT_DIR}/runs.json" >"$RUN_JSON"; then
    die "failed to pick a run from gh run list for commit ${FULL_SHA}. Raw list:
$(cat "${OUT_DIR}/runs.json")"
  fi
  RUN_ID="$(jq -r '.databaseId' "$RUN_JSON")"
  RUN_URL="$(jq -r '.url' "$RUN_JSON")"
  RUN_CONCLUSION="$(jq -r '.conclusion // ""' "$RUN_JSON")"
else
  VIEW_RAW=""
  if ! VIEW_RAW="$(run_capture "$RUN_JSON" gh run view "$RUN_ID" --repo "$REPO" \
      --json databaseId,url,conclusion,status,displayTitle,workflowName,event,createdAt,updatedAt,headSha)"; then
    die "gh run view ${RUN_ID} failed. Raw error:
${VIEW_RAW}"
  fi
  RUN_URL="$(jq -r '.url' "$RUN_JSON")"
  RUN_CONCLUSION="$(jq -r '.conclusion // ""' "$RUN_JSON")"
fi

{
  echo "run_id=${RUN_ID}"
  echo "run_url=${RUN_URL}"
  echo "run_conclusion=${RUN_CONCLUSION}"
} >>"$ENV_FILE"

echo "run:      $RUN_URL"
echo "run id:   $RUN_ID  conclusion=${RUN_CONCLUSION}"

# ---------- list / pick artifact ----------
ART_RAW=""
if ! ART_RAW="$(run_capture "${OUT_DIR}/artifacts.json" gh api "repos/${REPO}/actions/runs/${RUN_ID}/artifacts")"; then
  die "failed to list artifacts for run ${RUN_ID}. Raw error:
${ART_RAW}"
fi

ART_COUNT="$(jq '.artifacts | length' "${OUT_DIR}/artifacts.json")"
if [ "$ART_COUNT" -eq 0 ]; then
  die "run ${RUN_ID} (${RUN_URL}) has no artifacts (conclusion=${RUN_CONCLUSION})."
fi

if [ -z "$ARTIFACT_NAME" ]; then
  HAS_DEFAULT="$(jq -r --arg n "$DEFAULT_ARTIFACT" '[.artifacts[].name] | index($n) != null' "${OUT_DIR}/artifacts.json")"
  if [ "$HAS_DEFAULT" = "true" ]; then
    ARTIFACT_NAME="$DEFAULT_ARTIFACT"
  elif [ "$ART_COUNT" -eq 1 ]; then
    ARTIFACT_NAME="$(jq -r '.artifacts[0].name' "${OUT_DIR}/artifacts.json")"
  else
    NAMES="$(jq -r '.artifacts[].name' "${OUT_DIR}/artifacts.json")"
    die "run ${RUN_ID} has multiple artifacts and default ${DEFAULT_ARTIFACT} was not among them. Pass --artifact NAME.
Available:
${NAMES}"
  fi
fi

ART_MATCH="$(jq --arg n "$ARTIFACT_NAME" '[.artifacts[] | select(.name == $n)] | length' "${OUT_DIR}/artifacts.json")"
if [ "$ART_MATCH" = "0" ]; then
  NAMES="$(jq -r '.artifacts[].name' "${OUT_DIR}/artifacts.json")"
  die "artifact ${ARTIFACT_NAME} not found on run ${RUN_ID}. Available:
${NAMES}"
fi
EXPIRED="$(jq -r --arg n "$ARTIFACT_NAME" '[.artifacts[] | select(.name == $n) | .expired] | first' "${OUT_DIR}/artifacts.json")"
if [ "$EXPIRED" = "true" ]; then
  die "artifact ${ARTIFACT_NAME} on run ${RUN_ID} has expired (retention is 30 days on Linux build)."
fi

echo "artifact: $ARTIFACT_NAME"
echo "artifact=${ARTIFACT_NAME}" >>"$ENV_FILE"

# ---------- download ----------
DL_ERR=""
if ! DL_ERR="$(gh run download "$RUN_ID" --repo "$REPO" --name "$ARTIFACT_NAME" --dir "$DOWNLOAD_DIR" 2>&1)"; then
  die "gh run download failed for run ${RUN_ID} artifact ${ARTIFACT_NAME}. Raw error:
${DL_ERR}"
fi
printf '%s\n' "$DL_ERR" >"${LOG_DIR}/gh-download.log"
echo "$DL_ERR"

# ---------- find tarball ----------
mapfile -t TARBALLS < <(find "$DOWNLOAD_DIR" -type f \( -name '*.tar.gz' -o -name '*.tgz' \) ! -name '*.sha256' | sort)
if [ "${#TARBALLS[@]}" -eq 0 ]; then
  TREE="$(find "$DOWNLOAD_DIR" -print | sed 's/^/  /')"
  die "downloaded artifact ${ARTIFACT_NAME} but no .tar.gz found under ${DOWNLOAD_DIR}. Contents:
${TREE}"
fi
TARBALL="${TARBALLS[0]}"
echo "tarball:  $TARBALL"
echo "tarball=${TARBALL}" >>"$ENV_FILE"

SHA_FILE=""
if [ -f "${TARBALL}.sha256" ]; then
  SHA_FILE="${TARBALL}.sha256"
else
  mapfile -t SHA_CANDIDATES < <(find "$DOWNLOAD_DIR" -type f -name '*.sha256' | sort)
  if [ "${#SHA_CANDIDATES[@]}" -eq 1 ]; then
    SHA_FILE="${SHA_CANDIDATES[0]}"
  fi
fi
if [ -n "$SHA_FILE" ] && command -v sha256sum >/dev/null 2>&1; then
  echo "sha256:   $SHA_FILE"
  # package-linux.sh writes `sha256sum "$TARBALL"`, so the recorded path is the
  # CI absolute path. Compare hashes, not paths.
  expected="$(awk '{print $1}' "$SHA_FILE" | head -n 1)"
  actual="$(sha256sum "$TARBALL" | awk '{print $1}')"
  {
    echo "sha_file=$SHA_FILE"
    echo "expected=$expected"
    echo "actual=$actual"
  } >"${LOG_DIR}/sha256.log"
  if [ -z "$expected" ] || [ "$expected" != "$actual" ]; then
    echo "WARNING: sha256 mismatch. expected=${expected:-empty} actual=${actual:-empty} (file records a CI path; compared hashes only)" >&2
  else
    echo "sha256:   ok ($actual)"
  fi
fi

# ---------- unpack ----------
if ! tar -xzf "$TARBALL" -C "$UNPACK_DIR" 2>"${LOG_DIR}/tar.err"; then
  RAW="$(cat "${LOG_DIR}/tar.err")"
  die "tar -xzf failed for ${TARBALL}. Raw error:
${RAW}"
fi
tar -tzf "$TARBALL" >"${LOG_DIR}/tar.list" || true

mapfile -t BINS < <(find "$UNPACK_DIR" -type f -name workbench | sort)
if [ "${#BINS[@]}" -eq 0 ]; then
  TREE="$(find "$UNPACK_DIR" -print | sed 's/^/  /')"
  die "unpacked ${TARBALL} but did not find a 'workbench' binary. Contents:
${TREE}"
fi
BIN="${BINS[0]}"
chmod +x "$BIN" || true
echo "binary:   $BIN"
echo "binary=${BIN}" >>"$ENV_FILE"

# ---------- run commands (same as CI / README) ----------
printf 'name\texit\tlog\tcommand\n' >"$COMMANDS"
CMD_FAIL=0
run_logged() {
  local name="$1"
  shift
  local stdout="${LOG_DIR}/${name}.stdout"
  local stderr="${LOG_DIR}/${name}.stderr"
  local combined="${LOG_DIR}/${name}.log"
  local started ended rc
  started="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  set +e
  "$@" >"$stdout" 2>"$stderr"
  rc=$?
  set -e
  ended="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  {
    echo "command: $*"
    echo "started_utc: $started"
    echo "ended_utc: $ended"
    echo "exit: $rc"
    echo "----- stdout -----"
    cat "$stdout"
    echo "----- stderr -----"
    cat "$stderr"
  } >"$combined"
  printf '%s\t%s\t%s\t%s\n' "$name" "$rc" "$combined" "$*" >>"$COMMANDS"
  echo "[exit ${rc}] ${name}: $*"
  if [ "$rc" -ne 0 ]; then
    CMD_FAIL=1
    echo "  ---- ${name} stdout (tail) ----"
    tail -n 20 "$stdout" | sed 's/^/  | /'
    echo "  ---- ${name} stderr (tail) ----"
    tail -n 20 "$stderr" | sed 's/^/  | /'
  fi
  return 0
}

run_logged 01-version "$BIN" version
run_logged 02-tools "$BIN" tools
run_logged 03-demo-rain "$BIN" demo --weather=rain --tz="$DEMO_TZ" --now="$DEMO_NOW"
run_logged 04-demo-clear "$BIN" demo --weather=clear --tz="$DEMO_TZ" --now="$DEMO_NOW"

if [ "${#EXTRA_ARGS[@]}" -gt 0 ]; then
  run_logged 05-extra "$BIN" "${EXTRA_ARGS[@]}"
fi

echo "utc_end=$(date -u +%Y-%m-%dT%H:%M:%SZ)" >>"$ENV_FILE"

# ---------- summary + tarball ----------
{
  echo "proactive-workbench bug-repro bundle"
  echo "repo:       ${REPO}"
  echo "commit:     ${FULL_SHA}"
  echo "run:        ${RUN_URL}"
  echo "artifact:   ${ARTIFACT_NAME}"
  echo "binary:     ${BIN}"
  echo "out:        ${OUT_DIR}"
  echo
  echo "commands:"
  if command -v column >/dev/null 2>&1; then
    column -t -s $'\t' "$COMMANDS"
  else
    cat "$COMMANDS"
  fi
  echo
  if [ "$CMD_FAIL" -eq 0 ]; then
    echo "result: all workbench commands exited 0"
  else
    echo "result: one or more workbench commands failed (see logs/)"
  fi
} | tee "$SUMMARY"

BUNDLE_PATH="${OUT_DIR}.tar.gz"
tar -C "$(dirname "$OUT_DIR")" -czf "$BUNDLE_PATH" "$(basename "$OUT_DIR")"
echo
echo "bundle:   $BUNDLE_PATH"
echo "Attach that tarball to the GitHub issue (see .github/ISSUE_TEMPLATE/bug_report.md)."

if [ "$CMD_FAIL" -ne 0 ]; then
  exit 1
fi
exit 0
