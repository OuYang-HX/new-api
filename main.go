package main

import (
	"bytes"
	"embed"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/custom" // custom-hook: decoupled extensions
	"github.com/QuantumNous/new-api/custom/codex" // custom-hook: codex scheduler injection
	"github.com/QuantumNous/new-api/custom/token_config" // custom-hook: token config channel ops
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/oauth"
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	"github.com/QuantumNous/new-api/relay"
	"github.com/QuantumNous/new-api/router"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/authz"
	_ "github.com/QuantumNous/new-api/setting/performance_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	_ "net/http/pprof"
)

//go:embed web/default/dist
var buildFS embed.FS

//go:embed web/default/dist/index.html
var indexPage []byte

//go:embed web/classic/dist
var classicBuildFS embed.FS

//go:embed web/classic/dist/index.html
var classicIndexPage []byte

func main() {
	startTime := time.Now()

	err := InitResources()
	if err != nil {
		common.FatalLog("failed to initialize resources: " + err.Error())
		return
	}

	common.SysLog("New API " + common.Version + " started")
	if os.Getenv("GIN_MODE") != "debug" {
		gin.SetMode(gin.ReleaseMode)
	}
	if common.DebugEnabled {
		common.SysLog("running in debug mode")
	}

	defer func() {
		err := model.CloseDB()
		if err != nil {
			common.FatalLog("failed to close database: " + err.Error())
		}
	}()

	if common.RedisEnabled {
		// for compatibility with old versions
		common.MemoryCacheEnabled = true
	}
	if common.MemoryCacheEnabled {
		common.SysLog("memory cache enabled")
		common.SysLog(fmt.Sprintf("sync frequency: %d seconds", common.SyncFrequency))

		// Add panic recovery and retry for InitChannelCache
		func() {
			defer func() {
				if r := recover(); r != nil {
					common.SysLog(fmt.Sprintf("InitChannelCache panic: %v, retrying once", r))
					// Retry once
					_, _, fixErr := model.FixAbility()
					if fixErr != nil {
						common.FatalLog(fmt.Sprintf("InitChannelCache failed: %s", fixErr.Error()))
					}
				}
			}()
			model.InitChannelCache()
		}()

		go model.SyncChannelCache(common.SyncFrequency)
	}

	// 热更新配置
	go model.SyncOptions(common.SyncFrequency)

	// 周期性重载授权策略，保证多节点/多 master 部署下权限变更能传播到每个实例
	go authz.StartPolicySync(common.SyncFrequency)

	// 数据看板
	go model.UpdateQuotaData()

	if os.Getenv("CHANNEL_UPDATE_FREQUENCY") != "" {
		frequency, err := strconv.Atoi(os.Getenv("CHANNEL_UPDATE_FREQUENCY"))
		if err != nil {
			common.FatalLog("failed to parse CHANNEL_UPDATE_FREQUENCY: " + err.Error())
		}
		go controller.AutomaticallyUpdateChannels(frequency)
	}

	// custom-hook: Codex credential auto-refresh (moved to custom/codex)
	codex.StartCodexCredentialAutoRefreshTask()

	// Subscription quota reset task (daily/weekly/monthly/custom)
	service.StartSubscriptionQuotaResetTask()
	// custom-hook: inject codex scheduler (avoids import cycle: custom → custom/codex → model → custom)
	custom.SchedulerFuncs.StartCodexCredentialAutoRefreshTask = codex.StartCodexCredentialAutoRefreshTask
	// custom-hook: inject channel operation functions (avoids import cycle: custom → model → custom)
	// IMPORTANT: Must be set BEFORE model.InitDB() which calls custom.RegisterMigrations → initChannelOps
	custom.ChannelOperationFuncs.CloneFromTemplate = cloneChannelFromTemplate
	custom.ChannelOperationFuncs.UpdateNameAndKey = updateChannelNameAndKey
	custom.ChannelOperationFuncs.Delete = deleteChannelById
	custom.ChannelOperationFuncs.GetById = getChannelNameById
	custom.ChannelOperationFuncs.SyncFromTemplate = syncChannelsFromTemplate
	custom.ChannelOperationFuncs.GetDisabledChannels = getDisabledChannels
	// custom-hook: start custom background schedulers (includes codex credential refresh)
	custom.StartSchedulers()

	// custom-hook: initialize protocol adapter (must be before router setup)
	custom.InitProtocolAdapter(controller.Relay, model.GetEnabledModels)

	// Report this process as a system instance so the System Info page can show
	// all currently alive nodes in multi-instance deployments.
	service.StartSystemInstanceReporter()

	// Wire task polling adaptor factory (breaks service -> relay import cycle).
	// Must run before the system task runner starts: the async_task_poll handler
	// calls service.RunTaskPollingOnce, which needs this factory set.
	service.GetTaskAdaptorFunc = func(platform constant.TaskPlatform) service.TaskPollingAdaptor {
		a := relay.GetTaskAdaptor(platform)
		if a == nil {
			return nil
		}
		return a
	}

	// Register the periodic channel test, upstream model update, and async task
	// polling (Midjourney / Suno / video) jobs as scheduled system tasks
	// (DB-lease dedup across masters + run history), then start the runner that
	// schedules and executes them. Master-only execution and the UpdateTask
	// switch are enforced inside the runner and each handler's Enabled().
	controller.RegisterScheduledSystemTasks()
	service.StartSystemTaskRunner()

	if os.Getenv("BATCH_UPDATE_ENABLED") == "true" {
		common.BatchUpdateEnabled = true
		common.SysLog("batch update enabled with interval " + strconv.Itoa(common.BatchUpdateInterval) + "s")
		model.InitBatchUpdater()
	}

	if os.Getenv("ENABLE_PPROF") == "true" {
		gopool.Go(func() {
			log.Println(http.ListenAndServe("0.0.0.0:8005", nil))
		})
		go common.Monitor()
		common.SysLog("pprof enabled")
	}

	err = common.StartPyroScope()
	if err != nil {
		common.SysError(fmt.Sprintf("start pyroscope error : %v", err))
	}

	// Initialize HTTP server
	server := gin.New()
	server.Use(gin.CustomRecovery(func(c *gin.Context, err any) {
		common.SysLog(fmt.Sprintf("panic detected: %v", err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"message": fmt.Sprintf("Panic detected, error: %v. Please submit a issue here: https://github.com/Calcium-Ion/new-api", err),
				"type":    "new_api_panic",
			},
		})
	}))
	// This will cause SSE not to work!!!
	//server.Use(gzip.Gzip(gzip.DefaultCompression))
	server.Use(middleware.RequestId())
	server.Use(middleware.PoweredBy())
	server.Use(middleware.I18n())
	middleware.SetUpLogger(server)
	// Initialize session store
	store := cookie.NewStore([]byte(common.SessionSecret))
	store.Options(sessions.Options{
		Path:     "/",
		MaxAge:   2592000, // 30 days
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteStrictMode,
	})

	// 根据 PORT 自动生成不同的 session cookie 名称
	sessionName := "session"
	if port := os.Getenv("PORT"); port != "" {
		sessionName = "session_" + port
	}
	server.Use(sessions.Sessions(sessionName, store))

	InjectUmamiAnalytics()
	InjectGoogleAnalytics()

	// 设置路由
	router.SetRouter(server, router.ThemeAssets{
		DefaultBuildFS:   buildFS,
		DefaultIndexPage: indexPage,
		ClassicBuildFS:   classicBuildFS,
		ClassicIndexPage: classicIndexPage,
	})
	var port = os.Getenv("PORT")
	if port == "" {
		port = strconv.Itoa(*common.Port)
	}

	// Log startup success message
	common.LogStartupSuccess(startTime, port)

	err = server.Run(":" + port)
	if err != nil {
		common.FatalLog("failed to start HTTP server: " + err.Error())
	}
}

func InjectUmamiAnalytics() {
	analyticsInjectBuilder := &strings.Builder{}
	if os.Getenv("UMAMI_WEBSITE_ID") != "" {
		umamiSiteID := os.Getenv("UMAMI_WEBSITE_ID")
		umamiScriptURL := os.Getenv("UMAMI_SCRIPT_URL")
		if umamiScriptURL == "" {
			umamiScriptURL = "https://analytics.umami.is/script.js"
		}
		analyticsInjectBuilder.WriteString("<script defer src=\"")
		analyticsInjectBuilder.WriteString(umamiScriptURL)
		analyticsInjectBuilder.WriteString("\" data-website-id=\"")
		analyticsInjectBuilder.WriteString(umamiSiteID)
		analyticsInjectBuilder.WriteString("\"></script>")
	}
	analyticsInjectBuilder.WriteString("<!--Umami QuantumNous-->\n")
	analyticsInject := []byte(analyticsInjectBuilder.String())
	placeholder := []byte("<!--umami-->\n")
	indexPage = bytes.ReplaceAll(indexPage, placeholder, analyticsInject)
	classicIndexPage = bytes.ReplaceAll(classicIndexPage, placeholder, analyticsInject)
}

func InjectGoogleAnalytics() {
	analyticsInjectBuilder := &strings.Builder{}
	if os.Getenv("GOOGLE_ANALYTICS_ID") != "" {
		gaID := os.Getenv("GOOGLE_ANALYTICS_ID")
		// Google Analytics 4 (gtag.js)
		analyticsInjectBuilder.WriteString("<script async src=\"https://www.googletagmanager.com/gtag/js?id=")
		analyticsInjectBuilder.WriteString(gaID)
		analyticsInjectBuilder.WriteString("\"></script>")
		analyticsInjectBuilder.WriteString("<script>")
		analyticsInjectBuilder.WriteString("window.dataLayer = window.dataLayer || [];")
		analyticsInjectBuilder.WriteString("function gtag(){dataLayer.push(arguments);}")
		analyticsInjectBuilder.WriteString("gtag('js', new Date());")
		analyticsInjectBuilder.WriteString("gtag('config', '")
		analyticsInjectBuilder.WriteString(gaID)
		analyticsInjectBuilder.WriteString("');")
		analyticsInjectBuilder.WriteString("</script>")
	}
	analyticsInjectBuilder.WriteString("<!--Google Analytics QuantumNous-->\n")
	analyticsInject := []byte(analyticsInjectBuilder.String())
	placeholder := []byte("<!--Google Analytics-->\n")
	indexPage = bytes.ReplaceAll(indexPage, placeholder, analyticsInject)
	classicIndexPage = bytes.ReplaceAll(classicIndexPage, placeholder, analyticsInject)
}

func InitResources() error {
	// Initialize resources here if needed
	// This is a placeholder function for future resource initialization
	err := godotenv.Load(".env")
	if err != nil {
		if common.DebugEnabled {
			common.SysLog("No .env file found, using default environment variables. If needed, please create a .env file and set the relevant variables.")
		}
	}

	// 加载环境变量
	common.InitEnv()

	logger.SetupLogger()

	// Initialize model settings
	ratio_setting.InitRatioSettings()

	service.InitHttpClient()

	service.InitTokenEncoders()

	// Initialize SQL Database
	err = model.InitDB()
	if err != nil {
		common.FatalLog("failed to initialize database: " + err.Error())
		return err
	}
	if err = authz.Init(model.DB); err != nil {
		common.FatalLog("failed to initialize authorization: " + err.Error())
		return err
	}

	model.CheckSetup()

	// Initialize options, should after model.InitDB()
	model.InitOptionMap()

	// 清理旧的磁盘缓存文件
	common.CleanupOldCacheFiles()

	// 初始化模型
	model.GetPricing()

	// Initialize SQL Database
	err = model.InitLogDB()
	if err != nil {
		return err
	}

	// Initialize Redis
	err = common.InitRedisClient()
	if err != nil {
		return err
	}

	perfmetrics.Init()

	// 启动系统监控
	common.StartSystemMonitor()

	// Initialize i18n
	err = i18n.Init()
	if err != nil {
		common.SysError("failed to initialize i18n: " + err.Error())
		// Don't return error, i18n is not critical
	} else {
		common.SysLog("i18n initialized with languages: " + strings.Join(i18n.SupportedLanguages(), ", "))
	}
	// Register user language loader for lazy loading
	i18n.SetUserLangLoader(model.GetUserLanguage)

	// Load custom OAuth providers from database
	err = oauth.LoadCustomProviders()
	if err != nil {
		common.SysError("failed to load custom OAuth providers: " + err.Error())
		// Don't return error, custom OAuth is not critical
	}

	return nil
}

// cloneChannelFromTemplate clones a disabled channel (the "channel template") and creates
// a new enabled channel for the given user. The clone gets:
//   - Key replaced by ${token:<username>}
//   - Name set to "<template_channel_name>-<username>"
//   - Status set to enabled (1)
//   - HeaderOverride: ${token:self} replaced with ${token:<username>}
// This function lives in main to avoid import cycles (custom -> model -> custom).
func cloneChannelFromTemplate(channelTemplateId int, username string) (int, error) {
	tmpl, err := model.GetChannelById(channelTemplateId, true)
	if err != nil {
		return 0, fmt.Errorf("load channel template %d: %w", channelTemplateId, err)
	}

	channel := *tmpl // shallow copy all fields
	channel.Id = 0    // let GORM auto-generate
	channel.Key = fmt.Sprintf("${token:%s:%s}", tmpl.Name, username)
	channel.Name = fmt.Sprintf("%s-%s", tmpl.Name, username)
	channel.Status = common.ChannelStatusEnabled
	channel.UsedQuota = 0
	channel.Balance = 0
	channel.BalanceUpdatedTime = 0
	channel.TestTime = 0
	channel.ResponseTime = 0
	channel.CreatedTime = common.GetTimestamp()
	channel.ChannelInfo = model.ChannelInfo{} // reset multi-key state

	// Replace ${token:self} in header_override with the actual username reference
	if channel.HeaderOverride != nil && *channel.HeaderOverride != "" {
		ho := strings.ReplaceAll(*channel.HeaderOverride, "${token:self}", fmt.Sprintf("${token:%s:%s}", tmpl.Name, username))
		channel.HeaderOverride = &ho
	}

	if err := channel.Insert(); err != nil {
		return 0, fmt.Errorf("insert channel: %w", err)
	}

	return channel.Id, nil
}

// updateChannelNameAndKey updates the name and key of a channel by ID.
func updateChannelNameAndKey(channelId int, templateName string, username string) {
	ch, err := model.GetChannelById(channelId, true)
	if err != nil {
		return
	}
	ch.Name = fmt.Sprintf("%s-%s", templateName, username)
	ch.Key = fmt.Sprintf("${token:%s:%s}", templateName, username)
	_ = ch.Update()
}

// deleteChannelById deletes a channel by ID.
func deleteChannelById(channelId int) {
	ch, err := model.GetChannelById(channelId, true)
	if err != nil {
		return
	}
	_ = ch.Delete()
}

// getChannelNameById returns the channel name for a given ID.
func getChannelNameById(channelId int) string {
	ch, err := model.GetChannelById(channelId, true)
	if err != nil {
		return ""
	}
	return ch.Name
}

// syncChannelsFromTemplate re-clones shared fields from the channel template to all
// auto-created channels. Per-user fields (Key, Name, HeaderOverride) are preserved.
// We iterate all TokenConfigs and update their linked channels with the template's fields.
func syncChannelsFromTemplate(channelTemplateId int, username string) error {
	tmpl, err := model.GetChannelById(channelTemplateId, true)
	if err != nil {
		return fmt.Errorf("load channel template %d: %w", channelTemplateId, err)
	}

	// Get all TokenConfigs that have a linked channel
	configs, err := token_config.GetAllTokenConfigsFromDB()
	if err != nil {
		return fmt.Errorf("load token configs: %w", err)
	}

	for _, cfg := range configs {
		if cfg.ChannelId <= 0 {
			continue
		}
		ch, err := model.GetChannelById(cfg.ChannelId, true)
		if err != nil {
			continue
		}
		// Copy shared fields from template channel
		ch.Type = tmpl.Type
		ch.Models = tmpl.Models
		ch.Group = tmpl.Group
		ch.AutoBan = tmpl.AutoBan
		ch.BaseURL = tmpl.BaseURL
		ch.Priority = tmpl.Priority
		ch.Weight = tmpl.Weight
		ch.Tag = tmpl.Tag
		ch.ModelMapping = tmpl.ModelMapping
		ch.Other = tmpl.Other
		ch.Setting = tmpl.Setting
		ch.ParamOverride = tmpl.ParamOverride
		ch.StatusCodeMapping = tmpl.StatusCodeMapping
		ch.OtherSettings = tmpl.OtherSettings
		ch.OpenAIOrganization = tmpl.OpenAIOrganization
		ch.TestModel = tmpl.TestModel
		// Per-user fields: preserve with user's own username
		ch.Name = fmt.Sprintf("%s-%s", tmpl.Name, cfg.Username)
		ch.Key = fmt.Sprintf("${token:%s:%s}", tmpl.Name, cfg.Username)
		if tmpl.HeaderOverride != nil && *tmpl.HeaderOverride != "" {
			ho := strings.ReplaceAll(*tmpl.HeaderOverride, "${token:self}", fmt.Sprintf("${token:%s:%s}", tmpl.Name, cfg.Username))
			ch.HeaderOverride = &ho
		} else {
			ch.HeaderOverride = tmpl.HeaderOverride
		}
		_ = ch.Update()
	}
	return nil
}

// getDisabledChannels returns all manually disabled channels that can be used as channel templates.
func getDisabledChannels() []token_config.DisabledChannelItem {
	channels, err := model.GetAllChannels(0, 0, true, false)
	if err != nil {
		return nil
	}
	var result []token_config.DisabledChannelItem
	for _, ch := range channels {
		if ch.Status == common.ChannelStatusManuallyDisabled {
			result = append(result, token_config.DisabledChannelItem{
				Id:   ch.Id,
				Name: ch.Name,
				Type: ch.Type,
			})
		}
	}
	return result
}

