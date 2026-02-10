# Requirements Document

## Introduction

为 wechat-subscription-svc 微服务项目增强现有 CI 流程并新建 CD 流程。项目使用 Gitea Actions（语法兼容 GitHub Actions），已有基础 CI pipeline（lint、test、build）。目标是引入更全面的代码质量检查、构建缓存优化、安全扫描，并实现基于 tag 触发的自动化多架构 Docker 镜像构建与推送。

## Glossary

- **CI_Pipeline**: 持续集成流水线，在 push 到 main 分支或 PR 到 main 分支时触发，执行代码质量检查、测试和构建验证
- **CD_Pipeline**: 持续部署流水线，在推送 v* 格式的 tag 时触发，执行 Docker 镜像构建和推送
- **golangci-lint**: Go 语言静态分析工具，使用项目根目录的 .golangci.yml 配置文件
- **govulncheck**: Go 官方漏洞扫描工具，检查依赖中的已知安全漏洞
- **Module_Cache**: Go module 下载缓存，通过 actions/cache 或 actions/setup-go 的缓存功能实现
- **Multi_Arch_Image**: 支持多种 CPU 架构（linux/amd64 和 linux/arm64）的 Docker 镜像
- **Version_Tag**: 版本标签，格式为 v年.月.日（如 v26.2.9），同天多次发布使用 .1 .2 后缀
- **Build_Args**: Docker 构建时注入的参数，包括 VERSION、BUILD_TIME、GIT_COMMIT
- **Docker_Registry**: Docker 镜像仓库，用于存储和分发构建好的容器镜像

## Requirements

### Requirement 1: 集成 golangci-lint 静态分析

**User Story:** 作为开发者，我希望 CI 流水线使用 golangci-lint 进行静态分析，以便利用已有的 .golangci.yml 配置发现更多代码质量问题。

#### Acceptance Criteria

1. WHEN CI_Pipeline 执行 lint 阶段, THE CI_Pipeline SHALL 使用 golangci-lint 并加载项目根目录的 .golangci.yml 配置文件运行静态分析
2. WHEN golangci-lint 发现代码问题, THE CI_Pipeline SHALL 将该 lint 阶段标记为失败并输出具体的问题详情
3. WHEN golangci-lint 未发现任何问题, THE CI_Pipeline SHALL 将该 lint 阶段标记为通过

### Requirement 2: Go Module 缓存优化

**User Story:** 作为开发者，我希望 CI 流水线缓存 Go module 依赖，以便减少重复下载依赖的时间，加速流水线执行。

#### Acceptance Criteria

1. THE CI_Pipeline SHALL 在每个需要 Go 依赖的 job 中启用 Module_Cache
2. WHEN 缓存命中时, THE CI_Pipeline SHALL 跳过依赖下载步骤，直接使用缓存的依赖
3. WHEN 缓存未命中时, THE CI_Pipeline SHALL 下载依赖并在 job 结束后更新缓存

### Requirement 3: Docker 构建验证

**User Story:** 作为开发者，我希望在 PR 阶段验证 Dockerfile 能成功构建，以便在合并前发现构建问题。

#### Acceptance Criteria

1. WHEN CI_Pipeline 在 PR 或 push 到 main 时执行, THE CI_Pipeline SHALL 使用项目 Dockerfile 执行 Docker 构建验证
2. WHEN Docker 构建验证成功, THE CI_Pipeline SHALL 将该阶段标记为通过，且构建产物不推送到任何 Docker_Registry
3. WHEN Docker 构建验证失败, THE CI_Pipeline SHALL 将该阶段标记为失败并输出构建错误信息

### Requirement 4: 安全漏洞扫描

**User Story:** 作为开发者，我希望 CI 流水线自动扫描 Go 依赖中的已知安全漏洞，以便及时发现和修复安全风险。

#### Acceptance Criteria

1. WHEN CI_Pipeline 执行安全扫描阶段, THE CI_Pipeline SHALL 使用 govulncheck 扫描项目所有依赖的已知漏洞
2. WHEN govulncheck 发现安全漏洞, THE CI_Pipeline SHALL 将该阶段标记为失败并输出漏洞详情
3. WHEN govulncheck 未发现安全漏洞, THE CI_Pipeline SHALL 将该阶段标记为通过

### Requirement 5: Tag 触发的 CD 流水线

**User Story:** 作为开发者，我希望推送 v* 格式的 tag 时自动触发 CD 流水线，以便实现自动化发布流程。

#### Acceptance Criteria

1. WHEN 一个匹配 v* 模式的 Git tag 被推送时, THE CD_Pipeline SHALL 自动触发执行
2. WHEN 一个不匹配 v* 模式的 Git tag 被推送时, THE CD_Pipeline SHALL 不触发执行
3. WHEN CD_Pipeline 触发时, THE CD_Pipeline SHALL 从触发的 tag 中提取 Version_Tag 用于镜像标记

### Requirement 6: 多架构 Docker 镜像构建

**User Story:** 作为开发者，我希望 CD 流水线自动构建支持 linux/amd64 和 linux/arm64 的 Docker 镜像，以便在不同架构的服务器上部署。

#### Acceptance Criteria

1. WHEN CD_Pipeline 执行构建阶段, THE CD_Pipeline SHALL 使用 docker buildx 构建同时支持 linux/amd64 和 linux/arm64 架构的 Multi_Arch_Image
2. WHEN 构建 Multi_Arch_Image 时, THE CD_Pipeline SHALL 注入 Build_Args（VERSION 为触发的 Version_Tag，BUILD_TIME 为 UTC 格式的构建时间，GIT_COMMIT 为触发 tag 对应的完整 commit SHA）
3. IF Multi_Arch_Image 构建失败, THEN THE CD_Pipeline SHALL 将流水线标记为失败并输出构建错误信息

### Requirement 7: Docker 镜像推送与标记

**User Story:** 作为开发者，我希望构建好的 Docker 镜像自动推送到 Docker Registry 并打上版本标签和 latest 标签，以便部署时可以按版本或最新版本拉取镜像。

#### Acceptance Criteria

1. WHEN Multi_Arch_Image 构建成功, THE CD_Pipeline SHALL 将镜像推送到配置的 Docker_Registry
2. WHEN 推送镜像时, THE CD_Pipeline SHALL 同时为镜像打上 Version_Tag 标签和 latest 标签
3. IF 镜像推送到 Docker_Registry 失败, THEN THE CD_Pipeline SHALL 将流水线标记为失败并输出错误信息
4. THE CD_Pipeline SHALL 通过 Gitea Actions secrets 获取 Docker_Registry 的认证凭据，凭据不得硬编码在工作流文件中
