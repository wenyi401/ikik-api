# datamanagementd 部署说明（数据管理）

本文说明如何在宿主机部署 `datamanagementd`，并与主进程联动开启“数据管理”功能。

## 1. 关键约束

- 主进程固定探测路径：`/tmp/ikik-api-datamanagement.sock`
- 仅当该 Unix Socket 可连通且 `Health` 成功时，后台“数据管理”才会启用
- `datamanagementd` 使用 SQLite 持久化元数据，不依赖主库

## 2. 宿主机构建与运行

```bash
cd /opt/ikik-api-src/datamanagement
go build -o /opt/ikik-api/datamanagementd ./cmd/datamanagementd

mkdir -p /var/lib/ikik-api/datamanagement
chown -R ikik-api:ikik-api /var/lib/ikik-api/datamanagement
```

手动启动示例：

```bash
/opt/ikik-api/datamanagementd \
  -socket-path /tmp/ikik-api-datamanagement.sock \
  -sqlite-path /var/lib/ikik-api/datamanagement/datamanagementd.db \
  -version 1.0.0
```

## 3. systemd 托管（推荐）

仓库已提供示例服务文件：`deploy/ikik-api-datamanagementd.service`

```bash
sudo cp deploy/ikik-api-datamanagementd.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now ikik-api-datamanagementd
sudo systemctl status ikik-api-datamanagementd
```

查看日志：

```bash
sudo journalctl -u ikik-api-datamanagementd -f
```

也可以使用一键安装脚本（自动安装二进制 + 注册 systemd）：

```bash
# 方式一：使用现成二进制
sudo ./deploy/install-datamanagementd.sh --binary /path/to/datamanagementd

# 方式二：从源码构建后安装
sudo ./deploy/install-datamanagementd.sh --source /path/to/ikik-api
```

## 4. Docker 部署联动

若 `ikik-api` 运行在 Docker 容器中，需要将宿主机 Socket 挂载到容器同路径：

```yaml
services:
  ikik-api:
    volumes:
      - /tmp/ikik-api-datamanagement.sock:/tmp/ikik-api-datamanagement.sock
```

建议在 `docker-compose.override.yml` 中维护该挂载，避免覆盖主 compose 文件。

## 5. 依赖检查

`datamanagementd` 执行备份时依赖以下工具：

- `pg_dump`
- `redis-cli`
- `docker`（仅 `source_mode=docker_exec` 时）

缺失依赖会导致对应任务失败，并在任务详情中体现错误信息。
