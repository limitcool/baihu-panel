package router

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"strings"

	"github.com/engigu/baihu-panel/internal/constant"
	"github.com/engigu/baihu-panel/internal/controllers"
	"github.com/engigu/baihu-panel/internal/middleware"
	"github.com/engigu/baihu-panel/internal/models/vo"
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

	initStaticRoutes(root)
	initOpenAPIRoutes(root, urlPrefix)

	// API 路由组
	apiV1 := root.Group("/api/v1")
	initPublicAPIRoutes(apiV1, c)
	initAuthorizedAPIRoutes(apiV1, c)
	initAgentAPIRoutes(root, c)
	initOpenAPIV1Routes(root, c)

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

		serveSPA(ctx, urlPrefix, 200)
	})

	return router
}

func initStaticRoutes(root *gin.RouterGroup) {
	staticFS := static.GetFS()
	if staticFS == nil {
		return
	}

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

func initOpenAPIRoutes(root *gin.RouterGroup, urlPrefix string) {
	// OpenAPI documentation using Scalar UI (带 Basic Auth 认证)
	root.GET("/openapi/*any", func(c *gin.Context) {
		settingsSvc := services.NewSettingsService()
		siteConfig := settingsSvc.GetSection(constant.SectionSite)
		tokenJson := siteConfig[constant.KeyOpenapiToken]

		enabled := false
		if tokenJson != "" {
			var tokenConfig vo.TokenConfig
			if err := json.Unmarshal([]byte(tokenJson), &tokenConfig); err == nil {
				enabled = tokenConfig.Enabled
			}
		}

		// 如果未开启文档，直接返回 404 SPA 页面
		if !enabled {
			serveSPA(c, urlPrefix, 404)
			return
		}

		// 执行认证
		middleware.SwaggerAuth()(c)
		if c.IsAborted() {
			// 如果认证失败（且被中间件置为 404，如密码错误且我们想要隐藏它）
			if c.Writer.Status() == http.StatusNotFound {
				serveSPA(c, urlPrefix, 404)
			}
			return
		}

		// 获取内部路径并标准化（移除前后的所有斜杠）
		path := strings.Trim(c.Param("any"), "/")

		// 1. 根路径或空路径 -> 重定向到 index.html
		if path == "" {
			c.Redirect(http.StatusMovedPermanently, c.Request.URL.Path+"index.html")
			return
		}

		// 2. 提供 Scalar 渲染的 HTML 页面
		if path == "index.html" {
			scalarHTML := `<!doctype html>
<html>
  <head>
    <title>Baihu Panel API Reference</title>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <style>
      body { margin: 0; }
    </style>
  </head>
  <body>
    <script id="api-reference" data-url="` + urlPrefix + `/openapi/doc.json"></script>
    <script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference"></script>
  </body>
</html>`
			c.Header("Content-Type", "text/html; charset=utf-8")
			c.Status(http.StatusOK)
			c.Writer.Write([]byte(scalarHTML))
			c.Abort()
			return
		}

		// 3. 提供给 Scalar/Swagger 使用的 doc.json 内容
		if path == "doc.json" {
			// 这里借助 ginSwagger 仅生成 doc.json 内容
			h := ginSwagger.WrapHandler(swaggerFiles.Handler, ginSwagger.URL(urlPrefix+"/openapi/doc.json"))
			h(c)
			return
		}

		// 其他未匹配路径 -> 返回 404 SPA 页面
		serveSPA(c, urlPrefix, 404)
	})
}

func initPublicAPIRoutes(api *gin.RouterGroup, c *Controllers) {
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
}

func initAuthorizedAPIRoutes(api *gin.RouterGroup, c *Controllers) {
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

		registerTaskRoutes(authorized, c)
		registerEnvRoutes(authorized, c)
		registerScriptRoutes(authorized, c)
		registerFileRoutes(authorized, c)
		registerLogRoutes(authorized, c)
		registerTerminalRoutes(authorized, c)
		registerSettingsRoutes(authorized, c)
		registerDependencyRoutes(authorized, c)
		registerAgentRoutes(authorized, c)
		registerMiseRoutes(authorized, c)
		registerNotificationRoutes(authorized, c)
	}

	// 通知发送 API（使用通知 Token 认证，供脚本调用）
	notifyAPI := api.Group("/notify")
	notifyAPI.Use(middleware.NotifyTokenAuth())
	{
		notifyAPI.POST("/send", c.Notification.SendNotification)
	}
}

func registerTaskRoutes(g *gin.RouterGroup, c *Controllers) {
	tasks := g.Group("/tasks")
	{
		tasks.POST("", c.Task.CreateTask)
		tasks.GET("", c.Task.GetTasks)
		tasks.GET("/:id", c.Task.GetTask)
		tasks.PUT("/:id", c.Task.UpdateTask)
		tasks.DELETE("/:id", c.Task.DeleteTask)
		tasks.POST("/stop/:logID", c.Task.StopTask)
	}

	execution := g.Group("/execute")
	{
		execution.POST("/task/:id", c.Executor.ExecuteTask)
		execution.POST("/command", c.Executor.ExecuteCommand)
		execution.GET("/results", c.Executor.GetLastResults)
	}
}

func registerEnvRoutes(g *gin.RouterGroup, c *Controllers) {
	env := g.Group("/env")
	{
		env.POST("", c.Env.CreateEnvVar)
		env.GET("", c.Env.GetEnvVars)
		env.GET("/all", c.Env.GetAllEnvVars)
		env.GET("/:id", c.Env.GetEnvVar)
		env.GET("/:id/tasks", c.Env.GetAssociatedTasks)
		env.PUT("/:id", c.Env.UpdateEnvVar)
		env.DELETE("/:id", c.Env.DeleteEnvVar)
	}
}

func registerScriptRoutes(g *gin.RouterGroup, c *Controllers) {
	scripts := g.Group("/scripts")
	{
		scripts.POST("", c.Script.CreateScript)
		scripts.GET("", c.Script.GetScripts)
		scripts.GET("/:id", c.Script.GetScript)
		scripts.PUT("/:id", c.Script.UpdateScript)
		scripts.DELETE("/:id", c.Script.DeleteScript)
	}
}

func registerFileRoutes(g *gin.RouterGroup, c *Controllers) {
	files := g.Group("/files")
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
}

func registerLogRoutes(g *gin.RouterGroup, c *Controllers) {
	logs := g.Group("/logs")
	{
		logs.GET("", c.Log.GetLogs)
		logs.POST("/clear", c.Log.ClearLogs)
		logs.GET("/ws", c.LogWS.StreamLog)
		logs.GET("/:id", c.Log.GetLogDetail)
		logs.DELETE("/:id", c.Log.DeleteLog)
	}
}

func registerTerminalRoutes(g *gin.RouterGroup, c *Controllers) {
	g.GET("/terminal/ws", c.Terminal.HandleWebSocket)
	// g.POST("/terminal/exec", c.Terminal.ExecuteShellCommand) // 暂未使用，已注释
	g.GET("/terminal/cmds", c.Terminal.GetCommands)
}

func registerSettingsRoutes(g *gin.RouterGroup, c *Controllers) {
	settings := g.Group("/settings")
	{
		settings.POST("/password", c.Settings.ChangePassword)
		settings.GET("/site", c.Settings.GetSiteSettings)
		settings.PUT("/site", c.Settings.UpdateSiteSettings)
		settings.POST("/site/openapi-token/generate", c.Settings.GenerateOpenapiToken)
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
}

func registerDependencyRoutes(g *gin.RouterGroup, c *Controllers) {
	deps := g.Group("/deps")
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
}

func registerAgentRoutes(g *gin.RouterGroup, c *Controllers) {
	agents := g.Group("/agents")
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

	// Agent API（供前端调用，保持在 v1 下）
	agentAPIv1 := g.Group("/agent")
	{
		agentAPIv1.GET("/download", c.Agent.Download)
	}
}

func registerMiseRoutes(g *gin.RouterGroup, c *Controllers) {
	mise := g.Group("/mise")
	{
		mise.GET("/ls", c.Mise.List)
		mise.POST("/sync", c.Mise.Sync)
		mise.GET("/plugins", c.Mise.Plugins)
		mise.GET("/versions", c.Mise.Versions)
		mise.GET("/verify-cmd", c.Mise.VerifyCommand)
	}
}

func registerNotificationRoutes(g *gin.RouterGroup, c *Controllers) {
	notify := g.Group("/notify")
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

func initAgentAPIRoutes(root *gin.RouterGroup, c *Controllers) {
	// Agent API（供远程 Agent 调用，不使用 /v1 版本号）
	agentAPI := root.Group("/api/agent")
	{
		agentAPI.POST("/heartbeat", c.Agent.Heartbeat)
		agentAPI.GET("/tasks", c.Agent.GetTasks)
		agentAPI.POST("/report", c.Agent.ReportResult)
		agentAPI.GET("/download", c.Agent.Download) // 也在这里注册，兼容 Agent 调用
		agentAPI.GET("/ws", c.Agent.WSConnect)      // WebSocket 连接
	}
}

// serveSPA 注入配置并返回 index.html 给前端渲染
func serveSPA(ctx *gin.Context, urlPrefix string, status int) {
	data, err := static.ReadFile("index.html")
	if err != nil {
		// 如果读不到 index.html (如 dev 模式未 build)，返回基础 HTML
		// 如果已经是 /404 路径，则不再重定向以免死循环
		path := ctx.Request.URL.Path
		if strings.HasSuffix(path, "/404") {
			ctx.Data(status, "text/html; charset=utf-8", []byte("<!DOCTYPE html><html><body><h1>404 Not Found</h1><p>Frontend assets not found. Please run 'npm run build' or check dev server.</p><a href='/'>Go Home</a></body></html>"))
			ctx.Abort()
			return
		}

		fallback := `<!DOCTYPE html><html><head><meta charset="utf-8"/><title>404 Not Found</title></head><body>
			<script>
				const baseUrl = window.__BASE_URL__ || "/";
				if (!window.location.pathname.endsWith("/404")) {
					window.location.href = baseUrl + (baseUrl.endsWith("/") ? "" : "/") + "404";
				}
			</script>
			<p>Not Found. Redirecting...</p>
			</body></html>`
		ctx.Header("Content-Type", "text/html; charset=utf-8")
		ctx.Data(status, "text/html", []byte(fallback))
		ctx.Abort()
		return
	}

	html := string(data)

	// 注入配置变量供前端使用（API 调用和路由）
	configScript := `<script>window.__BASE_URL__ = "` + urlPrefix + `"; window.__API_VERSION__ = "/api/v1";</script>`
	html = strings.Replace(html, "</head>", configScript+"</head>", 1)

	ctx.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	ctx.Data(status, "text/html; charset=utf-8", []byte(html))
	ctx.Abort()
}
