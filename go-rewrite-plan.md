# go-rewrite 缺失功能补齐计划

## 目标
将 go-rewrite 分支从约 40% 还原度提升至 1:1 还原 master 分支。

## 优先级（P0 = 最高，P3 = 最低）

### P0 — 核心数据流（不补齐则应用无法正常工作）
1. **下载系统 (`download.js`)**
   - URL 参数解析 (`#noFlow`, `#arguments`, `cacheKey`, `headers`)
   - GitHub 代理加速 (`githubProxy`, `githubProxyRegex`)
   - 本地文件读取 (`/api/file/*`, `/api/module/*`)
   - 乐观缓存（乐观读取 + 后台更新）
   - 自定义缓存 (`resourceCache`, `headersResourceCache`)
   - Clash 预处理器验证
   - AGE 解密（如果配置了密钥）
   - 流量头提取
   - 订阅有效性检查 (`validCheck`)
   - 缓存阈值控制
   - 任务去重（`tasks` Map）
   - 代理请求（Stash/Loon/QX/ShadowRocket/Surge 不同的代理协议头）
   - `downloadFile` 辅助函数

2. **流量系统 (`flow.js`)**
   - `flowUrl` 从响应体获取流量信息
   - `flowHeaders` 自定义参数
   - `customProxy` 支持
   - `insecure` 支持
   - 平台特定代理请求头
   - 多 URL（换行分割）
   - 错误回退（HEAD 失败转 GET，GET 失败转响应体解析）
   - `headersResourceCache`
   - `flowTransfer` 流量单位转换
   - `validCheck` 过期/流量超限验证
   - `getRmainingDays` 重置日计算（支持 `startDate` + `cycleDays` 和月重置日）
   - `normalizeFlowHeader` 规范化/去重/URL 解码和数值校验

3. **下载端点 (`download.js`)**
   - query 参数覆盖：`url`, `ua`, `content`, `mergeSources`, `ignoreFailedRemoteSub`, `produceType`, `includeUnsupportedProxy`, `resultFormat`, `proxy`, `noCache`, `_fakeNode`, `fakeSub`, `prettyYaml`, `$options`
   - `mihomoExternal`/`mihomoMerge`/`mihomoMergeName`/`mihomoLocalPort`
   - User-Agent 分析推断平台 (`getPlatformFromHeaders`)
   - 假节点（`fakeNode`/`fakeSub`）回退策略
   - 响应变换器 (`applyResponseTransformers`)
   - AGE 输出加密 (`applyAgeOutputEncryption`)
   - 分享路由限制

### P1 — 安全与同步（影响用户体验，但不会导致应用崩溃）
4. **AGE 加密 (`age.js`)**
   - 集成 `filippo.io/age` 库
   - X25519 和 MLKEM768-X25519 密钥类型
   - WebCrypto X25519 fallback 机制
   - 公钥/私钥验证
   - `encryptArmor`/`decryptArmorIfPresent`
   - 密钥对生成
   - 密钥掩码 (`maskAgeSecretInUrl`)
   - 配置标准化 (`normalizeAgeSecretKeyConfig`, `normalizeAgePublicKeyConfig`)

5. **Gist 同步 (`gist.js`)**
   - 完整的 Gist 类
   - GitHub Gist 和 GitLab Snippet 的 locate/upload/download
   - 代理支持
   - `emptyFileFallback` 机制
   - 文件 diff 逻辑（create/update/delete）
   - `describeGistApiErrorResponse`
   - `getGithubGistBaseURL`
   - `hasGistSyncCredentials`
   - 环境变量触发（`SUB_STORE_DATA_URL`）
   - 定时备份（`SUB_STORE_BACKEND_DOWNLOAD_CRON`/`UPLOAD_CRON`）

6. **Sync Artifacts 完整实现**
   - `syncToGist` 批量同步
   - AGE 加密输出
   - `skipCronArtifacts` 逻辑

### P2 — 处理器与解析器深度（影响高级功能）
7. **Script Processor 完整上下文**
   - 提供 `$arguments`, `$options`, `$substore`, `lodash`, `ProxyUtils`, `yaml`, `Buffer`, `b64d`/`b64e`, `DOMAIN_RESOLVERS`, `scriptResourceCache`, `flowUtils`, `produceArtifact`, `require`
   - Script Operator 回写所有字段，不只 `type`/`name`/`server`

8. **Resolve Domain Operator 完整实现**
   - 6 种 DNS 提供商（Custom / Google / IP-API / Cloudflare / Ali / Tencent）
   - IPv4 / IPv6 / IP4P 类型
   - 结果缓存
   - EDNS、超时
   - 自定义 DNS URL（多路并发）
   - TLS 跳过验证
   - 解析后过滤（`removeFailed`/`IPOnly`/`IPv4Only`/`IPv6Only`）
   - 并发数可配置

9. **各 Producer 字段映射深度补齐**
   - Surge：http, h2-connect, direct, socks5, snell, ssh, trusttunnel, anytls
   - Loon：ssr, http, socks5, wireguard
   - QX：ssr, wireguard, socks5, http, anytls
   - V2Ray：支持所有协议（当前仅 vmess）
   - URI：ssr, hysteria, tuic, anytls, wireguard, socks5, http
   - sing-box：ssh, ssr, snell, naive, hysteria, anytls, tailscale
   - SurgeMac：mihomoExternal 回退机制
   - Clash：ssr, vless, socks5, http, snell, wireguard
   - ClashMeta：socks5, http, snell, wireguard

10. **Parser 字段深度补齐**
    - VLESS xhttp 传输（`mode`, `extra`, `downloadSettings`, `xmux`）
    - Shadowsocks 插件生态（`obfs-local`, `v2ray-plugin`, `shadow-tls`, `gost-plugin`）
    - Shadowrocket 专用变体（base64 编码 VLESS URI 等）
    - Hysteria2 端口跳跃（`server:port1,port2-port3` 多端口语法）
    - ECH 字段解析（Clash YAML 和 VLESS URI）
    - Clash 字段标准化（`servername`→`sni`, `benchmark-url`→`test-url`）
    - Packet Encoding（`none`/`packet`/`xudp`）
    - Base64 预处理器指示器（`c3NkOi8v`, `c2hhZG93`, `dmxlc3M=`）

11. **Processor 补齐**
    - Response Transformer（缺失）
    - Add Proxies From Subscription Operator（缺失）
    - Useless Filter：cipher/password 非 ASCII 检测、transport Host 非 ASCII 检测、关键词过滤
    - Quick Setting Operator：vmess aead, reuse, ecn, block-quic, ip-version, 嵌入式 useless 过滤
    - Sort Operator：random 排序
    - Regex Sort Operator：`order` 参数（`original`/`desc`/`asc`）
    - Regex Rename Operator：多个 `{expr, now}` 对象依次替换
    - Handle Duplicate Operator：两种 `action`（`delete`/`rename`），`template`（数字映射）、`link` 连接符、`position`
    - Conditional Filter：`attr` 可访问代理任意属性
    - Flag Operator：完整地理数据库推断国旗、`tw` 参数支持
    - Script Operator：完整上下文支持

### P3 — 辅助功能与优化
12. **GeoIP/Flag 完整支持**
    - 丰富的国家/地区旗帜映射（`Flags` 和 `ISOFlags` 双向）
    - `getFlag` 根据名称智能匹配关键词/ISO 代码/emoji
    - `removeFlag` 完整逻辑
    - `getISO`
    - MMDB 类（`@maxmind/geoip2-node`）
    - `geoip`, `ipaso`, `ipasn`

13. **脚本资源缓存**
    - `ResourceCache` 类，持久化缓存（`SCRIPT_RESOURCE_CACHE_KEY`）
    - TTL 规范化
    - 自动清理过期项
    - `_persist` 写入持久化存储
    - `revokeAll`

14. **Token 认证完整实现**
    - pathname 前缀匹配
    - 创建校验（payload.type/name 存在性、依赖数据存在性、唯一性）
    - AGE 公钥集成
    - `duration`/`datetime`/`count` 三种过期模式
    - `expiresValue`/`expiresUnit` 输入
    - 归档模式删除

15. **Archive 完整恢复**
    - 恢复后实际写回对应 store
    - `share` 归档类型支持
    - `sortArchives` 完整实现

16. **前后端 Merge 模式 + SPA 代理**
    - 静态文件服务
    - 路径 rewrite
    - 代理转发
    - SPA 路由回退

17. **数据迁移完整实现**
    - 多版本逐步升级
    - 字段重命名、结构调整
    - schemaVersion 详细版本追踪

18. **YAML 支持完整实现**
    - `!str` 标签 workaround（retry 机制）
    - 自定义选项映射层（`merge`, `lineWidth`, `noArrayIndent`, `noRefs`, `quotingType`, `forceQuotes`）

## 实施策略
1. 按 P0 → P1 → P2 → P3 顺序实施
2. 每个阶段完成后编译验证
3. 每个阶段添加对应单元测试（如有）
4. 保持 Go 代码风格，不引入过度复杂化
5. 必要时添加详细中文注释

## 验证标准
- 所有功能补齐后，Go 后端与 Node.js 后端在 API 行为、数据流、输出格式上完全一致
- 通过功能等价测试（手动 + 自动化）
