# Internal Token Management Feature - 修改总结

## 问题背景

华为内网 API 需要在请求头中注入动态获取的认证 Token（如 `${token:xxx}` 占位符），需要实现：

1. **Token 管理**：用户配置内部登录端点，自动获取和刷新 Token
2. **Token 注入**：在请求头中自动替换 `${token:name}` 占位符
3. **内网代理**：支持 NO_PROXY 通配符（如 `*.huawei.com`）
4. **渠道测试**：Custom 类型渠道的 URL 路径构建正确

---

## 核心修改

### 1. middleware/auth.go - 会话用户 ID 类型修复

**问题**：session 中存储的用户 ID 类型不一致，导致 `c.GetInt("id")` 读取到错误的值

**修改**：
```go
// 第 151 行 - authHelper 函数中
- c.Set("id", id)
+ c.Set("id", id.(int))

// 第 164 行 - TryUserAuth 函数中
- c.Set("id", id)
+ c.Set("id", id.(int))

// 第 200 行 - TokenOrUserAuth 函数中
- c.Set("id", id)
+ c.Set("id", id.(int))
```

**原因**：session 存储的值是 `int` 类型，但 middleware 原来直接存储可能导致类型不一致。显式转换为 `int` 确保类型正确。

---

### 2. controller/user.go - GetSelf 函数修复

**问题**：`GetSelf` 使用 `c.GetInt64("id")` 但 middleware 存储为 `int`

**修改**：
```go
func GetSelf(c *gin.Context) {
    id := c.GetInt("id")  // 改用 GetInt，与其他控制器一致
    userRole := c.GetInt("role")
    user, err := model.GetUserById(int(id), false)  // 显式转换为 int
    ...
}
```

---

### 3. relay/channel/api_request.go - Token 变量解析

**问题**：请求头中的 `${token:xxx}` 占位符没有被解析

**修改**：在 `processHeaderOverride` 函数中添加 token 解析：
```go
func processHeaderOverride(info *common.RelayInfo, c *gin.Context) (map[string]string, error) {
    headerOverride := make(map[string]string)
    // ... 解析 header_override JSON ...

    for key, value := range headerOverride {
        if strings.HasPrefix(value, "${token:") && strings.HasSuffix(value, "}") {
            value = service.ResolveTokenVariables(value, int(info.UserId))
        }
        headerOverride[key] = value
    }
    return headerOverride, nil
}
```

---

### 4. relay/channel/openai/adaptor.go - Custom 渠道 URL 路径修复

**问题**：Custom 类型渠道的 `GetRequestURL` 没有正确追加请求路径

**修改**：
```go
case constant.ChannelTypeCustom:
    url := info.ChannelBaseUrl
    url = strings.Replace(url, "{model}", info.UpstreamModelName, -1)
    // 新增：处理请求路径
    requestPath := info.RequestURLPath
    if strings.HasPrefix(requestPath, "/v1") {
        requestPath = requestPath[3:] // 移除 /v1 前缀
    }
    if requestPath != "" && !strings.HasSuffix(url, "/") && !strings.HasPrefix(requestPath, "/") {
        url = url + "/"
    }
    if requestPath != "" && !strings.HasSuffix(url, requestPath) {
        url = url + requestPath
    }
    return url, nil
```

**原因**：Custom 渠道的 base_url 可能已包含路径（如 `/api/v2`），需要正确追加 `/chat/completions` 而不是 `/v1/chat/completions`

---

### 5. service/http_client.go - NO_PROXY 通配符支持

**新增功能**：支持 `*.huawei.com` 等通配符模式

**修改**：将 `Proxy: http.ProxyFromEnvironment` 改为自定义函数：
```go
Proxy: proxyFromEnvironmentWithWildcard
```

**新增函数**：
```go
func proxyFromEnvironmentWithWildcard(req *http.Request) (*url.URL, error) {
    noProxy := os.Getenv("NO_PROXY")
    // 支持的通配符模式：
    // - exact match: apigw.huawei.com
    // - suffix wildcard: *.huawei.com
    // - prefix wildcard: 10.*
    // - port stripping: host:port -> host
}
```

---

### 6. router/api-router.go - Token Config 路由注册

**修改**：添加 Token Config API 路由
```go
// Token Config management (internal tokens for header injection)
tokenConfigRoute := userRoute.Group("/token-config")
tokenConfigRoute.Use(middleware.UserAuth())
{
    tokenConfigRoute.GET("/", controller.GetTokenConfigs)
    tokenConfigRoute.POST("/", controller.CreateTokenConfig)
    tokenConfigRoute.PUT("/:id", controller.UpdateTokenConfig)
    tokenConfigRoute.DELETE("/:id", controller.DeleteTokenConfig)
    tokenConfigRoute.POST("/:id/refresh", controller.ManualRefreshToken)
}
```

---

### 7. main.go - 启动 Token 刷新调度器

**修改**：
```go
// 启动内网 Token 自动刷新调度器
go service.StartTokenRefreshScheduler()
```

---

### 8. model/main.go - 数据库迁移

**修改**：在 `migrateDB()` 和 `migrateDBFast()` 中添加 `&TokenConfig{}`

---

### 9. service/token_refresh.go - 新文件

**功能**：Token 刷新服务

主要功能：
- 内存缓存存储 Token
- 30 秒定时检查需要刷新的 Token
- 支持 JSONPath 提取 Token
- 自动处理 Token 过期

---

### 10. model/token_config.go - 新文件

**功能**：Token Config 数据模型

主要字段：
- `Name`: 配置名称
- `LoginURL`: 登录端点
- `LoginMethod`: 登录方法 (POST/GET)
- `LoginHeaders`: 登录请求头 (JSON)
- `LoginBody`: 登录请求体 (支持 `{username}` 和 `{password}` 占位符)
- `Username`: 用户名
- `Password`: 密码
- `TokenJSONPath`: JSONPath 用于提取 Token（如 `$.result.token`）
- `RefreshInterval`: 刷新间隔（秒）
- `CurrentToken`: 当前有效的 Token
- `TokenExpiresAt`: Token 过期时间
- `Enabled`: 是否启用

---

### 11. controller/token_config.go - 新文件

**功能**：Token Config REST API 控制器

端点：
- `GET /api/user/token-config/` - 获取用户所有配置
- `POST /api/user/token-config/` - 创建新配置
- `PUT /api/user/token-config/:id` - 更新配置
- `DELETE /api/user/token-config/:id` - 删除配置
- `POST /api/user/token-config/:id/refresh` - 手动刷新 Token

---

### 12. web/default/src/hooks/use-sidebar-data.ts - 前端菜单

**修改**：添加 Internal Token 菜单项
```typescript
{
    title: t('Internal Token'),
    url: '/internal-token',
    icon: Key,
},
```

---

### 13. web/default/src/routes/_authenticated/internal-token/ - 新目录

**功能**：Internal Token 前端页面

文件：
- `index.tsx` - 路由定义
- 需要在 `web/default/src/features/internal-token/` 下创建完整页面组件

---

## 数据库要求

需要在数据库中添加 `current_token` 列（如果不存在）：
```sql
ALTER TABLE token_configs ADD COLUMN IF NOT EXISTS current_token TEXT;
```

或者删除表重新让 GORM AutoMigrate 创建：
```sql
DROP TABLE IF EXISTS token_configs;
```

---

## 启动环境变量

启动服务时设置 NO_PROXY：
```bash
export NO_PROXY="*.huawei.com,*.inhuawei.com,10.*,100.*,localhost,127.0.0.1"
./new-api-test
```

---

## 验证方法

1. **登录并访问 /internal-token 页面**
2. **创建 Token 配置**：
   - Name: `huawei-sso`
   - Login URL: `https://apigw.huawei.com/api/ssoproxysvr/v2/w3tokens`
   - Login Method: `POST`
   - Login Body: `{"account":"{username}","password":"{password}"}`
   - Token JSON Path: `$.result.token`
   - Username: 你的华为账号
   - Password: 你的华为密码

3. **配置渠道 Custom Header**：
   ```json
   {"X-Auth-Token": "${token:huawei-sso}"}
   ```

4. **测试渠道**应该返回成功

---

## 修改文件清单

| 文件 | 修改类型 | 说明 |
|------|---------|------|
| middleware/auth.go | 修改 | 修复用户 ID 类型存储 |
| controller/user.go | 修改 | GetSelf 使用 GetInt |
| controller/token_config.go | 新增 | Token Config API |
| relay/channel/api_request.go | 修改 | 解析 ${token:} 占位符 |
| relay/channel/openai/adaptor.go | 修改 | Custom 渠道 URL 路径 |
| service/http_client.go | 修改 | NO_PROXY 通配符支持 |
| service/token_refresh.go | 新增 | Token 刷新服务 |
| model/token_config.go | 新增 | Token Config 数据模型 |
| model/main.go | 修改 | 添加迁移 |
| router/api-router.go | 修改 | 注册路由 |
| main.go | 修改 | 启动调度器 |
| web/default/src/hooks/use-sidebar-data.ts | 修改 | 添加菜单 |
| web/default/src/routes/_authenticated/internal-token/ | 新增 | 前端路由 |

---

## 注意事项

1. **所有 c.GetInt("id") 返回 int，c.GetInt64("id") 返回 int64**
2. **Middleware 和 Controller 中用户 ID 类型必须一致**
3. **Custom 渠道的 base_url 应包含完整路径（如 `/api/v2`），不要以 `/` 结尾**
4. **NO_PROXY 设置使用逗号分隔的列表，支持 `*.domain.com` 语法**
