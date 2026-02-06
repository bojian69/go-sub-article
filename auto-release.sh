#!/bin/bash
# 自动发布脚本 - WeChat Subscription Service
# 功能：
#   1. 对比本地和远程 commit id，有更新才拉取代码
#   2. 自动打包多架构镜像
#   3. 可选推送到 Docker 仓库（需传入 docker-namespace）
#   4. 本地运行新镜像并反馈状态
# 
# 标签规则: v26.1.20 (v + 年后两位.月.日)
# 同一天多次提交: v26.1.20.1, v26.1.20.2 ...
#
# 使用方法:
#   ./auto-release.sh [选项] [docker-namespace]
#   
# 选项:
#   -f, --force    强制执行（忽略 commit id 对比）
#
# 示例:
#   ./auto-release.sh                    # 仅本地构建和运行
#   ./auto-release.sh myusername         # 构建并推送到 Docker Hub
#   ./auto-release.sh -f                 # 强制本地构建
#   ./auto-release.sh -f myusername      # 强制构建并推送

set -e

# 记录开始时间
START_TIME=$(date +%s)
START_TIME_STR=$(date '+%Y-%m-%d %H:%M:%S')

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 默认配置
FORCE_RUN=false
DOCKER_NAMESPACE=""
CONTAINER_NAME="wechat-subscription-svc"
IMAGE_NAME="wechat-subscription-svc"
HTTP_PORT=8080
GRPC_PORT=9090

# 显示使用说明
show_usage() {
    echo -e "${BLUE}使用方法:${NC}"
    echo "  $0 [选项] [docker-namespace]"
    echo ""
    echo -e "${BLUE}选项:${NC}"
    echo "  -f, --force    强制执行（忽略 commit id 对比）"
    echo "  -h, --help     显示帮助信息"
    echo ""
    echo -e "${BLUE}参数说明:${NC}"
    echo "  docker-namespace - Docker Hub 用户名或组织名（可选）"
    echo "                     不传则仅本地构建，不推送到仓库"
    echo ""
    echo -e "${BLUE}示例:${NC}"
    echo "  $0                    # 仅本地构建和运行"
    echo "  $0 myusername         # 构建并推送到 Docker Hub"
    echo "  $0 -f                 # 强制本地构建（忽略 commit 对比）"
    echo "  $0 -f myusername      # 强制构建并推送"
    exit 0
}

# 解析参数
while [[ $# -gt 0 ]]; do
    case $1 in
        -f|--force)
            FORCE_RUN=true
            shift
            ;;
        -h|--help)
            show_usage
            ;;
        -*)
            echo -e "${RED}错误: 未知选项 $1${NC}"
            show_usage
            ;;
        *)
            DOCKER_NAMESPACE="$1"
            shift
            ;;
    esac
done

echo -e "${BLUE}==========================================${NC}"
echo -e "${BLUE}  WeChat Subscription Service 自动发布   ${NC}"
echo -e "${BLUE}==========================================${NC}"
echo ""

# ==================== 步骤 1: 检查远程代码更新 ====================
echo -e "${YELLOW}步骤 1: 检查远程代码更新...${NC}"

# 获取当前分支
CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)
echo -e "当前分支: ${CURRENT_BRANCH}"

# 获取本地 commit id
LOCAL_COMMIT=$(git rev-parse HEAD 2>/dev/null || echo "")
echo -e "本地 commit: ${LOCAL_COMMIT:0:8}"

# 获取远程最新 commit id
echo "获取远程仓库信息..."
git fetch origin 2>/dev/null || {
    echo -e "${RED}错误: 无法获取远程仓库信息${NC}"
    exit 1
}

REMOTE_COMMIT=$(git rev-parse origin/${CURRENT_BRANCH} 2>/dev/null || echo "")
echo -e "远程 commit: ${REMOTE_COMMIT:0:8}"

# 对比本地和远程 commit id
if [ "$FORCE_RUN" = false ]; then
    if [ "$LOCAL_COMMIT" = "$REMOTE_COMMIT" ]; then
        END_TIME_STR=$(date '+%Y-%m-%d %H:%M:%S')
        echo -e "${GREEN}✓ 本地与远程代码一致，无需更新${NC}"
        echo ""
        echo -e "检查时间:    ${GREEN}${END_TIME_STR}${NC}"
        echo -e "${BLUE}提示: 使用 -f 参数可强制执行${NC}"
        exit 0
    fi
fi

# 本地和远程不一致，拉取远程代码
echo ""
echo -e "${YELLOW}检测到远程有更新，拉取代码...${NC}"
git pull origin ${CURRENT_BRANCH} || {
    echo -e "${RED}错误: 拉取代码失败，可能存在冲突${NC}"
    exit 1
}

# 更新本地 commit id
LOCAL_COMMIT=$(git rev-parse HEAD)
echo -e "${GREEN}✓ 代码已更新到: ${LOCAL_COMMIT:0:8}${NC}"
echo ""

# ==================== 步骤 2: 生成版本标签 ====================
echo -e "${YELLOW}步骤 2: 生成版本标签...${NC}"

# 获取当前日期信息
YEAR=$(date +%y)      # 年后两位 如 26
MONTH=$(date +%-m)    # 月份 不补零 如 2
DAY=$(date +%-d)      # 日期 不补零 如 2

# 基础标签 (格式: v26.2.2)
BASE_TAG="v${YEAR}.${MONTH}.${DAY}"

# 获取本地今天的所有标签，确定后缀
SUFFIX=""
EXISTING_TAGS=$(git tag -l "${BASE_TAG}*" 2>/dev/null | sort -V || echo "")

if [ -n "$EXISTING_TAGS" ]; then
    # 找出最大的后缀数字
    MAX_SUFFIX=0
    for tag in $EXISTING_TAGS; do
        if [ "$tag" = "$BASE_TAG" ]; then
            # 基础标签已存在，下一个应该是 .1
            if [ $MAX_SUFFIX -lt 1 ]; then
                MAX_SUFFIX=1
            fi
        elif [[ "$tag" =~ ^${BASE_TAG}\.([0-9]+)$ ]]; then
            NUM="${BASH_REMATCH[1]}"
            NEXT=$((NUM + 1))
            if [ $NEXT -gt $MAX_SUFFIX ]; then
                MAX_SUFFIX=$NEXT
            fi
        fi
    done
    
    if [ $MAX_SUFFIX -gt 0 ]; then
        SUFFIX=".${MAX_SUFFIX}"
    fi
fi

VERSION_TAG="${BASE_TAG}${SUFFIX}"
echo -e "本地已有标签: ${EXISTING_TAGS:-无}"
echo -e "${GREEN}✓ 新版本标签: ${VERSION_TAG}${NC}"

# 创建本地 Git 标签
echo "创建本地 Git 标签..."
git tag -a "$VERSION_TAG" -m "Release ${VERSION_TAG}" 2>/dev/null || {
    echo -e "${YELLOW}提示: 标签 ${VERSION_TAG} 已存在，跳过创建${NC}"
}
echo -e "${GREEN}✓ 本地标签已创建${NC}"

# 推送标签到远程
echo ""
echo "推送标签到远程..."
git push origin "$VERSION_TAG" 2>/dev/null || {
    echo -e "${YELLOW}提示: 推送标签失败，可能已存在或无权限${NC}"
}
echo -e "${GREEN}✓ 标签已推送${NC}"
echo ""

# ==================== 步骤 3: Docker 构建 ====================
echo -e "${YELLOW}步骤 3: Docker 镜像构建...${NC}"

BUILD_ARGS="--build-arg VERSION=${VERSION_TAG} \
    --build-arg BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ") \
    --build-arg GIT_COMMIT=${LOCAL_COMMIT}"

if [ -n "$DOCKER_NAMESPACE" ]; then
    # 有 namespace，构建多架构并推送
    REGISTRY="${DOCKER_REGISTRY:-docker.io}"
    FULL_IMAGE_NAME="${REGISTRY}/${DOCKER_NAMESPACE}/${IMAGE_NAME}:${VERSION_TAG}"
    LATEST_IMAGE="${REGISTRY}/${DOCKER_NAMESPACE}/${IMAGE_NAME}:latest"
    LOCAL_IMAGE="${IMAGE_NAME}:${VERSION_TAG}"
    
    echo -e "${BLUE}模式: 多架构构建 + 推送${NC}"
    echo -e "${BLUE}镜像名称: ${FULL_IMAGE_NAME}${NC}"
    echo -e "${BLUE}架构支持: linux/amd64, linux/arm64${NC}"
    echo ""
    
    # 登录 Docker 仓库
    echo "登录 Docker 仓库..."
    if [ -n "${DOCKER_USERNAME}" ] && [ -n "${DOCKER_PASSWORD}" ]; then
        echo "${DOCKER_PASSWORD}" | docker login "${REGISTRY}" -u "${DOCKER_USERNAME}" --password-stdin
        echo -e "${GREEN}✓ 登录成功${NC}"
    else
        echo -e "${YELLOW}提示: 如果未登录，请手动执行: docker login ${REGISTRY}${NC}"
    fi
    
    # 创建/使用 buildx builder
    echo ""
    echo "准备 buildx builder..."
    if ! docker buildx inspect multiarch-builder > /dev/null 2>&1; then
        echo "创建新的 builder: multiarch-builder"
        docker buildx create --name multiarch-builder --use
    else
        echo "使用现有 builder: multiarch-builder"
        docker buildx use multiarch-builder
    fi
    docker buildx inspect --bootstrap > /dev/null 2>&1
    echo -e "${GREEN}✓ Builder 准备完成${NC}"
    
    # 构建并推送多架构镜像
    echo ""
    echo "构建并推送多架构镜像..."
    docker buildx build \
        --platform linux/amd64,linux/arm64 \
        ${BUILD_ARGS} \
        -t "${FULL_IMAGE_NAME}" \
        -t "${LATEST_IMAGE}" \
        -f Dockerfile \
        --push \
        .
    
    echo -e "${GREEN}✓ 推送完成: ${FULL_IMAGE_NAME}${NC}"
    echo -e "${GREEN}✓ 推送完成: ${LATEST_IMAGE}${NC}"
    
    # 同时构建本地镜像用于运行
    echo ""
    echo "构建本地镜像用于测试..."
    docker build ${BUILD_ARGS} -t "${LOCAL_IMAGE}" -t "${IMAGE_NAME}:latest" -f Dockerfile .
    RUN_IMAGE="${LOCAL_IMAGE}"
else
    # 无 namespace，仅本地构建
    LOCAL_IMAGE="${IMAGE_NAME}:${VERSION_TAG}"
    
    echo -e "${BLUE}模式: 仅本地构建（不推送）${NC}"
    echo -e "${BLUE}镜像名称: ${LOCAL_IMAGE}${NC}"
    echo ""
    
    docker build ${BUILD_ARGS} -t "${LOCAL_IMAGE}" -t "${IMAGE_NAME}:latest" -f Dockerfile .
    echo -e "${GREEN}✓ 本地构建完成: ${LOCAL_IMAGE}${NC}"
    RUN_IMAGE="${LOCAL_IMAGE}"
fi

echo ""

# ==================== 步骤 4: 本地运行并检查状态 ====================
echo -e "${YELLOW}步骤 4: 本地运行新镜像...${NC}"

# 停止并删除旧容器
if docker ps -a --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
    echo "停止并删除旧容器..."
    docker stop ${CONTAINER_NAME} 2>/dev/null || true
    docker rm ${CONTAINER_NAME} 2>/dev/null || true
    echo -e "${GREEN}✓ 旧容器已清理${NC}"
fi

# 检查配置文件
CONFIG_FILE="$(pwd)/configs/config.prod.yaml"
if [ ! -f "$CONFIG_FILE" ]; then
    echo -e "${YELLOW}警告: 配置文件不存在: ${CONFIG_FILE}${NC}"
    echo -e "${YELLOW}请从 config.example.yaml 复制并配置${NC}"
fi

# 创建日志目录
mkdir -p "$(pwd)/logs"

# 启动新容器
echo "启动新容器..."
docker run -d \
    --name ${CONTAINER_NAME} \
    -p ${HTTP_PORT}:8080 \
    -p ${GRPC_PORT}:9090 \
    -v ${CONFIG_FILE}:/app/configs/config.prod.yaml:ro \
    -v $(pwd)/logs:/app/logs \
    -e APP_ENV=prod \
    --restart unless-stopped \
    ${RUN_IMAGE}

# 等待容器启动
echo "等待容器启动..."
sleep 3

# 检查容器状态
CONTAINER_STATUS=$(docker inspect --format='{{.State.Status}}' ${CONTAINER_NAME} 2>/dev/null || echo "unknown")
CONTAINER_HEALTH=$(docker inspect --format='{{if .State.Health}}{{.State.Health.Status}}{{else}}no-healthcheck{{end}}' ${CONTAINER_NAME} 2>/dev/null || echo "unknown")

echo ""
echo -e "${BLUE}容器运行状态:${NC}"
echo -e "  容器名称: ${CONTAINER_NAME}"
echo -e "  镜像版本: ${RUN_IMAGE}"
echo -e "  运行状态: ${CONTAINER_STATUS}"
echo -e "  健康状态: ${CONTAINER_HEALTH}"

if [ "$CONTAINER_STATUS" = "running" ]; then
    echo -e "${GREEN}✓ 容器运行正常${NC}"
    
    # 尝试健康检查
    echo ""
    echo "执行健康检查..."
    sleep 2
    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:${HTTP_PORT}/health 2>/dev/null || echo "000")
    
    if [ "$HTTP_CODE" = "200" ]; then
        echo -e "${GREEN}✓ 服务响应正常 (HTTP ${HTTP_CODE})${NC}"
    elif [ "$HTTP_CODE" = "000" ]; then
        echo -e "${YELLOW}⚠ 服务暂未响应，可能仍在启动中${NC}"
    else
        echo -e "${YELLOW}⚠ 服务响应异常 (HTTP ${HTTP_CODE})${NC}"
    fi
    
    # 显示容器日志（最后10行）
    echo ""
    echo -e "${BLUE}容器日志（最后10行）:${NC}"
    docker logs --tail 10 ${CONTAINER_NAME} 2>&1 || true
else
    echo -e "${RED}✗ 容器启动失败${NC}"
    echo ""
    echo -e "${BLUE}容器日志:${NC}"
    docker logs ${CONTAINER_NAME} 2>&1 || true
    exit 1
fi

echo ""

# ==================== 完成 ====================
END_TIME=$(date +%s)
END_TIME_STR=$(date '+%Y-%m-%d %H:%M:%S')
DURATION=$((END_TIME - START_TIME))
DURATION_MIN=$((DURATION / 60))
DURATION_SEC=$((DURATION % 60))

echo -e "${BLUE}==========================================${NC}"
echo -e "${GREEN}✓ 所有操作完成！${NC}"
echo -e "${BLUE}==========================================${NC}"
echo ""
echo -e "开始时间:    ${GREEN}${START_TIME_STR}${NC}"
echo -e "结束时间:    ${GREEN}${END_TIME_STR}${NC}"
echo -e "执行耗时:    ${GREEN}${DURATION_MIN}分${DURATION_SEC}秒${NC}"
echo ""
echo -e "Git commit:  ${GREEN}${LOCAL_COMMIT:0:8}${NC}"
echo -e "版本标签:    ${GREEN}${VERSION_TAG}${NC}"
echo -e "本地镜像:    ${GREEN}${RUN_IMAGE}${NC}"
if [ -n "$DOCKER_NAMESPACE" ]; then
    echo -e "远程镜像:    ${GREEN}${FULL_IMAGE_NAME}${NC}"
fi
echo -e "HTTP 地址:   ${GREEN}http://localhost:${HTTP_PORT}${NC}"
echo -e "gRPC 地址:   ${GREEN}localhost:${GRPC_PORT}${NC}"
echo ""
echo -e "${BLUE}==========================================${NC}"
