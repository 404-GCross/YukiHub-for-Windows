#!/usr/bin/env bash
# 一键把历史提交的作者 / 提交者改成你自己。
#
# 背景：仓库最初是用占位署名（YukiHub Dev <dev@yukihub.local>）提交的，
# 推到 GitHub / gitcode 后每条记录都显示成「神秘来客」。这个脚本重写
# **全部历史提交**的身份信息。
#
# 用法（在 Git Bash 或任意 bash 里）：
#   bash scripts/rewrite-author.sh
#
# 脚本会交互式询问名字与邮箱（只在本机输入，不经过任何第三方），
# 改写完成后打印需要你自己执行的 push 命令。
#
# ⚠️ 注意
#   - 改写后**所有提交的 SHA 都会变**，本地与远端历史会「分叉」，
#     这是正常现象，force push 之后即恢复一致。
#   - 推送必须带 --force。已经 clone 过本仓库的其他人需要重新 clone，
#     或 `git fetch origin && git reset --hard origin/main`。
#   - 脚本会自动打一个本地备份 tag，反悔时可 `git reset --hard <tag>` 回退。

set -euo pipefail

# 要被替换的旧署名邮箱。默认是仓库最初使用的占位邮箱；
# 如果已经改写过一轮，可以传环境变量指定上一轮的邮箱：
#   OLD_EMAIL=上一轮的邮箱 bash scripts/rewrite-author.sh
OLD_EMAIL="${OLD_EMAIL:-dev@yukihub.local}"

cd "$(dirname "$0")/.."

echo "== 改写前的提交者统计 =="
git log --format='%an <%ae>' | sort | uniq -c
echo

if [ -n "$(git status --porcelain)" ]; then
	echo "✗ 工作区不干净，请先提交或 stash 后再运行。"
	exit 1
fi

if ! command -v git >/dev/null 2>&1; then
	echo "✗ 找不到 git。"
	exit 1
fi

read -r -p "新的提交者名字（例如 Yuki）: " GIT_NAME
read -r -p "新的提交者邮箱（你的 GitHub / gitcode 账号邮箱）: " GIT_EMAIL

if [ -z "$GIT_NAME" ] || [ -z "$GIT_EMAIL" ]; then
	echo "✗ 名字与邮箱都不能为空。"
	exit 1
fi

echo
echo "== 将把 <$OLD_EMAIL> 的提交改写为：$GIT_NAME <$GIT_EMAIL> =="
read -r -p "确认继续？(yes/no) " CONFIRM
if [ "$CONFIRM" != "yes" ]; then
	echo "已取消。"
	exit 0
fi

# 之后的提交也用这个署名
git config user.name "$GIT_NAME"
git config user.email "$GIT_EMAIL"

BACKUP_TAG="backup-before-author-rewrite-$(date +%Y%m%d%H%M%S)"
git tag "$BACKUP_TAG"
echo "已打本地备份 tag：$BACKUP_TAG"

FILTER_BRANCH_SQUELCH_WARNING=1 git filter-branch --force \
	--env-filter '
		if [ "$GIT_AUTHOR_EMAIL" = "'"$OLD_EMAIL"'" ]; then
			export GIT_AUTHOR_NAME="'"$GIT_NAME"'"
			export GIT_AUTHOR_EMAIL="'"$GIT_EMAIL"'"
		fi
		if [ "$GIT_COMMITTER_EMAIL" = "'"$OLD_EMAIL"'" ]; then
			export GIT_COMMITTER_NAME="'"$GIT_NAME"'"
			export GIT_COMMITTER_EMAIL="'"$GIT_EMAIL"'"
		fi
	' \
	--tag-name-filter cat -- --all

echo
echo "== 改写后的提交者统计 =="
git log --format='%an <%ae>' | sort | uniq -c

echo
echo "== 下一步：由你执行推送（--force 必须带）=="
for remote in origin gitcode; do
	if git remote get-url "$remote" >/dev/null 2>&1; then
		echo "  git push --force $remote main"
	fi
done

echo
echo "推送完成后可以删掉本地备份 tag："
echo "  git tag -d $BACKUP_TAG"
