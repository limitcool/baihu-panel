package router

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/engigu/baihu-panel/internal/controllers"
	"github.com/engigu/baihu-panel/internal/middleware"
	"github.com/engigu/baihu-panel/internal/services"
	"github.com/engigu/baihu-panel/internal/static"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

type Controllers struct {
	Task         *controllers.TaskController
	Auth         *controllers.AuthController
	Env          *controllers.EnvController
	Script       *controllers.ScriptController
	Executor     *controllers.ExecutorController
	File         *controllers.FileController
	Dashboard    *controllers.DashboardController
	Log          *controllers.LogController
	LogWS        *controllers.LogWSController
	Terminal     *controllers.TerminalController
	Settings     *controllers.SettingsController
	Dependency   *controllers.DependencyController
	Agent        *controllers.AgentController
	Mise         *controllers.MiseController
	Notification *controllers.NotificationController
}

func mustSubFS(fsys fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		panic(err)
	}
	return sub
}

// cacheControl 返回设置 Cache-Control header 的中间件
func cacheControl(value string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", value)
		c.Next()
	}
}

func Setup(c *Controllers) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(middleware.GinLogger(), middleware.GinRecovery())

	// 获取 URL 前缀
	cfg := services.GetConfig()
	urlPrefix := strings.TrimSuffix(cfg.Server.URLPrefix, "/")

	// 创建一个路由组，如果有前缀则使用前缀，否则使用根路径
	var root *gin.RouterGroup
	if urlPrefix != "" {
		root = router.Group(urlPrefix)
	} else {
		root = router.Group("")
	}

	staticFS := static.GetFS()
	if staticFS != nil {
		// 静态资源服务（Vue SPA），带缓存头部
		assetsGroup := root.Group("/assets")
		assetsGroup.Use(cacheControl("public, max-age=31536000, immutable")) // 带哈希的资源缓存
		assetsGroup.StaticFS("/", http.FS(mustSubFS(staticFS, "assets")))

		// logo.svg 短缓存实现
		root.GET("/logo.svg", func(ctx *gin.Context) {
			data, err := static.ReadFile("logo.svg")
			if err != nil {
				ctx.Status(404)
				return
			}
			ctx.Header("Cache-Control", "public, max-age=86400") // 缓存1天
			ctx.Data(200, "image/svg+xml", data)
		})
	}

	// Swagger documentation (带 Basic Auth 认证)
	root.GET("/swagger/*any", middleware.SwaggerAuth(), ginSwagger.WrapHandler(swaggerFiles.Handler))

	// API 路由组
	api := root.Group("/api/v1")
	{
		// Health check (无需认证)
		api.GET("/ping", func(ctx *gin.Context) {
			ctx.JSON(200, gin.H{"message": "pong"})
		})

		// Authentication routes (无需认证)
		auth := api.Group("/auth")
		{
			auth.POST("/login", c.Auth.Login)
			auth.POST("/logout", c.Auth.Logout)
			auth.POST("/register", c.Auth.Register)
		}

		// 公开的站点设置（无需认证）
		api.GET("/settings/public", c.Settings.GetPublicSiteSettings)

		// 需要认证的路由
		authorized := api.Group("")
		authorized.Use(middleware.AuthRequired())
		{
			// 获取当前用户
			authorized.GET("/auth/me", c.Auth.GetCurrentUser)

			// 仪表盘统计
			authorized.GET("/stats", c.Dashboard.GetStats)
			authorized.GET("/sentence", c.Dashboard.GetSentence)
			authorized.GET("/sendstats", c.Dashboard.GetSendStats)
			authorized.GET("/taskstats", c.Dashboard.GetTaskStats)

			// 任务模块
			tasks := authorized.Group("/tasks")
			{
				tasks.POST("", c.Task.CreateTask)
				tasks.GET("", c.Task.GetTasks)
				tasks.GET("/:id", c.Task.GetTask)
				tasks.PUT("/:id", c.Task.UpdateTask)
				tasks.DELETE("/:id", c.Task.DeleteTask)
				tasks.POST("/stop/:logID", c.Task.StopTask)
			}

			// 任务执行模块
			execution := authorized.Group("/execute")
			{
				execution.POST("/task/:id", c.Executor.ExecuteTask)
				execution.POST("/command", c.Executor.ExecuteCommand)
				execution.GET("/results", c.Executor.GetLastResults)
			}

			// 环境变量模块
			env := authorized.Group("/env")
			{
				env.POST("", c.Env.CreateEnvVar)
				env.GET("", c.Env.GetEnvVars)
				env.GET("/all", c.Env.GetAllEnvVars)
				env.GET("/:id", c.Env.GetEnvVar)
				env.GET("/:id/tasks", c.Env.GetAssociatedTasks)
				env.PUT("/:id", c.Env.UpdateEnvVar)
				env.DELETE("/:id", c.Env.DeleteEnvVar)
			}

			// 脚本模块
			scripts := authorized.Group("/scripts")
			{
				scripts.POST("", c.Script.CreateScript)
				scripts.GET("", c.Script.GetScripts)
				scripts.GET("/:id", c.Script.GetScript)
				scripts.PUT("/:id", c.Script.UpdateScript)
				scripts.DELETE("/:id", c.Script.DeleteScript)
			}

			// 文件管理模块
			files := authorized.Group("/files")
			{
				files.GET("/tree", c.File.GetFileTree)
				files.GET("/content", c.File.GetFileContent)
				files.GET("/download", c.File.DownloadFile)
				files.POST("/content", c.File.SaveFileContent)
				files.POST("/create", c.File.CreateFile)
				files.POST("/delete", c.File.DeleteFile)
				files.POST("/rename", c.File.RenameFile)
				files.POST("/move", c.File.MoveFile)
				files.POST("/copy", c.File.CopyFile)
				files.POST("/upload", c.File.UploadArchive)
				files.POST("/uploadfiles", c.File.UploadFiles)
			}

			// 日志查看模块
			logs := authorized.Group("/logs")
			{
				logs.GET("", c.Log.GetLogs)
				logs.POST("/clear", c.Log.ClearLogs)
				logs.GET("/ws", c.LogWS.StreamLog)
				logs.GET("/:id", c.Log.GetLogDetail)
				logs.DELETE("/:id", c.Log.DeleteLog)
			}

			// 终端模块
			authorized.GET("/terminal/ws", c.Terminal.HandleWebSocket)
			authorized.POST("/terminal/exec", c.Terminal.ExecuteShellCommand)
			authorized.GET("/terminal/cmds", c.Terminal.GetCommands)

			// 设置中心模块
			settings := authorized.Group("/settings")
			{
				settings.POST("/password", c.Settings.ChangePassword)
				settings.GET("/site", c.Settings.GetSiteSettings)
				settings.PUT("/site", c.Settings.UpdateSiteSettings)
				settings.POST("/site/api-token/generate", c.Settings.GenerateApiToken)
				settings.GET("/paths", c.Settings.GetPaths)
				settings.GET("/scheduler", c.Settings.GetSchedulerSettings)
				settings.PUT("/scheduler", c.Settings.UpdateSchedulerSettings)
				settings.GET("/about", c.Settings.GetAbout)
				settings.GET("/loginlogs", c.Settings.GetLoginLogs)
				settings.POST("/backup", c.Settings.CreateBackup)
				settings.GET("/backup/status", c.Settings.GetBackupStatus)
				settings.GET("/backup/download", c.Settings.DownloadBackup)
				settings.POST("/restore", c.Settings.RestoreBackup)
				// 通用设置接口
				settings.GET("/:section/:key", c.Settings.GetSetting)
				settings.POST("/:section/:key/generate", c.Settings.GenerateSettingToken)
			}

			// Dependency routes (依赖管理)
			deps := authorized.Group("/deps")
			{
				deps.GET("", c.Dependency.List)
				deps.POST("", c.Dependency.Create)
				deps.DELETE("/:id", c.Dependency.Delete)
				deps.POST("/install", c.Dependency.Install)
				deps.POST("/install-cmd", c.Dependency.GetInstallCommand)
				deps.POST("/uninstall/:id", c.Dependency.Uninstall)
				deps.POST("/reinstall/:id", c.Dependency.Reinstall)
				deps.POST("/reinstall-all", c.Dependency.ReinstallAll)
				deps.POST("/reinstall-all-cmd", c.Dependency.GetReinstallAllCommand)
				deps.GET("/installed", c.Dependency.GetInstalled)
			}

			// Agent routes (Agent 管理)
			agents := authorized.Group("/agents")
			{
				agents.GET("", c.Agent.List)
				agents.GET("/version", c.Agent.GetVersion)
				agents.PUT("/:id", c.Agent.Update)
				agents.DELETE("/:id", c.Agent.Delete)
				agents.POST("/:id/token", c.Agent.RegenerateToken)
				agents.POST("/:id/update", c.Agent.ForceUpdate)
				// 令牌管理
				agents.GET("/tokens", c.Agent.ListTokens)
				agents.POST("/tokens", c.Agent.CreateToken)
				agents.DELETE("/tokens/:id", c.Agent.DeleteToken)
			}

			// Mise routes (Mise 管理)
			mise := authorized.Group("/mise")
			{
				mise.GET("/ls", c.Mise.List)
				mise.POST("/sync", c.Mise.Sync)
				mise.GET("/plugins", c.Mise.Plugins)
				mise.GET("/versions", c.Mise.Versions)
				mise.GET("/verify-cmd", c.Mise.VerifyCommand)
			}

			// Agent API（供前端调用，保持在 v1 下）
			agentAPIv1 := authorized.Group("/agent")
			{
				agentAPIv1.GET("/download", c.Agent.Download)
			}

			// 通知推送模块
			notify := authorized.Group("/notify")
			{
				notify.GET("/types", c.Notification.GetChannelTypes)
				notify.GET("/channels", c.Notification.GetChannels)
				notify.POST("/channels", c.Notification.SaveChannel)
				notify.DELETE("/channels/:id", c.Notification.DeleteChannel)
				notify.POST("/channels/test", c.Notification.TestChannel)
				notify.GET("/bindings", c.Notification.GetBindings)
				notify.POST("/bindings", c.Notification.SaveBinding)
				notify.DELETE("/bindings/:id", c.Notification.DeleteBinding)
			}
		}

		// 通知发送 API（使用通知 Token 认证，供脚本调用）
		notifyAPI := api.Group("/notify")
		notifyAPI.Use(middleware.NotifyTokenAuth())
		{
			notifyAPI.POST("/send", c.Notification.SendNotification)
		}
	}

	// Agent API（供远程 Agent 调用，不使用 /v1 版本号）
	agentAPI := root.Group("/api/agent")
	{
		agentAPI.POST("/heartbeat", c.Agent.Heartbeat)
		agentAPI.GET("/tasks", c.Agent.GetTasks)
		agentAPI.POST("/report", c.Agent.ReportResult)
		agentAPI.GET("/download", c.Agent.Download) // 也在这里注册，兼容 Agent 调用
		agentAPI.GET("/ws", c.Agent.WSConnect)      // WebSocket 连接
	}

	// SPA 兜底路由 - 返回 index.html（HTML禁用缓存以保证实时同步）
	// 必须在最后注册，作为兜底路由
	router.NoRoute(func(ctx *gin.Context) {
		path := ctx.Request.URL.Path

		// 如果配置了前缀，只处理带前缀的路径
		if urlPrefix != "" && !strings.HasPrefix(path, urlPrefix) {
			ctx.Status(404)
			return
		}

		// 解析实际的相对路径
		relPath := strings.TrimPrefix(path, urlPrefix)
		if !strings.HasPrefix(relPath, "/") {
			relPath = "/" + relPath
		}

		// 拦截属于 API 或静态资源目录下不存在的请求，绝不能返回 index.html 造成前端 MIME 错误
		if strings.HasPrefix(relPath, "/api/") || strings.HasPrefix(relPath, "/assets/") {
			ctx.String(404, "Not Found")
			return
		}

		data, err := static.ReadFile("index.html")
		if err != nil {
			ctx.Status(404)
			return
		}

		html := string(data)

		// 注入配置变量供前端使用（API 调用和路由）
		configScript := `<script>window.__BASE_URL__ = "` + urlPrefix + `"; window.__API_VERSION__ = "/api/v1";</script>`
		html = strings.Replace(html, "</head>", configScript+"</head>", 1)

		ctx.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		ctx.Data(200, "text/html; charset=utf-8", []byte(html))
	})

	return router
}
