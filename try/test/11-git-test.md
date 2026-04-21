# Git 和 Worktree 工具测试

此文件用于测试 EnterWorktree、ExitWorktree 等 Git 相关工具。

## EnterWorktree
- 创建隔离的 git worktree
- 在独立分支上工作
- 不影响主分支

## ExitWorktree
- 退出 worktree 会话
- 可选择保留或删除 worktree

## 使用场景
- 并行开发多个功能
- 安全地测试实验性代码
- 独立的代码审查环境

## 注意
需要在 git 仓库中才能使用 worktree 功能。
