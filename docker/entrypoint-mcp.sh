#!/bin/sh
set -e

# 检查配置文件是否存在，不存在则从 example 复制
# 注意：MCP 服务运行在 /app 目录下，配置文件在 /app/conf
if [ ! -f "conf/app.conf" ]; then
    echo "Config file not found, copying from example..."
    if [ -f "conf/app.conf.example" ]; then
        cp conf/app.conf.example conf/app.conf
    else
        echo "Warning: conf/app.conf.example not found!"
    fi
fi

echo "Waiting for MySQL..."
while ! nc -z mysql 3306 2>/dev/null; do
    sleep 1
done
echo "MySQL is up! Configuring app.conf..."

# 注入环境变量配置
sed -i "s/^db_host=.*/db_host=$DB_HOST/" conf/app.conf
sed -i "s/^db_port=.*/db_port=$DB_PORT/" conf/app.conf
sed -i "s/^db_username=.*/db_username=$DB_USER/" conf/app.conf
sed -i "s/^db_password=.*/db_password=$DB_PASSWORD/" conf/app.conf
sed -i "s/^db_database=.*/db_database=$DB_DATABASE/" conf/app.conf

echo "Starting MCP Server..."
# 默认启用 SSE 模式
exec /app/mcp-server -addr 0.0.0.0:9090 -transport sse
