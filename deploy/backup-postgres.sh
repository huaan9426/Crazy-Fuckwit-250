#!/bin/sh

set -eu

DEPLOY_ROOT=${DEPLOY_ROOT:-/srv/crazy-fuckwit-250}
BACKUP_DIR=${BACKUP_DIR:-${DEPLOY_ROOT}/backups}
COMPOSE_FILE=${COMPOSE_FILE:-${DEPLOY_ROOT}/docker-compose.yml}
ENV_FILE=${ENV_FILE:-${DEPLOY_ROOT}/.env}
RETENTION_DAYS=${RETENTION_DAYS:-14}
LOCK_FILE=${LOCK_FILE:-/run/lock/crazy-fuckwit-250-backup.lock}

case "$RETENTION_DAYS" in
  ''|*[!0-9]*)
    echo "RETENTION_DAYS 必须是大于或等于 0 的整数，当前值为：${RETENTION_DAYS}" >&2
    exit 1
    ;;
esac

if [ ! -f "$COMPOSE_FILE" ]; then
  echo "找不到 Docker Compose 配置：${COMPOSE_FILE}" >&2
  exit 1
fi

if [ ! -f "$ENV_FILE" ]; then
  echo "找不到数据库环境变量文件：${ENV_FILE}" >&2
  exit 1
fi

# flock 是 Linux 提供的文件锁工具。定时任务和人工操作可能恰好在同一时间执行，
# 如果两次 pg_dump 同时读取数据库，不会破坏 PostgreSQL 数据，但会重复占用磁盘和
# 数据库资源。这里把文件描述符 9 绑定到固定锁文件，并使用非等待模式取得锁；已有
# 备份正在运行时，本次调用会正常退出，让下一次定时任务再继续，而不是排队堆积。
exec 9>"$LOCK_FILE"
if ! flock -n 9; then
  echo "已有 PostgreSQL 备份正在运行，本次任务不再重复执行。"
  exit 0
fi

install -d -m 700 "$BACKUP_DIR"

timestamp=$(date -u +%Y%m%dT%H%M%SZ)
final_path=${BACKUP_DIR}/crazy_fuckwit_250_${timestamp}.dump
temporary_path=${final_path}.tmp

cleanup() {
  if [ -n "${temporary_path:-}" ] && [ -f "$temporary_path" ]; then
    rm -f "$temporary_path"
  fi
}
trap cleanup EXIT HUP INT TERM

if [ -e "$final_path" ] || [ -e "$temporary_path" ]; then
  echo "同一秒内已经创建过备份，拒绝覆盖现有文件：${final_path}" >&2
  exit 1
fi

# pg_dump 的 custom 格式不是可以直接阅读的 SQL 文本，而是 PostgreSQL 为备份和恢复
# 准备的归档格式。它会保留表结构、约束和数据，并允许 pg_restore 在恢复时选择对象。
# 命令在数据库容器内部执行，因此用户名和数据库名直接读取容器已有的 POSTGRES_USER
# 与 POSTGRES_DB；宿主机不需要解析或打印密码。输出先写入同目录临时文件，只有转储
# 成功且 pg_restore 能读懂归档目录后才通过 mv 原子改名，外部永远不会看到半份备份。
docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" exec -T db \
  sh -c 'exec pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" --format=custom --compress=6 --no-owner --no-privileges' \
  >"$temporary_path"

if [ ! -s "$temporary_path" ]; then
  echo "pg_dump 没有生成有效内容，备份失败。" >&2
  exit 1
fi

docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" exec -T db \
  pg_restore --list <"$temporary_path" >/dev/null

chmod 600 "$temporary_path"
mv "$temporary_path" "$final_path"
temporary_path=

# 保留策略只匹配本脚本固定命名的 .dump 文件，不会清理 .env、部署文件或人工保存的
# 其他归档。RETENTION_DAYS 默认是 14，运维人员可以在临时命令中覆盖它，但无需为了
# 调整保留天数修改脚本。清理放在新备份成功之后，避免一次失败任务先删掉旧的可用副本。
find "$BACKUP_DIR" -type f -name 'crazy_fuckwit_250_*.dump' -mtime "+${RETENTION_DAYS}" -delete

backup_size=$(du -h "$final_path" | awk '{print $1}')
echo "PostgreSQL 备份完成：${final_path}（${backup_size}）"
