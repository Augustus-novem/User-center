# 03 Git 开发流程

Git 不仅是版本控制，也是之后的学习路径。

## 基线

```bash
git checkout master
git pull
git status
git tag -a community-base -m "baseline before community backend evolution"
git push origin community-base
```

## 一个 milestone 一个分支

```bash
git checkout master
git pull
git checkout -b feat/community-m01-follow
```

## 一个 milestone 保留多个提交

建议：design → domain/db → repository → service/web → test → handoff。

```text
docs(m01): finalize follow design
feat(m01): add follow persistence
feat(m01): expose follow APIs
test(m01): cover idempotency and pagination
docs(m01): add learning handoff
```

## Commit 前

```bash
git status --short
git diff --stat
git diff
git diff --check
go test ./...
go vet ./...
```

涉及并发：`go test -race ./...`。

不要让 Agent 未审核就 `git add . && git commit`。

## Merge 后打 tag

```bash
git checkout master
git pull
git tag -a community-m01 -m "M01 follow completed"
git push origin community-m01
```

## 为什么不 squash

完成后需要用：

```bash
git diff community-m03..community-m04
git log --oneline community-m03..community-m04
git show <commit>
```

逐阶段学习。

## Agent 禁止

`git reset --hard`、`git clean -fd`、`git push --force`、`git rebase -i`、`git commit --amend`，除非用户明确授权。

## Commit message

```text
feat(mXX):
fix(mXX):
test(mXX):
docs(mXX):
refactor(mXX):
perf(mXX):
chore(mXX):
```
