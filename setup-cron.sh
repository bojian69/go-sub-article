#!/bin/bash
# 设置 cron 计划任务 - WeChat Subscription Service
# 每隔5分钟执行一次 auto-release.sh
#
# 使用方法:
#   ./setup-cron.sh [docker-namespace]
#
# 示例:
#   ./setup-cron.sh                # 仅本地构建
#   ./setup-cron.sh myusername     # 构建并推送

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# 获取脚本所在目录的绝对路径
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
AUTO_RELEASE_SCRIPT="${SCRIPT_DIR}/auto-release.sh"
LOG_FILE="${SCRIPT_DIR}/auto-release.log"

# Docker namespace（可选）
DOCKER_NAMESPACE="${1:-}"

# cron 标识符，用于识别和更新现有任务
CRON_MARKER="# wechat-subscription-svc-auto-release"

echo -e "${BLUE}==========================================${NC}"
echo -e "${BLUE}  WeChat Subscription Service            ${NC}"
echo -e "${BLUE}       设置 Cron 计划任务                 ${NC}"
echo -e "${BLUE}==========================================${NC}"
echo ""

# 检查 auto-release.sh 是否存在
if [ ! -f "$AUTO_RELEASE_SCRIPT" ]; then
    echo -e "${RED}错误: auto-release.sh 不存在: ${AUTO_RELEASE_SCRIPT}${NC}"
    exit 1
fi

# 确保脚本有执行权限
chmod +x "$AUTO_RELEASE_SCRIPT"

# 构建 cron 命令
if [ -n "$DOCKER_NAMESPACE" ]; then
    CRON_CMD="cd ${SCRIPT_DIR} && ${AUTO_RELEASE_SCRIPT} ${DOCKER_NAMESPACE} >> ${LOG_FILE} 2>&1"
else
    CRON_CMD="cd ${SCRIPT_DIR} && ${AUTO_RELEASE_SCRIPT} >> ${LOG_FILE} 2>&1"
fi

# cron 表达式: 每隔5分钟执行
CRON_SCHEDULE="*/5 * * * *"
CRON_ENTRY="${CRON_SCHEDULE} ${CRON_CMD}"

echo -e "${BLUE}计划任务配置:${NC}"
echo -e "  执行频率: 每隔5分钟"
echo -e "  脚本路径: ${AUTO_RELEASE_SCRIPT}"
echo -e "  日志文件: ${LOG_FILE}"
if [ -n "$DOCKER_NAMESPACE" ]; then
    echo -e "  Docker namespace: ${DOCKER_NAMESPACE}"
else
    echo -e "  Docker namespace: (无，仅本地构建)"
fi
echo ""

# 获取当前 crontab
CURRENT_CRONTAB=$(crontab -l 2>/dev/null || echo "")

# 检查是否已存在相关任务
if echo "$CURRENT_CRONTAB" | grep -q "$CRON_MARKER"; then
    echo -e "${YELLOW}检测到已存在的计划任务，将进行更新...${NC}"
    # 删除旧的任务（删除标记行和下一行）
    NEW_CRONTAB=$(echo "$CURRENT_CRONTAB" | grep -v "$CRON_MARKER" | grep -v "wechat-subscription-svc/auto-release.sh")
else
    NEW_CRONTAB="$CURRENT_CRONTAB"
fi

# 确保末尾有换行
if [ -n "$NEW_CRONTAB" ] && [ "${NEW_CRONTAB: -1}" != $'\n' ]; then
    NEW_CRONTAB="${NEW_CRONTAB}
"
fi

# 添加新任务
NEW_CRONTAB="${NEW_CRONTAB}${CRON_MARKER}
${CRON_ENTRY}"

# 写入 crontab
echo "$NEW_CRONTAB" | crontab -

echo -e "${GREEN}✓ 计划任务已设置${NC}"
echo ""

# 显示当前 crontab
echo -e "${BLUE}当前 crontab 内容:${NC}"
crontab -l | tail -5
echo ""

# 创建日志文件
touch "$LOG_FILE"
echo -e "${GREEN}✓ 日志文件已创建: ${LOG_FILE}${NC}"
echo ""

echo -e "${BLUE}==========================================${NC}"
echo -e "${GREEN}✓ 设置完成！${NC}"
echo -e "${BLUE}==========================================${NC}"
echo ""
echo -e "${BLUE}常用命令:${NC}"
echo "  查看计划任务:  crontab -l"
echo "  查看执行日志:  tail -f ${LOG_FILE}"
echo "  手动执行:      ${AUTO_RELEASE_SCRIPT}"
echo "  强制执行:      ${AUTO_RELEASE_SCRIPT} -f"
echo "  删除计划任务:  crontab -e  # 然后删除相关行"
echo ""
echo -e "${BLUE}==========================================${NC}"
