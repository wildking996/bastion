#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  scripts/open-pr.sh --issue <number> --title "<conventional title>" [options]

Required:
  --issue <n>            GitHub issue number to close (adds "Closes #n" to PR body)
  --title "<title>"      PR title (recommend Conventional Commits, e.g. "feat: ...")

Options:
  --base <branch>        Base branch (default: main)
  --no-rebase            Skip rebase onto origin/<base>
  --skip-tests           Skip go test/go vet
  --draft                Create as draft PR
  --repo <owner/name>    Override repo (default: inferred from git remote "origin")
  --head-owner <owner>   PR head owner (default: same as --repo owner)
  --token-file <path>    Token file path (default: token.txt)
  -h, --help             Show help

Env:
  GITHUB_TOKEN           If set, used instead of reading token file

Notes:
  - Must be run inside the repo with a non-base branch checked out.
  - Uses curl + jq + git. Does not auto-merge.
EOF
}

die() {
  echo "error: $*" >&2
  exit 1
}

require_cmd() {
  local cmd="$1"
  if command -v "$cmd" >/dev/null 2>&1; then
    return 0
  fi
  if command -v "${cmd}.exe" >/dev/null 2>&1; then
    return 0
  fi
  die "missing required command: $cmd"
}

git_repo_slug_from_origin() {
  local url
  url="$(git remote get-url origin 2>/dev/null || true)"
  [ -n "$url" ] || die "cannot determine git remote 'origin' URL"

  # Supports:
  # - git@github.com:owner/repo.git
  # - ssh://git@github.com/owner/repo.git
  # - https://github.com/owner/repo.git
  # - https://<host>/owner/repo(.git)
  url="${url%.git}"

  local slug=""
  if [[ "$url" =~ ^git@[^:]+:([^/]+/[^/]+)$ ]]; then
    slug="${BASH_REMATCH[1]}"
  elif [[ "$url" =~ ^ssh://git@[^/]+/([^/]+/[^/]+)$ ]]; then
    slug="${BASH_REMATCH[1]}"
  elif [[ "$url" =~ ^https?://[^/]+/([^/]+/[^/]+)$ ]]; then
    slug="${BASH_REMATCH[1]}"
  fi

  [ -n "$slug" ] || die "unsupported origin URL format: $url"
  echo "$slug"
}

read_token() {
  local token_file="$1"
  if [ -n "${GITHUB_TOKEN:-}" ]; then
    printf '%s' "${GITHUB_TOKEN}"
    return 0
  fi
  [ -f "$token_file" ] || die "token file not found: $token_file (or set GITHUB_TOKEN)"
  tr -d '\r\n' <"$token_file"
}

main() {
  local issue=""
  local title=""
  local base="main"
  local do_rebase="true"
  local do_tests="true"
  local draft="false"
  local repo=""
  local head_owner=""
  local token_file="token.txt"

  while [ "${1:-}" != "" ]; do
    case "$1" in
      --issue)
        issue="${2:-}"
        shift 2
        ;;
      --title)
        title="${2:-}"
        shift 2
        ;;
      --base)
        base="${2:-}"
        shift 2
        ;;
      --no-rebase)
        do_rebase="false"
        shift 1
        ;;
      --skip-tests)
        do_tests="false"
        shift 1
        ;;
      --draft)
        draft="true"
        shift 1
        ;;
      --repo)
        repo="${2:-}"
        shift 2
        ;;
      --head-owner)
        head_owner="${2:-}"
        shift 2
        ;;
      --token-file)
        token_file="${2:-}"
        shift 2
        ;;
      -h|--help)
        usage
        exit 0
      ;;
      *)
        die "unknown argument: $1"
        ;;
    esac
  done

  require_cmd git
  require_cmd curl
  require_cmd jq
  if [ "$do_tests" = "true" ]; then
    require_cmd go
  fi

  [ -n "$issue" ] || die "--issue is required"
  [[ "$issue" =~ ^[0-9]+$ ]] || die "--issue must be a number"
  [ -n "$title" ] || die "--title is required"

  git rev-parse --is-inside-work-tree >/dev/null 2>&1 || die "not inside a git repository"

  local branch
  branch="$(git rev-parse --abbrev-ref HEAD)"
  [ "$branch" != "HEAD" ] || die "detached HEAD is not supported"
  [ "$branch" != "$base" ] || die "refusing to open PR from base branch: $base"

  if [ -n "$(git status --porcelain)" ]; then
    die "working tree is not clean (commit or stash changes first)"
  fi

  if [ -z "$repo" ]; then
    repo="$(git_repo_slug_from_origin)"
  fi
  if [ -z "$head_owner" ]; then
    head_owner="${repo%%/*}"
  fi

  local token
  token="$(read_token "$token_file")"
  [ -n "$token" ] || die "empty token (set GITHUB_TOKEN or check token file)"

  git fetch origin --prune --tags
  if [ "$do_rebase" = "true" ]; then
    git rebase "origin/${base}"
  fi

  if [ "$do_tests" = "true" ]; then
    go test ./...
    go vet ./...
  fi

  git push --force-with-lease origin "$branch"

  local body
  body=$'Closes #'"${issue}"$'\n\n'"${title}"

  local payload
  payload="$(jq -n \
    --arg title "$title" \
    --arg head "${head_owner}:${branch}" \
    --arg base "$base" \
    --arg body "$body" \
    --argjson draft "$draft" \
    '{title:$title, head:$head, base:$base, body:$body, draft:$draft}')"

  local resp
  resp="$(curl -sS -X POST \
    -H "Authorization: token ${token}" \
    -H 'Accept: application/vnd.github+json' \
    "https://api.github.com/repos/${repo}/pulls" \
    -d "${payload}")"

  if echo "$resp" | jq -e '.html_url' >/dev/null 2>&1; then
    echo "$resp" | jq -r '"PR #" + (.number|tostring) + " created: " + .html_url'
    exit 0
  fi

  local msg
  msg="$(echo "$resp" | jq -r '.message // empty')"
  if [ "$msg" = "Validation Failed" ]; then
    echo "PR already exists or validation failed; searching existing open PRs for branch: ${branch}"
    curl -sS \
      -H "Authorization: token ${token}" \
      -H 'Accept: application/vnd.github+json' \
      "https://api.github.com/repos/${repo}/pulls?state=open&per_page=100" \
    | jq -r ".[] | select(.head.ref==\"${branch}\") | \"existing PR #\\(.number): \\(.html_url)\""
    exit 0
  fi

  echo "$resp" | jq -r '.'
  exit 1
}

main "$@"
