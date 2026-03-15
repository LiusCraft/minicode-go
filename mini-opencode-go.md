---
title: Go 版 Mini OpenCode 实现笔记
description: 面向 AI Coding Agent MVP 的工程总结
---

# Go 版 Mini OpenCode 实现笔记

占位符定义：`{{opencode}}` 表示当前 OpenCode 仓库根目录:`/Users/liushunshun/workspace/coding/ai/opencode`。后文所有“可以去哪里学”的源码路径，都用这个占位符标注，例如 `{{opencode}}/packages/opencode/src/session/prompt.ts`。

这份文档的目标不是复刻完整 OpenCode，而是帮助你用 Go 做一个最小可用的 AI coding agent：能理解代码、调用工具、修改文件、执行命令，并把结果继续喂回模型。

## 1. 先明确你要做什么

- 形态：一个本地 CLI 工具，比如 `minioc "修复当前项目的 lint 错误"`
- 目标：打通 `用户输入 -> 模型推理 -> 工具调用 -> 工具结果回灌 -> 最终回答`
- MVP 重点：稳定完成真实小任务，而不是一开始把功能铺太大
- 第一版建议只支持单仓库、本地运行、单模型、单 agent

## 2. MVP 范围

### 必须有

- CLI 入口
- 会话模型和消息持久化
- 模型调用层
- 工具注册与执行
- 核心工具：`read_file`、`glob`、`grep`、`bash`、`edit` 或 `write_file`
- 基础权限系统
- 流式或准流式输出

### 先不要做

- 多 agent 并行
- MCP
- 插件系统
- TUI / Web UI
- 复杂工作区管理
- 自动 PR / 自动发版
- 复杂上下文压缩和长程记忆

## 3. 推荐技术栈

- 语言：Go `1.23+`
- 模型 SDK：`github.com/openai/openai-go/v3`
- 模型接口：优先用 `Responses API` + tool calling
- CLI：`flag` 或 `cobra`
- HTTP：先不做，或者用标准库 `net/http`
- 存储：SQLite
- 日志：`log/slog`
- 搜索：优先直接调用 `rg`
- 命令执行：标准库 `os/exec`

## 4. 你最小要拆成哪几层

建议先拆成 6 层，职责尽量单一：

- `cli`：参数解析、工作目录、输出
- `session`：会话生命周期、消息追加、循环编排
- `llm`：对接 `openai-go`，隔离 SDK 细节
- `tool`：工具定义、注册、执行
- `safety`：路径限制、命令确认、权限策略
- `store`：SQLite 或 JSONL 持久化

建议的数据流：

```text
CLI -> Session Loop -> Prompt Builder -> LLM
                        |                |
                        v                v
                   Tool Registry <- Tool Executor
                        |
                        v
                       Store
```

## 5. 建议目录结构

```text
cmd/minioc/main.go
internal/agent/loop.go
internal/llm/openai.go
internal/tools/registry.go
internal/tools/read.go
internal/tools/glob.go
internal/tools/grep.go
internal/tools/bash.go
internal/tools/edit.go
internal/safety/path.go
internal/safety/permission.go
internal/store/session.go
```

## 6. 主循环怎么想

你的核心不是“聊天”，而是“能连续执行动作的循环”。

最小闭环：

1. 组装系统提示词和用户输入
2. 把可用工具描述传给模型
3. 如果模型输出普通文本，直接返回给用户
4. 如果模型发起工具调用，就执行工具
5. 把工具结果作为新的输入回灌给模型
6. 继续循环，直到得到最终回答或达到 `maxSteps`

你真正要守住的是这几个边界：

- 每一步都要知道当前 workdir
- 每次工具调用都要有结构化记录
- 每次工具结果都要能重新喂回模型
- 任何副作用都要能审计

## 7. 每一块可以去 OpenCode 学什么

下面这张表是最重要的部分。你写 Go 版 mini opencode 时，每做一块都可以去这些位置对照思路。

| 你要做的模块       | 你在 Go 里要实现什么                     | 可以重点参考的 OpenCode 位置                                                                                                                                                                                                       |
| ------------------ | ---------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| CLI 入口           | 命令解析、启动会话、传入用户 prompt      | `{{opencode}}/packages/opencode/src/index.ts`、`{{opencode}}/packages/opencode/src/cli/cmd/run.ts`、`{{opencode}}/packages/opencode/src/cli/cmd/tui/thread.ts`                                                                     |
| 工作目录与项目边界 | 确定仓库根、工作目录、运行实例边界       | `{{opencode}}/packages/opencode/src/project/instance.ts`、`{{opencode}}/packages/opencode/src/project/project.ts`、`{{opencode}}/packages/opencode/src/control-plane/workspace.ts`                                                 |
| 配置加载           | API key、模型名、默认行为、用户配置      | `{{opencode}}/packages/opencode/src/config/config.ts`                                                                                                                                                                              |
| 会话模型           | session/message/part 的组织方式          | `{{opencode}}/packages/opencode/src/session/index.ts`、`{{opencode}}/packages/opencode/src/session/message.ts`、`{{opencode}}/packages/opencode/src/session/message-v2.ts`、`{{opencode}}/packages/opencode/src/session/schema.ts` |
| 提示词构造         | system prompt、工具注入、上下文拼装      | `{{opencode}}/packages/opencode/src/session/system.ts`、`{{opencode}}/packages/opencode/src/session/prompt.ts`                                                                                                                     |
| 主循环             | 用户消息进入后如何持续调用模型和工具     | `{{opencode}}/packages/opencode/src/session/prompt.ts`、`{{opencode}}/packages/opencode/src/session/processor.ts`、`{{opencode}}/packages/opencode/src/session/llm.ts`                                                             |
| Provider 抽象      | 隔离模型 SDK，后面可切 provider          | `{{opencode}}/packages/opencode/src/provider/provider.ts`、`{{opencode}}/packages/opencode/src/provider/schema.ts`、`{{opencode}}/packages/opencode/src/provider/transform.ts`                                                     |
| 工具框架           | 工具定义、注册、统一上下文、输出截断     | `{{opencode}}/packages/opencode/src/tool/tool.ts`、`{{opencode}}/packages/opencode/src/tool/registry.ts`、`{{opencode}}/packages/opencode/src/tool/truncation.ts`、`{{opencode}}/packages/opencode/src/tool/schema.ts`             |
| 文件读取类工具     | `read/glob/grep/ls` 这类只读能力         | `{{opencode}}/packages/opencode/src/tool/read.ts`、`{{opencode}}/packages/opencode/src/tool/glob.ts`、`{{opencode}}/packages/opencode/src/tool/grep.ts`、`{{opencode}}/packages/opencode/src/tool/ls.ts`                           |
| 文件修改类工具     | `edit/write/apply_patch/multiedit`       | `{{opencode}}/packages/opencode/src/tool/edit.ts`、`{{opencode}}/packages/opencode/src/tool/write.ts`、`{{opencode}}/packages/opencode/src/tool/apply_patch.ts`、`{{opencode}}/packages/opencode/src/tool/multiedit.ts`            |
| 命令执行工具       | `bash` 的超时、工作目录、输出处理        | `{{opencode}}/packages/opencode/src/tool/bash.ts`                                                                                                                                                                                  |
| 权限系统           | 哪些操作允许、拒绝、确认                 | `{{opencode}}/packages/opencode/src/agent/agent.ts`、`{{opencode}}/packages/opencode/src/permission/next.ts`                                                                                                                       |
| 仓库外路径限制     | 防止工具越界访问                         | `{{opencode}}/packages/opencode/src/tool/external-directory.ts`                                                                                                                                                                    |
| 会话存储           | 存消息、状态、恢复会话                   | `{{opencode}}/packages/opencode/src/storage/db.ts`、`{{opencode}}/packages/opencode/src/session/session.sql.ts`                                                                                                                    |
| HTTP / 服务层      | 如果未来想做前后端分离，可看接口组织方式 | `{{opencode}}/packages/opencode/src/server/server.ts`、`{{opencode}}/packages/opencode/src/server/routes/session.ts`、`{{opencode}}/packages/opencode/src/server/routes/question.ts`                                               |
| 任务型工具         | 以后若想扩展子 agent 或任务代理          | `{{opencode}}/packages/opencode/src/tool/task.ts`                                                                                                                                                                                  |

## 8. 用 `openai-go` 时建议怎么落地

你已经决定先用官方库，这是很合理的。

建议：

- 第一版只包一个 `internal/llm/openai.go`
- 在你自己的代码里定义一个小接口，不要让业务层直接依赖 `openai-go` 的具体类型
- 第一版优先用 `Responses API`
- 先把非流式 + tool loop 跑通，再补流式

建议接口大概像这样：

```go
type Client interface {
	Run(ctx context.Context, req Request) (Result, error)
}
```

`Request` 里放：

- model
- instructions
- input
- tools
- previous response id

`Result` 里放：

- text
- tool calls
- response id
- raw metadata

这样后面你就算换成别的 provider，也不需要把 `session` 和 `tool` 层全改掉。

## 9. 安全规则一定要先加

这是 coding agent 和普通聊天工具最大的差异。

建议默认规则：

- `read/glob/grep` 默认允许
- `edit/write/apply_patch` 默认确认
- `bash` 默认确认
- 仓库外路径默认拒绝
- 删除类命令默认拒绝
- 高风险 git 命令默认拒绝

至少要做的保护：

- 所有路径先转绝对路径再校验是否在 repo root 内
- `bash` 必须带超时
- `bash` 输出必须截断
- 每个工具调用都记录参数和结果摘要
- 修改文件前最好检查当前内容是否仍匹配预期

这一块可以重点看：

- `{{opencode}}/packages/opencode/src/permission/next.ts`
- `{{opencode}}/packages/opencode/src/agent/agent.ts`
- `{{opencode}}/packages/opencode/src/tool/external-directory.ts`

## 10. 最适合你的 7 天实现顺序

### Day 1：项目骨架

- 初始化 Go module
- 做 `main.go`
- 加配置读取
- 跑通一次普通文本响应

重点参考：

- `{{opencode}}/packages/opencode/src/index.ts`
- `{{opencode}}/packages/opencode/src/config/config.ts`

### Day 2：会话与消息

- 定义 session/message/event
- 把 user 和 assistant 消息存起来
- 支持 `--continue`

重点参考：

- `{{opencode}}/packages/opencode/src/session/index.ts`
- `{{opencode}}/packages/opencode/src/session/message.ts`
- `{{opencode}}/packages/opencode/src/session/schema.ts`

### Day 3：Prompt 与主循环

- 组 system prompt
- 把用户输入和历史消息拼起来
- 做 `maxSteps` 循环

重点参考：

- `{{opencode}}/packages/opencode/src/session/system.ts`
- `{{opencode}}/packages/opencode/src/session/prompt.ts`
- `{{opencode}}/packages/opencode/src/session/processor.ts`

### Day 4：只读工具

- `read_file`
- `glob`
- `grep`
- `list_dir` 或 `ls`

重点参考：

- `{{opencode}}/packages/opencode/src/tool/read.ts`
- `{{opencode}}/packages/opencode/src/tool/glob.ts`
- `{{opencode}}/packages/opencode/src/tool/grep.ts`
- `{{opencode}}/packages/opencode/src/tool/ls.ts`

### Day 5：副作用工具

- `bash`
- `edit` 或 `write_file`
- 工具结果回灌模型

重点参考：

- `{{opencode}}/packages/opencode/src/tool/bash.ts`
- `{{opencode}}/packages/opencode/src/tool/edit.ts`
- `{{opencode}}/packages/opencode/src/tool/write.ts`

### Day 6：权限与安全

- 命令确认
- 路径限制
- 输出截断
- 命令超时

重点参考：

- `{{opencode}}/packages/opencode/src/permission/next.ts`
- `{{opencode}}/packages/opencode/src/agent/agent.ts`
- `{{opencode}}/packages/opencode/src/tool/truncation.ts`
- `{{opencode}}/packages/opencode/src/tool/external-directory.ts`

### Day 7：验证与打磨

- 拿 5 到 10 个真实任务测试
- 修正工具误用
- 修正上下文不够的问题
- 写 README 和使用说明

重点参考：

- `{{opencode}}/CONTRIBUTING.md`
- `{{opencode}}/README.md`
- `{{opencode}}/packages/opencode/test`

## 11. 推荐阅读顺序

如果你时间有限，按这个顺序看最值钱：

1. `{{opencode}}/packages/opencode/src/index.ts`
2. `{{opencode}}/packages/opencode/src/cli/cmd/run.ts`
3. `{{opencode}}/packages/opencode/src/session/prompt.ts`
4. `{{opencode}}/packages/opencode/src/session/processor.ts`
5. `{{opencode}}/packages/opencode/src/tool/registry.ts`
6. `{{opencode}}/packages/opencode/src/tool/read.ts`
7. `{{opencode}}/packages/opencode/src/tool/bash.ts`
8. `{{opencode}}/packages/opencode/src/tool/edit.ts`
9. `{{opencode}}/packages/opencode/src/permission/next.ts`
10. `{{opencode}}/packages/opencode/src/storage/db.ts`

## 12. 你自己的 MVP 完成标准

做到下面这些，第一版就算成立：

- [ ] 能在当前仓库启动并识别 repo root
- [ ] 能向模型发请求并收到回答
- [ ] 能注册并执行 `read_file`
- [ ] 能注册并执行 `glob`
- [ ] 能注册并执行 `grep`
- [ ] 能注册并执行 `bash`
- [ ] 能注册并执行 `edit` 或 `write_file`
- [ ] 工具结果能继续回灌给模型
- [ ] 有 `maxSteps` 防止死循环
- [ ] 有超时、输出截断、路径限制
- [ ] 有最基本的会话持久化
- [ ] 能完成至少 5 个真实小任务

## 13. 一个最重要的工程建议

不要一开始就想着“做一个完整的 OpenCode”。

更好的目标是：

- 第一周只做一条稳定主链路
- 先把工具做少，但把边界做清楚
- 先让 agent 能稳定完成小任务
- 再逐步扩展到更多工具、更多策略、更多 UI

如果你照这个思路做，Go 版 `mini opencode` 会比“功能很多但跑不通”的版本更快进入可用状态。
