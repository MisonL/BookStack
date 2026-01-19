package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/TruthHun/BookStack/commands"
	"github.com/TruthHun/BookStack/mcp"
	"github.com/astaxie/beego"
	"github.com/astaxie/beego/orm"
	_ "github.com/go-sql-driver/mysql"
)

func main() {
	// 1. 加载配置
	// 默认加载 conf/app.conf
	if err := beego.LoadAppConfig("ini", "conf/app.conf"); err != nil {
		// 如果找不到，尝试上级目录（开发调试用）
		if err := beego.LoadAppConfig("ini", "../conf/app.conf"); err != nil {
			// 再试一次 ../../conf/app.conf
			if err := beego.LoadAppConfig("ini", "../../conf/app.conf"); err != nil {
				fmt.Println("Failed to load app.conf:", err)
				os.Exit(1)
			}
		}
	}

	// 2. 注册 Model（必须在数据库操作前）
	commands.RegisterModel()

	// 3. 初始化数据库连接
	initDatabase()

	// 4. 启动 MCP Server
	// 支持通过环境变量或命令行标志配置监听地址
	addr := flag.String("addr", "0.0.0.0:9090", "MCP server listen address for SSE (optional)")
	transport := flag.String("transport", "stdio", "Transport mode: 'stdio' or 'sse'")
	flag.Parse()

	if *transport == "sse" {
		fmt.Printf("Starting MCP Server (SSE) on %s...\n", *addr)
		if err := mcp.ServeSSE(*addr); err != nil {
			fmt.Println("Server error:", err)
			os.Exit(1)
		}
	} else {
		fmt.Println("Starting MCP Server (Stdio)...")
		if err := mcp.ServeStdio(); err != nil {
			fmt.Println("Server error:", err)
			os.Exit(1)
		}
	}
}

func initDatabase() {
	// Read from ENV or fallback to app.conf
	getEnvOrConf := func(envKey, confKey string) string {
		if val := os.Getenv(envKey); val != "" {
			return val
		}
		return beego.AppConfig.String(confKey)
	}

	adapter := getEnvOrConf("DB_ADAPTER", "db_adapter")
	host := getEnvOrConf("DB_HOST", "db_host")
	port := getEnvOrConf("DB_PORT", "db_port")
	user := getEnvOrConf("DB_USER", "db_username")
	password := getEnvOrConf("DB_PASSWORD", "db_password")
	database := getEnvOrConf("DB_DATABASE", "db_database")

	if adapter == "" {
		adapter = "mysql"
	}

	var dsn string
	if adapter == "mysql" {
		dsn = fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&loc=Local", user, password, host, port, database)
	} else {
		fmt.Println("Unsupported database adapter:", adapter)
		os.Exit(1)
	}

	if err := orm.RegisterDriver(adapter, orm.DRMySQL); err != nil {
		fmt.Println("Failed to register driver:", err)
		os.Exit(1)
	}

	if err := orm.RegisterDataBase("default", adapter, dsn); err != nil {
		fmt.Println("Failed to register database:", err)
		os.Exit(1)
	}

	// 设置数据库前缀（如果 models 需要）
	// BookStack 的 models 使用 conf.GetDatabasePrefix()，它从 beego.AppConfig 读取
	// 这里只要 app.conf 加载正确即可。

	if beego.AppConfig.String("runmode") == "dev" {
		orm.Debug = true
	}
}
