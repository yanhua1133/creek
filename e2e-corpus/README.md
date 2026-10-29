# 共享 E2E 输入语料

## 结论

本目录只保存全项目共用的大型开源项目输入语料。Parser、Common AST、IR、符号表、调用图和后续模块都读取这里的同一份源码，不得在各自模块中复制 submodule。

## 目录职责

- `projects.json`：锁定项目标识、语言、仓库、commit、Star 核验值、扩展名、排除路径和源码文件数。
- `projects/`：五个固定 commit 的 Git submodule，只读使用。
- `../internal/e2etest/`：各模块共用的清单读取、commit 校验和源码过滤实现。
- 本目录禁止存放任何模块的 expected、golden、指标基线或分析结果。

## 模块预期

每个模块在自己的 `test/e2e/fixture/expected/` 中保存预期结果。expected 必须写明测试目的、共享项目标识、逐项断言和核心结构化输出 SHA-256。

## 初始化

```text
git submodule update --init --recursive e2e-corpus/projects
```
