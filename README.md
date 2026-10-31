# gWvljBt

一个用 **Wails v3 + Vue 3 + TypeScript** 写的本地 Galgame / 视觉小说收藏管理器。
游戏资料、补丁记录、海报全部落在本机 SQLite 和用户目录里。界面走「私人藏书目录册」的路子：墨色或纸色底、发丝分隔线、衬线标题配等宽数字，封面是主角，配色与壁纸都能自己换。

---

## 功能

### 游戏库
- 新增 / 编辑 / 删除作品，字段包括标题（原名 / 别名）、会社、发行商、发售日、简介、标签、分类、评分、游玩状态、通关时长、安装目录、启动程序、存档目录、封面与横幅。
- 从 **VNDB** 或 **Bangumi** 搜索并一键抓取元数据（标题、封面、发售日、会社、简介、评分、标签都会自动填，已有内容不会被空值覆盖）。
- 卡片网格展示，支持按状态（未开始 / 在玩 / 已通关 / 搁置 / 弃坑）、收藏、会社、标签、关键词筛选与排序。
- 一键启动游戏（自动识别目录里的主程序）并记录最后游玩时间，一键打开安装目录 / 存档目录。
- 游玩状态、评分（0–10）、累计时长（快速 +30 / +60 分钟）都能在详情页改。

### 游戏扫描器
选一个总目录（比如整块装游戏的盘），自动把下面像 galgame 的安装目录找出来，勾选后批量入库。
- 判定看**文件指纹**而不是目录名：引擎特征文件（`.xp3` KiriKiri、`.ypf` YU-RIS、`.pfs` BGI、`.npa` Nitroplus、`.int` CatSystem2、`.rpa` Ren'Py、`.nsa`/`nscript.dat` NScripter、`.cpz` CMVS、`data.win` GameMaker、`unityplayer.dll` Unity 等）、主程序（自动排除 `unins`/`setup`/`update` 这类安装器）、以及 `voice`/`bgm`/`scene`/`data` 这类素材目录。
- 找到候选后**不再往它里面下钻**，避免把游戏内部目录当成另一个游戏；标题会从目录名清洗（去掉 `[会社]`、`汉化版`、`v1.02` 这类噪音）。
- 列表里给出引擎、文件数、体积、置信度，已经在库里的目录会标出来并自动取消勾选。
- **自动补元数据**：扫完会按标题逐个去 VNDB / Bangumi 搜一遍，标题相似度 ≥ 0.6 才认，然后拉详情补上封面、会社、发行日期、简介、标签、评分；抓到的行会显示来源和匹配度。
- 抓到的标题直接变成列表里可编辑的输入框，用户可以随手改；没抓到（或抓错了）就回退成目录名清洗出来的标题，其余字段空着，导入后在详情页接着填。
- 导入时复用建库逻辑：目录已存在则跳过，主程序找不到会再试一次，标题为空则回退目录名；元数据里的封面会顺手抓进 `media/posters/`，断网也不会丢。

### 补丁管理
记录「哪个游戏装了哪些补丁」，需要的时候能一键清掉，但**本工具不提供任何补丁下载**。
- 补丁条目：名称、类型（汉化 / 解锁 / 修复 / MOD / 破解 / 其他）、版本、来源、来源链接、备注、是否已安装。
- **快照对比**：先给游戏目录建基线快照，装完补丁再点一次「检测变化」，新增和改动的文件会自动列出来，勾选后即为该补丁登记的文件清单。
- **手动浏览**：目录树里直接勾文件，已经在别的补丁里登记过的会标出来。
- 卸载时可以选「只标记为未安装」或「删除登记的文件」；删除前会再逐个确认存在性，并列出删不掉的项。
- 校验：随时检查登记的文件在磁盘上还在不在。
- 页面里放了一个 [2DFan](https://www.2dfan.com/) 的跳转链接，补丁自己去找。

### 统计
收录总数、已通关、收藏、总时长、平均分，外加发行年份柱状图、状态分布、TOP 会社、补丁类型环形图、高频标签、库健康度（缺封面 / 目录不存在）和最近更新。

### 外观
- 6 套完整纸墨样式（墨 / 夜 / 弁柄 / 和纸 / 樱 / 抹茶），每套自带明暗与整套配色，切换即换整本的感觉。
- 在预设之上还能改：强调色（8 个颜色或自己取色，也可以「跟随样式」）、壁纸（选本地图片，可调模糊与压色浓度，不设壁纸时用纸面颗粒底）、圆角、界面密度、海报比例、动效开关。
- 侧栏底部是当前时间与日期，不再有单独的明暗开关。

---

## 数据放在哪

| 内容 | 位置 |
| --- | --- |
| 配置 | `%USERPROFILE%\gWvljBt\config.json` |
| 数据库（游戏 / 补丁 / 文件清单 / 目录快照） | `%USERPROFILE%\gWvljBt\app.db` |
| 海报、横幅（导入的图片会复制进来） | `%USERPROFILE%\gWvljBt\media\posters|banners\` |
| 壁纸（设置里选的图片会复制进来） | `%USERPROFILE%\gWvljBt\media\wallpapers\` |
| WebView2 的缓存 / 用户数据 | `%USERPROFILE%\gWvljBt\webview\` |

想整套搬走（U 盘、另一台机器）时设 `GWVLJBT_HOME` 环境变量指向新目录即可，配置、数据库、图片、WebView2 缓存都会跟着走。

除了配置走 JSON，其余数据全部用 **GORM + SQLite**（WAL 模式）存储。数据库里只有记录，**删除游戏或补丁都不会动磁盘上的游戏文件**——除非你在卸载补丁时明确选择「删除文件」。

驱动用的是纯 Go 的 `github.com/glebarez/sqlite`（底层 modernc.org/sqlite），**不需要 cgo、不需要 C 编译器**，`CGO_ENABLED=0` 也能正常构建运行。

数据库 schema 由 GORM 自动迁移，首次运行会自动建表。

---

## 开发

```bash
# 开发模式（前端热更新 + Go 热重载）
wails3 dev

# 只跑前端（浏览器里调试用，媒体资源由后端 dev server 提供）
cd frontend && pnpm dev

# 生产构建
wails3 build
```

开发模式下窗口直接加载 Vite dev server，所以 `/media/` 由 `frontend/vite.config.ts` 代理到后端 `127.0.0.1:39217`（`main.go` 里的 `startDevMediaServer`，只在 `FRONTEND_DEVSERVER_URL` 存在时启动）。生产模式下由 Wails 的 asset middleware 直接提供 `/media/`。

改了 Go 的服务方法签名之后要重新生成前端绑定：

```bash
wails3 generate bindings -clean=true -ts -i
```

### 目录结构

```
main.go              启动、窗口、/media 静态资源、事件注册
config/              配置结构与读写（config.json）
data/                GORM 模型、SQLite 打库、媒体落盘、目录扫描与差异、galgame 目录识别
configService.go     配置服务（读写、打开数据目录、选壁纸）
gameService.go       游戏 CRUD、筛选、启动、统计
patchService.go      补丁 CRUD、快照对比、卸载
metaService.go       VNDB / Bangumi 元数据
scanService.go       游戏扫描器（选目录、扫描、批量入库）
frontend/src/
  views/             游戏库、详情、补丁、统计、设置
  components/        卡片、编辑器、海报、扫描器
  lib/               api 封装、主题、格式化、类型
  state/             配置 / 游戏库 / UI 的响应式状态
  styles/base.css    设计系统变量与通用类
```

### 元数据接口

- **VNDB**：`POST https://api.vndb.org/kana/vn`，可填 API Token（设置页），不填也能用。
- **Bangumi**：`POST https://api.bgm.tv/v0/search/subjects`（`type: 4` 即游戏），会带自定义 UA，可填 Access Token。

两者都做了简单的限速，接口不可用时界面会提示错误而不是卡住。
