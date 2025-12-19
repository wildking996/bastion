# `scripts/open-pr.sh`

一个“一键推送 + 创建 PR”的脚本，用于把你当前的 feature 分支推送到 GitHub 并自动创建 PR（关联并关闭 issue）。

## 依赖

- `git`
- `curl`
- `jq`
- `go`（脚本默认会跑 `go test ./...` + `go vet ./...`）

## 认证方式

二选一：

1. 设置环境变量 `GITHUB_TOKEN`（推荐用于临时会话）
2. 在仓库根目录放置 `token.txt`（默认读取该文件；文件内容为 PAT，单行即可）

建议把 `token.txt` 做本地忽略（不要提交到仓库）：

- 本地：`.git/info/exclude` 加一行 `token.txt`

## 基本用法

在目标分支（例如 `feat/todo-6-log-search`）上执行：

```bash
scripts/open-pr.sh --issue 3 --title "feat: HTTP audit log search and filtering"
```

脚本会：

1. `git fetch origin --tags --prune`
2. `git rebase origin/main`（可用 `--no-rebase` 关闭）
3. 执行 `go test ./...` 与 `go vet ./...`（可用 `--skip-tests` 关闭）
4. `git push --force-with-lease origin <current-branch>`
5. 调用 GitHub API 创建 PR，并在正文加入 `Closes #<issue>`

## 常用参数

- 指定 base 分支：

```bash
scripts/open-pr.sh --issue 3 --title "feat: ..." --base main
```

- PR 来自 fork（head owner 不等于 repo owner）：

```bash
scripts/open-pr.sh --issue 3 --title "feat: ..." --head-owner your-fork-owner
```

- 创建 Draft PR：

```bash
scripts/open-pr.sh --issue 3 --title "feat: ..." --draft
```

- 不 rebase / 不跑测试（只用于你明确知道自己在做什么的情况）：

```bash
scripts/open-pr.sh --issue 3 --title "feat: ..." --no-rebase --skip-tests
```

## 注意事项

- 脚本拒绝在 base 分支（默认 `main`）上直接创建 PR。
- 脚本要求工作区干净（`git status` 无未提交改动）。
- 使用 `--force-with-lease` 推送，适配 rebase 后更新远端分支的场景。
