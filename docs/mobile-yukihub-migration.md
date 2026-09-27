# 与 Android 版 YukiHub 的数据迁移设计

本文定义桌面版与 Android 版之间的数据契约。阶段 2 的实现以本文为准。

## 一、两侧的数据形态

### Android 版 YukiHub

- 本地库：SQLite `yukihub.db`（`DB_VERSION = 22`）
- 交换格式：**schema 5 快照 JSON**（gzip 压缩可选），可导出为文件或经 WebDAV / 自建服务器同步
- 关键表：`games`、`play_sessions`、`metadata_cache`、`settings`

### 桌面版本项目

- 本地库：DuckDB `yukihub.db`
- 关键表：`games`、`play_sessions`、`game_metadata_sources`、`game_tags`、`categories`、
  `game_categories`、`game_progress`、`game_reviews`、`game_filter_presets`

两侧的 `games` 语义不同：Android 侧是"视觉小说条目"（含三语标题、引擎类型、SAF 路径），
桌面版本项目来自上游，是"PC 游戏库条目"（含启动方式、LE/Steam 配置、分类标签）。

**统一后的模型必须同时容纳两者**，而不是二选一。

## 二、快照格式（需冻结的契约）

```jsonc
{
  "app": "YukiHub",
  "schema": 5,
  "lightweight": true,
  "created_at": 0,
  "note": "Only text metadata is synced. ...",
  "profile":  { "name": "...", "signature": "...", "avatar_uri": "https://..." },
  "settings": { "metadata_source": "vndb", "sort_mode": "recent", "ui_scale": 1.0, ... },
  "games": [ /* 见下表 */ ],
  "play_sessions": [ /* 见下表 */ ],
  "metadata_cache": [ /* 见下表 */ ]
}
```

### games 元素

`local_id`、`title`、`original_title`、`engine`、`root_uri`、`cover_uri`、
`cover_persist_uri`、`cover_source_type`、`emulator_package`、`launch_target`、
`winlator_launch_mode`、`description`、`tags`、`gamehub_local_game_id`、
`gamehub_launch_mode`、`play_status`、`total_play_time`、`last_played_at`、
`playtime_reset_at`、`created_at`、`updated_at`、`hidden`、`favorite`、`nsfw`

### play_sessions 元素

`session_uuid`、`game_local_id`、`game_root_uri`、`gamehub_local_game_id`、`game_title`、
`game_engine`、`game_emulator_package`、`start_time`、`end_time`、`duration`、
`launch_type`、`device_id`、`created_at`、`updated_at`

### metadata_cache 元素

`game_local_id`、`game_root_uri`、`game_title`、`source`、`source_id`、`json`、`updated_at`

### 快照中刻意不包含的内容

以下内容被 Android 版明确排除在同步之外，桌面版也必须遵守，**不得擅自加入**：

- 扫描目录配置（含用户本机路径，隐私与跨设备无效）
- 自定义背景图 / 背景视频（本地文件引用）
- `trailer_path`、`logo_path`、`bg_path`（本地文件路径）
- 音乐厅数据（`music_albums` / `music_tracks`，依赖设备本地 SAF 授权）
- 游戏本体、存档文件、二进制封面图

游玩记录条数上限：同步与本地备份均为 **30 条**（按时间取尾部）。

## 三、字段映射

### 游戏条目

| Android 字段 | 桌面端字段 | 说明 |
| --- | --- | --- |
| `local_id` | `legacy_local_id`（新增列） | 保留原 ID，用于回写与去重，不替代主键 |
| `title` | `name` | 主标题 |
| `original_title` | `aliases` 中的原名项 | 桌面端用别名数组承载 |
| `description` | `summary` | |
| `tags`（逗号/分隔文本） | `game_tags` 表 | 需要拆分并去重 |
| `play_status` | `status` | 见下方状态映射 |
| `nsfw` | `is_nsfw` | |
| `total_play_time`（**毫秒**） | 由 `play_sessions` 聚合 | 不直接写入；见时长规则 |
| `last_played_at` | 由会话聚合 | |
| `playtime_reset_at` | `playtime_reset_at`（新增列） | 必须保留，否则清零历史会复活 |
| `created_at` / `updated_at` | `created_at` / `updated_at` | |
| `hidden` | `hidden`（新增列） | |
| `favorite` | 归类到"收藏"系统分类 | 复用 `categories` 的 `is_system` |
| `root_uri` | `path` / `game_directory` | 见路径模型 |
| `engine` | 无需映射 | Android 专用（引擎类型），桌面端不存储 |
| `emulator_package`、`launch_target`、`winlator_launch_mode`、`gamehub_*` | 无需映射 | Android 专用启动方式 |
| `cover_uri` / `cover_persist_uri` | `cover_url` / 本地封面文件 | 见封面规则 |
| `cover_source_type` | 并入元数据来源 | |

### 游玩状态映射

Android 版只有 5 态，桌面版（上游）有 6 态：

| Android | 桌面端 | 备注 |
| --- | --- | --- |
| `unplayed` | `not_started` | |
| `playing` | `playing` | |
| `completed` | `completed` | |
| `onhold` | `on_hold` | 兼容 `on_hold` / `shelved` / `paused` 等历史写法 |
| `dropped` | `dropped` | |
| — | `want_to_play` | Android 版没有"想玩"；导出到 Android 时降级为 `unplayed` |

映射必须容忍下划线、过去式与大小写差异（历史上出现过多种写法）。

### 时间与单位

- Android 的 `play_sessions.duration` 为**秒**；Android 的 `games.total_play_time` 为**毫秒**。
  两者单位不同，是历史上最容易出错的地方，必须有专门测试。
- 时间戳为 Unix 毫秒。
- 桌面端数据库使用 `TIMESTAMPTZ`，写入前需按用户配置的时区归一化。
- 桌面端导出到 Android 时，时间必须写成 Android 可解析的格式。

## 四、身份与去重（最关键的部分）

### 问题

Android 版用自增整数 `local_id`，桌面端用 UUID。两侧各自新增条目后 ID 空间会碰撞，
这是"一次导入能用、持续同步后出现重复条目"的根因。

### 方案

为每个游戏维护一个**稳定身份键**，优先级如下：

1. **规范化后的游戏路径**（Android 侧 `root_uri` ↔ 桌面端 `path`）
2. Android 侧的 `gamehub_local_game_id`（无本地目录的条目）
3. **标题精确匹配**（仅在路径为空且唯一匹配时使用，不唯一则视为新条目）

同时在 `games` 表新增 `legacy_local_id` 与 `source_device_id` 两列：
前者保留 Android 侧的整数 ID，后者记录条目来源设备，避免多设备互相覆盖。

### 路径规范化

Android 侧的 `root_uri` 可能是 `content://` 形式（SAF 树 URI），需要：

1. 去掉 `file://` 前缀
2. `content://` 取出 `DocumentsContract` 的 docId
3. 折叠重复的 `/`、去掉结尾 `/`、转为小写

桌面端路径直接使用 Windows 绝对路径。**两侧规范化后的字符串不等价**，
因此跨端匹配以 `legacy_local_id` 与标题为主，路径匹配仅用于同平台内部。
这是本设计中最需要谨慎处理的一点。

## 五、合并语义

必须与 Android 版保持一致，否则两端会反复互相覆盖：

| 项 | 规则 |
| --- | --- |
| 总游玩时长 | **取最大值**，不做覆盖。防止较短时长覆盖较长时长 |
| 游戏标题 | 以"有云端元数据的一侧"为准，避免被本地扫描生成的目录名覆盖 |
| 游玩记录 | 以 `session_uuid` 为幂等键，重复导入不产生重复会话 |
| 时长重算 | 只增不减（重置清零除外） |
| 清零语义 | `playtime_reset_at` 之前的历史会话不计入统计，但记录本身保留 |
| 冲突方向 | 由同一套算法裁决，不允许两端各有一份实现 |

### 聚合补偿

当 Android 侧只有 `games.total_play_time` 而没有对应明细会话时（历史数据），
需要生成一条"聚合会话"，否则总时长会在同步过程中丢失。
现有导入器已经实现了这一点（用确定性 UUID 生成），导出方向需要对称实现。

## 六、封面

- Android 侧的封面可能是 `content://` 本地 URI（跨设备无效）或网络 URL。
- 导出到桌面端时：网络 URL 直接沿用；本地 URI 不迁移，改为按元数据来源重新下载。
- 桌面端导出到 Android 时同理：只写网络 URL。
- 本地封面文件名约定需要两端一致，避免重复下载。

## 七、待决策问题

1. **谁是权威源？** 桌面端与 Android 版同时在线时的冲突裁决者需要一个明确答案。
   建议：不做实时双向同步，而是"以任一端为源、显式导入导出"，避免两端同时写。
2. 桌面端是否复用 Android 版的自建同步服务器（`/api/sync/*`）？
   若复用，需要服务端确认新客户端可被接受（版本、UA、协议）。
3. 桌面端是否保留自有的云同步（上游 `cloudsync` 的墓碑与脏表机制）？
   与 Android 版的哈希比对机制是两套算法，**必须择一**，不能并存。
4. 元数据缓存的 JSON 结构是否沿用 Android 版的 `VnMetadata` 字段？
   建议沿用，便于两端共享缓存。

## 八、测试矩阵

阶段 2 的验收依赖以下样例集（需要真实数据，不含隐私信息）：

| 样例 | 覆盖点 |
| --- | --- |
| 空库 | 首次导入 / 首次导出 |
| 单条游戏 + 多条会话 | 基础映射与时长计算 |
| 含清零记录 | `playtime_reset_at` 语义 |
| 只有总时长、无明细 | 聚合补偿会话 |
| 重复导入同一份快照 | 幂等性，不得产生重复条目与会话 |
| 双端各自新增条目 | ID 空间不碰撞 |
| 时长单位混淆 | 秒/毫秒换算 |
| 跨时区时间戳 | 时区归一化 |
| 无本地目录条目（`root_uri` 为空） | 按标题匹配的降级路径 |
| 路径不可达 | 导入后可正常显示，启动时给出明确提示 |
| 大量条目（万级） | 性能与不丢数据 |
