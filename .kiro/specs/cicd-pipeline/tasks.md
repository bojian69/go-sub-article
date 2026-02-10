# Implementation Plan: CI/CD Pipeline

## Overview

分三个阶段实施：先修改 Dockerfile 和新增版本信息包以支持多架构构建和构建参数注入，然后增强现有 CI workflow，最后新建 CD workflow。每个阶段的修改都是增量的，确保不破坏现有功能。

## Tasks

- [x] 1. 新增版本信息包并修改 Dockerfile
  - [x] 1.1 创建 `internal/version/version.go`，声明 Version、BuildTime、GitCommit 变量（默认值为 "dev"、"unknown"、"unknown"），供 ldflags 注入
    - _Requirements: 6.2_
  - [x] 1.2 修改 `Dockerfile`：添加 ARG VERSION、BUILD_TIME、GIT_COMMIT 声明；移除构建阶段硬编码的 `GOOS=linux GOARCH=amd64`；在 go build 的 -ldflags 中注入版本信息变量
    - _Requirements: 6.1, 6.2_

- [x] 2. 增强 CI Workflow
  - [x] 2.1 修改 `.gitea/workflows/ci.yaml` 的 lint job：为 `actions/setup-go` 添加 `cache: true`；在现有 gofmt 和 go vet 之后新增 golangci-lint step（使用 `golangci/golangci-lint-action`）
    - _Requirements: 1.1, 2.1_
  - [x] 2.2 修改 `.gitea/workflows/ci.yaml`：新增 security job（依赖 lint），安装并运行 `govulncheck ./...`；为 setup-go 添加 `cache: true`
    - _Requirements: 4.1, 2.1_
  - [x] 2.3 修改 `.gitea/workflows/ci.yaml` 的 test job：为 `actions/setup-go` 添加 `cache: true`
    - _Requirements: 2.1_
  - [x] 2.4 修改 `.gitea/workflows/ci.yaml` 的 build job：为 `actions/setup-go` 添加 `cache: true`；在 Go binary 构建之后新增 Docker 构建验证 step（`docker build` 不推送）
    - _Requirements: 2.1, 3.1, 3.2_

- [x] 3. Checkpoint - 验证 CI 修改
  - 确保 CI YAML 语法正确，所有 job 依赖关系正确。如有问题请提出。

- [x] 4. 新建 CD Workflow
  - [x] 4.1 创建 `.gitea/workflows/cd.yaml`：配置 `on.push.tags: ['v*']` 触发条件；定义 GO_VERSION 和 IMAGE_NAME 环境变量
    - _Requirements: 5.1, 5.2_
  - [x] 4.2 在 cd.yaml 中实现 build-and-push job：checkout 代码；setup Docker Buildx（`docker/setup-buildx-action`）；login Docker Registry（`docker/login-action`，使用 secrets）；使用 `docker/metadata-action` 生成 version tag 和 latest tag；使用 `docker/build-push-action` 构建多架构镜像（linux/amd64, linux/arm64）并推送，传入 build-args（VERSION=${{ github.ref_name }}、BUILD_TIME、GIT_COMMIT=${{ github.sha }}）
    - _Requirements: 5.3, 6.1, 6.2, 7.1, 7.2, 7.4_

- [x] 5. Final checkpoint - 验证所有修改
  - 确保 CI 和 CD YAML 语法正确，Dockerfile 修改不影响本地构建。如有问题请提出。
