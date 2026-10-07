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

其中 `json` 是 Android 侧 `VnMetadata` 的 JSON blob，两端**原样搬运**，不做字段改写。
桌面端把它存在 `game_metadata_sources.cache_json`（`migration178` 新增），
导入与导出两个方向都直接传递该字符串，以保证「Android → 桌面端 → Android」
不丢失桌面端没有对应列的字段（截图、罗马音标题、封面分级等）。

### 快照中刻意不包含的内容

以下内容被 Android 版明确排除在同步之外，桌面版也必须遵守，**不得擅自加入**：

- 扫描目录配置（含用户本机路径，隐私与跨设备无效）
- 自定义背景图 / 背景视频（本地文件引用）
- `trailer_path`、`logo_path`、`bg_path`（本地文件路径）
- 音乐厅数据（`music_albums` / `music_tracks`，依赖设备本地 SAF 授权）
- 游戏本体、存档文件、二进制封面图

游玩记录条数上限：**Android 是「所有游戏合计取最新 30 条」**
（`SyncManager.buildLocalSnapshot` 对 `exportPlaySessionsJson()` 返回的
**单个扁平数组**整体做 `tail(sessions, 30)`），桌面端是**每个游戏各 30 条**
（`buildYukiHubSessions`），这是有意的差异，理由见 §五。

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
| `favorite` | 归类到"收藏"系统分类 | 复用 `categories` 的 `is_system`（`system:favorites`）；导入方向已于 2026-09-28 补齐 |
| `root_uri` | `path` / `game_directory` | 见路径模型 |
| `engine` | 无需映射 | Android 专用（引擎类型），桌面端不存储 |
| `emulator_package`、`launch_target`、`winlator_launch_mode`、`gamehub_*` | 无需映射 | Android 专用启动方式 |
| `cover_uri` / `cover_persist_uri` | `cover_url` / 本地封面文件 | 见封面规则 |
| `cover_source_type` | 并入元数据来源 | |

### 游玩状态映射

桌面端已与 Android 版统一为同一套 5 态，双向为恒等映射，不再有"想玩"：

| Android | 桌面端 | 备注 |
| --- | --- | --- |
| `unplayed` | `unplayed` | 上游历史值 `not_started` 与 `want_to_play` 已并入 |
| `playing` | `playing` | |
| `completed` | `completed` | |
| `onhold` | `onhold` | 兼容 `on_hold` / `shelved` / `paused` 等历史写法 |
| `dropped` | `dropped` | |

映射必须容忍下划线、过去式与大小写差异（历史上出现过多种写法）。

### 时间与单位

- Android 的 `play_sessions.duration` 与 `games.total_play_time` **均为毫秒**（不是秒）。
  证据（手机版 `data/GameRepository.java`）：
  - `finishPlaySession()`：`rawDuration = max(0, end - start)`，`start/end` 取自
    `System.currentTimeMillis()`；随后 `UPDATE games SET total_play_time = total_play_time + ?`
    累加的正是该 duration。
  - `exportGamesJson()` / `exportPlaySessionsJson()`：两个字段均**原值写入 JSON**，无换算。
  桌面端导入器 `durationSeconds := durationMillis / 1000` 与之一致，**不要改成不换算**。
- 时间戳为 Unix 毫秒。
- 桌面端数据库使用 `TIMESTAMPTZ`，写入前需按用户配置的时区归一化。
- 桌面端导出到 Android 时，时间必须写成 Android 可解析的格式（Unix 毫秒整数）。

> 澄清：本文件此前曾写成"会话 duration 为秒、total_play_time 为毫秒、两者单位不同"，
> 与手机版源码不符，已于 2026-09-28 按上述证据更正。

### Android 侧已完成的过滤

`exportPlaySessionsJson()` 的 SQL 自带条件
`COALESCE(ps.end_time, ps.start_time, 0) >= IFNULL(g.playtime_reset_at, 0)`，
即**导出的会话已排除清零前的记录**。桌面端导入时只需正确持久化 `playtime_reset_at`，
不需要二次过滤；但桌面端自身导出到 Android 时必须施加同样的过滤，否则清零历史会在
Android 侧复活。

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

### 现状核对（2026-09-28，导入侧合并已落地后更新）

上表是**目标语义**。逐条核对桌面端实现与目标语义的差距：

**① 「取最大值」——导入方向已落地（`merge` / `merge_sessions` 动作）**

桌面端没有游戏级 `total_play_time` 列，总时长由会话聚合而来，"取最大值"
由两个机制组合实现：

1. **会话并集去重**：合并导入把快照会话写入已有游戏，按
   `(game_id, start_time, end_time)` 去重（`addImportedItemSessions` 的
   `NOT EXISTS`），两端相同的物理会话只留一份；
2. **聚合补偿**：快照 `total_play_time` 超过已录会话总和的差额，
   补成一条确定性 UUID 的聚合会话（`convertYukiHubSessions`）。

最终桌面总时长 = max(桌面已录时长, 快照 total_play_time)。
测试：`TestYukiHubImportMergeSessionsTakesMaxPlaytime`。

注意：`skip`（默认）动作下命中条目仍整条跳过，记录不写入——这是用户选择，
不算语义缺口。导入器此前丢弃 `samePathAction` 参数（无合并路径）已修复。

**①.1 云同步 / WebDAV 自持同步用 `sync_merge`（2026-10-06 补）**

云同步此前走 `merge_sessions`：只并会话，**完全不碰已有游戏记录**。后果是
手机端改过的 `play_status` / `hidden` / `nsfw` / 标题同步回桌面端时被整段丢掉
（反向的 PC → 手机方向没问题，因为手机端 `importGamesJson` 会更新已有条目）。

新增 `ImportActionSyncMerge`（`SamePathActionSyncMerge` = `"sync_merge"`）专供
同步链路，语义对齐手机版 `importGamesJson`：

- 并集去重地并入对端会话（与 `merge_sessions` 相同，含聚合补偿）；
- 更新已有游戏字段，**两个守卫同时成立才写**：
  1. **对端 `updated_at` 不早于本地**（旧快照不得覆盖新编辑）；
  2. **文本字段非空**——对端可能只带 `games` 段、没有任何 `metadata_cache`，
     那时 `summary` / `company` / `release_date` 全是空串，照抄会把桌面端刮削
     好的资料冲成空白（这是它与 `merge` / `update_existing` 的唯一区别，
     后者面向 PotatoVN / Playnite 这类自带完整元数据的格式）。
- 来源（`source_type` / `source_id` / `cached_at`）只在**对端来源不是 `local`**
  时改写，避免「对端没元数据」被翻译成「把桌面端来源抹成 local」；
- `playtime_reset_at` 只允许**更晚且非空**时前进，退不回去；
- `is_nsfw` 仍按权威来源名单采信；桌面端本机字段（`path` 等）不受影响。

测试：`TestYukiHubSyncMergeAppliesNewerFieldsOnly`。
`.ykbak` 手动导入仍是用户在下拉里自选的 `skip` / `merge_sessions` / `merge`，
不受影响。

**② 标题匹配与预览/导入不一致——已修复**

Android 侧 `root_uri` 为空时走 `findByTitleForEmptyRoot` **纯标题匹配**，
不管本地游戏有没有路径。桌面端导入器此前有两处偏差：

- `findExistingGameConflict` 的 NameAndPath 分支要求路径相等，
  桌面端**有本机路径**的同名游戏不命中 → 快照被导入成**重复条目**；
- `PreviewYukiHubImport` 早已做纯标题匹配，导入却新建 —— 预览显示
  "已存在"、实际却重复导入，两处行为矛盾。

修复：YukiHub 导入器在通用判定未命中时追加纯标题匹配
（`existingNames` 命中即视为同一条目），与 Android 侧语义和预览行为对齐。
副作用：skip 动作下，桌面端手动添加的同名游戏（带路径）也会被跳过——
这是标题匹配语义的固有代价，Android 侧同样如此。

**③ 会话幂等的实际保证是"条目级"而非"键级"**

契约要求以 `session_uuid` 为幂等键。实际实现是两层近似：
条目被跳过（skip 或 merge 之外的路径）时会话根本不会写入；
条目合并时按 `(game_id, start_time, end_time)` 去重。数据库层没有
`session_uuid` 唯一约束，`play_sessions.id`（= session_uuid）是主键但
去重判断不查它。正常流程下等价；极端情况（同 UUID 不同时间戳的手工数据）
会插入重复。记录在案，暂不修——需要唯一约束级别的保证时再上。

**④ 游玩状态（play_status）在 merge 动作下不更新**

`updateImportedItemMetadata` 的 UPDATE 不含 `status` 列，合并元数据时
桌面端游玩状态保持不变。这是通用导入路径的既有行为（PotatoVN 相同）。
Android 为权威源的状态合并是否要覆盖，待决策。

**⑤ 游玩状态已在两端对齐（原"想玩"往返丢失问题已消除）**

桌面端已按本文档契约收敛为与 Android 完全一致的 5 态：`migration179` 把存量
`not_started` / `want_to_play` 并入 `unplayed`，`on_hold` 改写为 `onhold`，
筛选预设里保存的状态条件同步转换。双向映射因此成为恒等映射，不再存在只在
单端出现的状态；历史快照里残留的旧值在导入时仍按上表的容错规则归一。

**⑥ 导入时不重复下载封面**

`convertYukiHubGame` 把 `cover_source_url` 也设为网络封面地址，
命中的条目会被"整条跳过"——即封面已在本地时不会重新下载，符合契约。

**⑦ `favorite` 导入方向缺失——已修复（2026-09-28）**

导出方向早已把 `system:favorites` 分类读成 `favorite`（`loadFavorites`），
但导入方向完全丢弃了快照的 `favorite`：桌面端没有游戏级收藏列，
收藏关系存在 `game_categories`，而导入器的落库路径（staging + INSERT）根本不碰分类表。
结果是「桌面端导出 → 手机版 → 回导」会静默丢失收藏，两端行为不对称。

修复：`ImportItem` / `CommitItem` 新增 `Favorite` 字段，`CommitItems` 增加
`addImportedItemFavorites` 步骤，把标记收藏的条目写入 `game_categories`
（`system:favorites`，`ON CONFLICT DO NOTHING`）。
语义上**只加不删**：快照里的 `favorite=false` 无法区分"明确取消收藏"与"本轮未同步"，
删除既有收藏属破坏性操作，按合并语义「只增不减」处理。
测试：`TestYukiHubImportAppliesFavoriteToSystemCategory`。

**⑧ 元数据缓存（`metadata_cache`）双向缺失——已修复（2026-09-29）**

导入方向此前只把缓存 JSON 解析成身份与少量字段，**丢弃原始 blob**；导出方向
完全不产出 `metadata_cache`。结果是「Android → 桌面端 → Android」会静默丢失
桌面端没有对应列的字段（截图、罗马音标题、封面分级）。

修复：`game_metadata_sources` 新增 `cache_json` 列（`migration178`），导入时原样写入
`entry.json`、导出时原样读回；刮削路径也把该来源的负载按同一结构写入，
供离线展示与导出复用。空负载不覆盖已有缓存（缺失 `metadata_cache` 元素只是身份信息，
用它清空对端缓存属破坏性写入）。
测试：`TestYukiHubImporterPreviewAndImport`、`TestApplyRemoteMetadataCachesSourcePayload`、
`TestYukiHubMetadataCacheRoundTrip`。

### 聚合补偿

当 Android 侧只有 `games.total_play_time` 而没有对应明细会话时（历史数据），
需要生成一条"聚合会话"，否则总时长会在同步过程中丢失。
现有导入器已经实现了这一点（用确定性 UUID 生成）。

### 导出方向（桌面端 → Android）

落点：`internal/service/exporter/yukihub.go`（`Build()` 生成快照、`Export()` 写 gzip 文件）。

- 单位：桌面端 `play_sessions.duration` 是秒，导出时 × 1000 写入 `duration`；
  `games.total_play_time` 由清零之后的会话求和后同样换算为毫秒。
- 清零过滤：与 Android 侧 `exportPlaySessionsJson()` 对称，只导出
  `COALESCE(end_time, start_time) >= playtime_reset_at` 的会话，避免清零历史复活。
- 条数上限（**与 Android 刻意不同**）：桌面端**每个游戏**导出最新 30 条会话，
  而 Android 的 `tail(sessions, 30)` 是**所有游戏合计** 30 条（见 §二）。
  桌面端放宽是因为它的总时长由会话求和得出：少导出会话会让回导后的总时长缩水。
  Android 导入时不校验条数，收到多少收多少，它自己下一次备份时再裁到 30 条。
- **`root_uri` 恒为空串**，不写 Windows 绝对路径。对端 Android 会走
  `findByTitleForEmptyRoot` 按标题匹配；这与导入方向对称（桌面端导入 Android 备份时
  同样不把对端的 `content://` 路径写进 `path` / `game_directory`）。
  *遗留*：Android 源生条目（本地 `root_uri` 非空）经桌面端回导时仍可能在对端产生重复，
  彻底解决需新增 `legacy_root_uri` 列保存对端原始路径并在导出时回填。
- 无标题条目（`name` 去掉空白后为空）跳过导出：对端会把空标题落成"未命名游戏"，
  只会制造无法匹配的占位记录。
- `local_id` 由 `games.legacy_local_id` 还原（保留的 Android 整数 ID），非法或缺失时为 0。
- `original_title` 取第一个非空别名；`tags` 用英文逗号拼接。
- `play_status` 已是与 Android 一致的 5 态恒等映射；导入旧快照时仍按状态映射表容错归一历史写法。
- `cover_uri` 只写 `http(s)://` 开头的网络封面，本地封面不迁移。
- `favorite` 由系统收藏分类（`game_categories` 中的 `system:favorites`）导出。
- `metadata_cache` 由 `game_metadata_sources.cache_json` 导出（2026-09-29 补齐）：
  负载即 Android 侧 `VnMetadata` JSON，逐字节原样搬运，不在 `games` 元素里重复表达；
  `game_local_id` 由 `games.legacy_local_id` 还原，缺失或非法（≤0）的条目跳过，
  避免产出无法被对端关联的孤儿缓存。桌面端没有对应列的字段（截图、罗马音标题、
  封面分级）只存在于该 JSON 中。
  测试：`TestYukiHubMetadataCacheRoundTrip`。
- 游玩记录另写入 `launch_type = "external"`、`device_id = "desktop"`，
  `game_root_uri` 与游戏条目保持一致（空）。

### 实机核对结论（2026-09-30，用真实手机版备份）

拿一份手机版本地备份（`yukihub_backup_1790751445692.ykbak`，32 游戏 / 30 会话 /
42 条元数据缓存，gzip + schema 5）跑端到端导入，结论与修掉的问题：

**导入方向可用**（32 条 0 失败、0 跳过），核对通过的项：

| 项 | 手机版备份 | 桌面端导入后 |
| --- | --- | --- |
| 游戏条目 | 32 | 32 |
| 状态分布 | completed 21 / unplayed 9 / playing 1 / dropped 1 | 完全一致 |
| 收藏 | 1 | 1（写入 `system:favorites` 分类） |
| NSFW / 隐藏 / 清零 | 2 / 0 / 9 | 2 / 0 / 9 |
| 元数据来源 | — | 38 条（vndb 30 / bangumi 3 / ymgal 3 / hikarinagi 2） |
| 标签 | — | 199 |
| 总时长 | 天使☆嚣嚣 26347745 ms | 26347 秒（÷1000，含清零过滤） |
| 游玩记录 | 30 条 | 60 行 = 30 真实 + 30 聚合补偿 |

修复的三个缺陷（都属于「只映射、没落库」）：

1. **状态全丢**：落库的 `INSERT INTO games` 列清单里没有 `status`，所有条目
   拿到列默认值「未玩」。补齐后 4 种状态逐条对上。
2. **别名全丢**：同一条路径也没有 `aliases`，原文名/罗马字标题丢失，
   影响展示与按标题匹配。补齐后 32 条里 30 条带别名（另 2 条备份里本身没有原文名）。
3. **合并路径同样缺**：`updateImportedItemMetadata` 的暂存表与 UPDATE 也没有这两列，
   已按手机版 `importGamesJson` 的规则补上——只有对端 `updated_at` 不早于本地时才覆盖。
   回归测试：`TestYukiHubImportPersistsStatusAndAliases`。

**导出方向此前没有任何入口**：`exporter` 包只被测试引用，界面上无法导出 `.ykbak`。
2026-09-30 接线为 `ImportService.SelectYukiHubExportPath` / `ExportToYukiHub`，
入口在「设置 → 全量数据备份 → YukiHub 手机版迁移」。

同时修掉一个会**抹掉手机端数据**的隐患：手机版 `importGamesJson` 用的是
`optString(key, 本地值)`，**字段存在但为空串会被当作「清空」**。桌面端没有对应概念的
字段（`engine`、`emulator_package`、`launch_target`、`winlator_launch_mode`、
`gamehub_local_game_id`、`gamehub_launch_mode`、`cover_persist_uri`、
`cover_source_type`）原先会导出成 `""` / `0`，现在改为 `omitempty` 直接省略，
对端便会保留自己的值。`cover_uri` 也改为优先取 `cover_source_url`（元数据来源的
原始网络地址），本地缓存文件对端拿不到，两者都不是网络地址时省略该字段。

**仍未对齐的部分（有意保留）**：

1. 会话条数上限：桌面端是每游戏 30 条，Android 是合计 30 条。
   桌面端放宽是因为它的总时长由会话求和得出：少导出会话会让回导后的总时长缩水。
   Android 导入时不校验条数，收到多少收多少，它自己下一次备份时再裁到 30 条。
   代价是两端的 `play_sessions` 数组不会逐字节相同，加上两端 JSON 序列化顺序本就不同，
   同一条云端快照在两端的 SHA-256 必然不同 —— 于是桌面端与手机端交替同步时，
   「已是最新」几乎不会命中，每次都会各传一遍。这是哈希比对式同步的固有性质，
   不是数据错误：导入是纯增量合并，不会丢数据。
2. `settings`：桌面端只写 `metadata_source`。手机端的排序/缩放/扫描等偏好属于
   设备本地偏好，桌面端没有对应概念，不迁移。

> 已对齐（2026-10-05 修订）：快照头字段（`profile` / `lightweight` / `note` /
> `backup_type`）桌面端**已经**按手机版逐字写出；`metadata_cache` 元素已补齐
> 手机版匹配所必需的 `game_root_uri` / `game_title`；`original_title` /
> `description` / `tags` / `end_time` 已改为 `omitempty`，不再用空串把对端字段抹掉。
> （本文档早前记成「桌面端不写快照头字段」，与代码不符，已更正。）

### 空游戏库的同步语义（桌面端增量）

手机版 `syncToServer` 有一条「本地库为空 + 云端有数据 + 从没同步过 → 直接下载」的分支
（`SyncManager.java` 的 `isSnapshotEmpty`）。桌面端在 `SyncAccountNow` 里把这条**放宽**成
「本地库为空 + 云端有数据 → 直接下载」，不再要求「从没同步过」。

原因：本同步机制没有任何删除传播（导入是纯增量、没有墓碑），单条游戏的删除本来就不会
同步出去。如果不放宽，用户「清空游戏库后再点同步」会落进
`localChanged && !remoteChanged` → **上传**，也就是用空库把云端抹掉——一次纯粹的误伤，
而空库恰恰是重装、换设备、手滑清库后最需要云端的时候。要显式清空云端应当另做覆盖式操作。

### WebDAV 自持同步（2026-10-06 桌面端补齐）

手机版有两条并列的同步通道，除 `syncToServer`（传到自建账号服务）之外，还有
`SyncManager.sync()`——把**同一份 schema 5 快照**同步到用户自己的 WebDAV 网盘。
桌面端此前只有前者，现已补齐（`internal/service/self_sync_service.go` + 设置页
「数据管理 → WebDAV 同步」）。

必须与手机版逐字一致的几项：

| 项 | 取值 | 依据 |
|---|---|---|
| 云端目录 | `YukiHub/` | `SyncManager.REMOTE_DIR` |
| 云端文件 | `YukiHub/YukiHub_sync.json` | `SyncManager.REMOTE_FILE` |
| 文件编码 | gzip(快照 JSON) | `compressGzip` / `decompressIfGzip` |
| 快照形态 | 云同步形态（`created_at=0`、`lightweight=true`、无 `backup_type`） | `buildLocalSnapshot()` |
| 判定基准 | 上次同步的**快照 SHA-256**，存在 `SelfSyncLastHash` | `KEY_LAST_SYNC_HASH` |
| 冲突解决 | 智能合并 / 使用云端 / 使用本地 / 取消 | `RESOLVE_{MERGE,USE_REMOTE,USE_LOCAL,CANCEL}` |
| 自动同步 | 启动时一次，距上次不足 **10 分钟**跳过，冲突按「智能合并」 | `maybeAutoWebDavSync()` |
| 地址补全 | 缺协议头补 `https://` | `validateServerUrl()` |

一条桌面端**刻意放宽**的地方（与账号云同步同源）：手机版的
`(lastHash 为空) && 云端有数据 && isSnapshotEmpty(local) → 直接下载` 在桌面端放宽为
「本地库为空 + 云端有数据 → 直接下载」，理由见上一节。

快照的构造与导入语义与账号云同步**共用同一份实现**
（`buildYukiHubSnapshot` / `importYukiHubSnapshot`），两条通道的唯一差别是传输方式。
因此「桌面端传给云端的必须和手机版一致」这条约束在两条通道上同时成立。

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
4. ~~元数据缓存的 JSON 结构是否沿用 Android 版的 `VnMetadata` 字段？~~
   **已决策（2026-09-29）**：沿用。桌面端 `game_metadata_sources.cache_json` 直接存
   `VnMetadata` JSON，导出与导入原样搬运，两端共享同一份缓存结构。

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

### 阶段 2 验收结论（2026-09-28）

- **万级样例（0 丢失、0 重复）已通过**：`internal/service/test/yukihub_scale_test.go`
  生成 10,000 游戏 / 17,501 会话 / 20,000 标签 / 10,000 元数据源的 schema 5 快照，
  经**真实 Committer** 落库后逐项核对计数与去重数，并验证重复导入幂等。
  详见 [ROADMAP.md](ROADMAP.md) 的"阶段 2 验证记录"。
- **契约字段落库缺陷已修复**：`legacy_local_id`、`source_device_id`、
  `playtime_reset_at`、`hidden` 此前在导入器 staging 表与 INSERT/UPDATE 中缺失，
  落库时被静默丢弃；现已补齐，`playtime_reset_at` 为 0 时落 NULL（不落 1970）。
  这条缺陷只有走真实落库才能暴露——**导入器的测试必须查库，不能只断言内存对象**。
- 测试矩阵中其余样例（空库、清零、聚合补偿、双端新增、时区、单位换算、
  无路径降级等）已由导入器 / 导出器的单测与集成测试覆盖（见
  `internal/service/importer/yukihub_test.go`、`internal/service/exporter/yukihub_test.go`）。

## 九、2026-10-06 审计补充（第三次逐项核对）

本轮把「数据层 / 刮削 / 统计 / 本地备份」四块逐字段比对了一遍，结论与处置如下。

### 9.1 已修：来源名单与白名单漏项

历史上「哪些来源算可信 NSFW」写死过两次，随着 `hikarinagi` / `nextmoe` /
`bangumi_mirror` 陆续加入，这些硬编码全部落后。现已统一收敛到
`gamehelper.NSFWAuthoritativeSources()` / `gamehelper.IsSupportedMetadataSource()`：

| 位置 | 原状 | 后果 |
| --- | --- | --- |
| `service/game_service.go` 的远程更新 | 硬编码 `Bangumi/VNDB/Hikarinagi` | `bangumi_mirror` / `nextmoe` 的 NSFW 永不更新 |
| `service/download_service.go` 的元数据合并 | 同上 | 同上 |
| `service/download_service.go` 的 `parseMetaSource` | 漏 `bangumi_mirror` / `nextmoe` | 下载任务带这两个来源时报 `unsupported metadata source`，并把游戏静默落成 `Local` |
| `service/mcp_read_service.go` 的来源白名单 | 漏 `bangumi_mirror` / `nextmoe` | MCP 按来源筛选时静默过滤掉用户勾选的来源 |

**有意保留的差异**：桌面端 `NSFWAuthoritativeSources` 含 `bangumi` / `bangumi_mirror`，
手机端实际只采信 `{vndb, hikarinagi, nextmoe}`。原因是**机制不同**：手机端靠
`coverSexual > 0.5` 自动判定，而 Bangumi 的解析器从不填这个字段，所以它不是「不信任
Bangumi」，而是手机端没有读取 Bangumi 的 `nsfw` 字段。桌面端各 getter 会直接产出
`IsNSFW`（Bangumi 的 R18 标记是权威的），把它一并采信是**超集且更准**，不改。

### 9.2 已修：统计不认「清零」

契约写明「`playtime_reset_at` 之前的历史会话不计入统计，但记录本身保留」，
`gamehelper.QueryGamesPlayTime`（游戏库时长/排序）与导出方向都遵守了，
但 **`stats_service.go` / `ai_stats_builder.go` 里 0 处引用该列** ——
同一款游戏的「游戏库时长」与「统计页时长」显示两个数。

手机端之所以看不到这个矛盾，是因为它重置时**直接删掉**旧会话；
桌面端按契约保留记录，所以必须在查询侧过滤。

修法：在 `stats_service.go` 定义唯一的会话来源常量 `statsSessionSource`
（`play_sessions` 与 `games` 内连接 + 清零判定，别名固定 `ps`），
把 19 处统计查询的 `FROM play_sessions` / `LEFT JOIN play_sessions ps`
统一换成它；`ai_stats_builder.go` 的 5 处同理。
回归测试：`internal/service/stats_reset_test.go`。

### 9.3 已修：`.ykbak` 导入语义与手机端不一致

手机端导入本地备份**只有一个按钮**，固定走「合并更新」（`updated_at` 守卫 +
`optString` 非空才覆盖）。桌面端 `.ykbak` 导入弹窗原本：

- 默认动作是 `skip`（已存在的游戏**什么都不更新**）；
- 三个选项里没有语义正确的 `sync_merge`（云同步内部用的就是它）。

现已在 `GameImportModal` 里：`sync_merge` 加入 `SamePathAction` 类型；
当 `source === "yukihub"` 时默认选 `sync_merge` 并多出一个按钮；
其余格式（PotatoVN / Playnite / …）保持原有默认与选项不变。

### 9.4 已验证「不是问题」的（避免重复排查）

- **Appender 与 Exec 写 `TIMESTAMPTZ` 的时区解释不一致**：
  实测（`Asia/Shanghai` 会话时区，`dbutils.AppendRows` vs `Exec`）两者写入的
  `epoch_ms` **完全相同**，当前 `duckdb-go/v2` 驱动下不可复现。另外导入的中转表
  （`temp_import_games` / `temp_import_play_sessions`）本身列类型就是 `TIMESTAMPTZ`，
  最终 `INSERT ... SELECT` 是 TIMESTAMPTZ→TIMESTAMPTZ，全程没有「字符串强转成时间」，
  因此也不受会话时区影响。
- **游玩状态取值**：两端逐字一致（`unplayed/playing/completed/onhold/dropped`），
  默认都是 `unplayed`，不存在「同步后状态显示不出来」。
- **`games` 段 / `play_sessions` 段 / `metadata_cache` 段** 的段名集合两端一致，
  本地备份与云同步是同一份负载、只差信封字段。

### 9.5 仍未修（已知、有意识保留或待决策）

| 项 | 状态 | 说明 |
| --- | --- | --- |
| 会话上限：手机全局 30 / 桌面每游戏 30 | 保留 | 桌面总时长由会话求和，少导会让回导后时长缩水；文档 §五 已说明 |
| 会话幂等键：手机按 `session_uuid` / 桌面按 `(game_id, start_time, end_time)` | 保留 | 极端情况下同 UUID 不同时间戳会插重复，文档 §四 已记录 |
| `metadata_cache` 中 `source_id` 为空的条目在桌面端被丢弃 | 保留 | 手机端该列可空、桌面端 `NOT NULL`；仅当来源 payload 本身没有 id 时触发，此时缓存无匹配价值 |
| `duration` 毫秒↔秒换算的取整误差 | 保留 | 单位不同导致的固有误差，非逻辑错误 |
| `original_title` 经桌面往返只保留第一个别名 | 保留 | 桌面 `aliases` 是数组、手机是单值列 |
| `favorite` 只增不减（取消收藏不同步） | 保留 | 手机端布尔列 → 桌面系统分类，只增不减是刻意的 |
| 统计页的 `heatmap` / `current_streak` / `active_days` / `all_sessions_*` 后端算了但前端未展示；`PlayHeatmap.tsx` 未被引用 | 保留 | **手机端没有热力图**，不展示反而与手机端一致；组件属备用 |

## 十、2026-10-06 第四轮：参考源更新（手机版 0.3.1 / 上游 LunaBox 1.13.3）

参考包：`参考文件/YukiHub-main.zip`（手机版 0.3.1，此前对齐的是 0.30p 快照）、
`参考文件/lunabox/LunaBox-main.zip`（上游 1.13.3，本仓基线 1.13.0 硬分叉）。
差异比对结果见 `.tmp_ref/mobile_api_diff.txt`、`start_diff.txt`、`mute_diff.txt`。

### 10.1 手机版 0.30p → 0.3.1 的应用层差异（共 13 个文件）

绝大多数是 Android 专有内容，与桌面端无关：品牌文案「鲲 Galgame」→「NextMoe·未萌」、
`applyDynamicTheme` 异常兜底、BigScreen 的 PV 文件夹选择器（复用系统文件选择器）、
WebView 焦点跳过、新增 `GalToolboxActivity` / `ActionButtonStyle`、
`ONLINE_READY` 打开在线展厅。**登录/注册改 POST + JSON body 桌面端早已如此。**

真正影响行为语义、且桌面端原先不一致的只有一处：**已隐藏游戏的可见性**（见 10.3）。

### 10.2 上游 LunaBox 1.13.2 / 1.13.3 的回灌判定

「抛弃」状态（`dropped`）、YukiHub 备份导入的状态/NSFW 修复、封面查看器缩放另存为、
次级排序、元数据来源排除模式、攻略文档扫描、来源 ID 映射懒加载、定时备份三件套、
Windows 管理员启动模式、Wails beta.24 —— **本仓全部已具备**（前两项本身就是本仓
维护者 `@xm486` 回灌给上游的）。Playnite ZIP 导入尚未做（功能增强，非缺陷）。

本轮实做两项上游修复：

- **配置原子写 + `.bak` 恢复**（上游 1.13.2）。老实现 `os.WriteFile` 直接覆盖
  `appconf.json`，中断/断电会留下半截 JSON，下次启动整份设置丢失。现在改为
  临时文件 → fsync → 原子替换，并维护一份可解析快照用于恢复。
  *未引入 `natefinch/atomic`*：`writeFileAtomic` 自实现（Windows 上
  `os.Rename` 走 `MoveFileEx(MOVEFILE_REPLACE_EXISTING)`），保持零新依赖。
- **Windows 后台静音残留**（上游 1.13.2）。根因是原实现每次都按 PID 重新枚举
  音频会话，而游戏退出后它的流与 PID 已经消失，枚举不到就没法解除静音，
  Windows 那层持久静音就留在系统里。现在**静音时保留 COM 会话引用**，
  直到恢复成功才释放；并新增 `stopSessionAudio`（收尾期只解静音、3 次短重试）、
  进程退出即恢复、多输出设备兜底恢复、恢复不到会话时报错而不静默清状态。
  已移植上游的纯状态机单测 `TestRetainedAudioRestorationKeepsOnlyFailedSessions`
  （不需要音频设备），另两个需真实输出设备、`YUKIHUB_AUDIO_INTEGRATION=1` 才跑。

### 10.3 已修：隐藏游戏语义（对齐手机版）

手机版 0.3.1 新增 `GameRepository.getHiddenGames()` 与 `setHidden()`，并把
`getAll()` 的过滤条件固定为 `hidden=0`（除隐藏管理入口外，所有页面都看不到隐藏游戏）。
桌面端原先**完全没有隐藏入口**，且 `GetGames` 不传 `exclude_hidden` 时会把隐藏游戏
一起列出来 —— 表现为「手机上隐藏的游戏在电脑库里照常出现，且无法恢复」。

- 新增 `GameService.SetGameHidden(gameID, hidden)`：**只**改 `hidden` 与 `updated_at`，
  不走 `UpdateGame` 的整行覆盖（详情页手上的对象可能已过期）。`updated_at` 必须推进，
  否则云同步的「对端不早于本地」判断会让这次隐藏永远传不过去。
- 游戏库 / 收藏页 / 首页「最近游玩」默认排除隐藏游戏；游戏库过滤器新增
  「显示已隐藏的游戏」开关（`localStorage` 记忆），配合批量操作里的
  「隐藏所选 / 取消隐藏所选」构成完整的隐藏与恢复闭环。
- 大屏模式沿用既有的 `bigscreen_show_hidden_game` 开关，语义已一致，未改。

### 10.4 仍未做（留待后续）

| 项 | 判断 |
| --- | --- |
| Playnite 导入支持 ZIP（含封面） | 上游 1.13.2 的功能增强，本仓仍只支持 JSON；工作量中 |
| 1.13.3 抽屉定位异常 / 玻璃光晕 | 上游 1.13.2 引入的 UI 缺陷修复，本仓前端已重度分叉且未复现同类问题 |
| 元数据来源下拉里给 NextMoe 加「推荐」字样 | 桌面端是**多选开关列表**而非单选下拉，已改为把 NextMoe 排到 VNDB 之后（对齐手机版次序），推荐语义写进来源说明文案 |
| 手机版更新源单选（GitCode / GitHub） | 桌面端走 LunaBox 自己的 manifest 更新服务，机制不同，不适用 |

## 十一、2026-10-07 第五轮：三条通道（账号云同步 / WebDAV / 本地备份）一致性复核

用户诉求：确认「WebDAV 自持同步、账号服务器同步、本地 `.ykbak` 备份」导出与导入的
是同一份东西，并继续跟进手机端。

### 11.1 复核结论：导出侧三通道已经等价（本轮补上回归测试）

三条通道共用同一份负载构造：账号云同步与 WebDAV 都走
`buildYukiHubSnapshot` → `exporter.Build()`（云同步形态），本地 `.ykbak` 走
`ExportToYukiHub` → `exporter.BuildLocalBackup()`（本地形态）。两者最终都调用
`exporter.build(kind)`，**负载完全一致，只有信封字段不同**：

| 字段 | 云同步 / WebDAV | 本地 `.ykbak` | 手机版依据 |
| --- | --- | --- | --- |
| `created_at` | `0` | 当前毫秒 | `SyncManager.buildLocalSnapshot` vs `MainActivity.exportLocalBackup` |
| `note` | 云同步文案 | 本地备份文案 | 同上 |
| `backup_type` | 省略 | `local_full` | 仅 `exportLocalBackup` 追加 |
| 其余全部 | 相同 | 相同 | 同一个 `buildLocalSnapshot(30)` |

三处也都注入了 `profile`（昵称 / 头像）与 `settings.metadata_source`，不存在
「某条通道少带一段」的情况。

**新增回归测试**：`internal/service/test/yukihub_export_test.go` 的
`TestYukiHubThreeChannelsExportIdenticalPayload` —— 用真实库构造带别名 / 清零 /
收藏 / 标签 / 元数据缓存 / 本地封面 / 无标题条目的数据，把两份快照的信封归一化后
断言**逐字节相等**，并断言云同步形态不含 `backup_type`。以后谁在某条通道上单独加
字段，这个测试会直接失败。

### 11.2 已修：封面候选只取「首个非空」

`firstNetworkURL(CoverSourceURL, CoverURL)` 的实现与自身文档不符：文档写「返回第一个
http(s) 地址」，实现却返回**第一个非空**。若 `cover_source_url` 是本地路径
（历史数据 / `content://`），后面那个有效的网络 `cover_url` 会被直接挤掉，导致封面
明明有网络地址却不同步。现改为逐个候选做 `networkCoverURI` 校验，返回首个可跨设备
的地址（`TestFirstNetworkURLPrefersUsableRemoteCover`）。

### 11.3 已修：导入侧采纳快照里的全局资料源（对齐手机版）

手机版 `SyncManager.importSnapshot` 读到 `settings.metadata_source` 会
`putString(KEY_METADATA_SOURCE, source)`，即**导入侧采纳对端的全局资料源**；
桌面端此前只导出不回写，于是：

- 手机端把资料源改成 nextmoe → 同步到桌面端**不变**；
- 桌面端下一次上传带上自己的旧值（如 vndb）→ **把手机端的设置顶回去**，来回翻。

现在三条导入通道（账号同步 / WebDAV / 本地 `.ykbak`）都会采纳它：

- 导入器 `YukiHubImporter` 新增 `MetadataSource()`，把快照声明的值原样透出（不碰配置）；
- service 层 `resolveImportedMetadataSource()` 做决策：**白名单外 / 缺失 / 与当前一致
  一律不动**。白名单 = `appconf.IsSelectableMetadataSource`，正好是手机版 importSnapshot
  接受的那六个值（vndb / bangumi / bangumi_mirror / ymgal / hikarinagi / nextmoe）——
  桌面端自己的 `IsSupportedMetadataSource` 更宽（含 steam / dlsite 等），不能直接用，
  否则会把手机端不认识的来源当成跨端偏好写过去；
- 变化时落盘 `appconf.SaveConfig` 并广播 `yukihub-sync:applied`，让前端刷新配置
  （否则设置页草稿还是旧值，用户接着点「保存」会把它写回去）。

测试：`importer/yukihub_test.go`（资料源透出、缺失时不造值）、
`service/import_metadata_source_test.go`（白名单 + 采纳决策）。

### 11.4 仍保留的差异（本轮复核后确认有意）

| 项 | 说明 |
| --- | --- |
| 会话条数上限：手机全库 30 / 桌面每游戏 30 | 桌面总时长由会话求和，少导会让回导后时长缩水（§五） |
| `settings` 段只写 `metadata_source` | 手机端的排序 / 缩放 / 扫描等属设备本地偏好，桌面端无对应概念；手机端用 `has()` 守卫，缺失即保留其原值，因此不写是安全的 |
| 手机端 `buildLocalSnapshot` 会把 `settings` 写满十几个键 | 桌面端没有这些概念，写过去只会用桌面值覆盖手机偏好 |

## 十二、2026-10-07 第六轮：大屏模式全面对齐（S4/S5/S6 补齐）

用户反馈「大屏模式随便看一眼全是问题，详情甚至直接跳转到游戏库」。对照手机端
`com.yuki.yukihub.bigscreen`（20 个类）逐项复核后，本轮补齐了**浮层菜单体系**与
**大屏内设置**，并把卡片/侧栏/提示条对齐手机端的视觉与键位。

### 12.1 修掉的核心问题：「详细」会跳出大屏

手机端详情层按钮是「游玩 / 观看 PV（有预告片才显示）/ 详细」，其中「详细」调的是
`onRequestGameMenu` —— **打开大屏内的游戏操作菜单**，不是跳去别处。桌面端此前把它接到
`navigate(/game/:id)`，等于把用户从大屏踢回普通界面（用户看到的「跳转到游戏库」）。

现在三层分工明确：

| 入口 | 行为 |
| --- | --- |
| 货架卡片 Ⓐ / 点击 | 启动游戏 |
| 货架 Ⓨ / 卡片右键 | 打开**详情层**（`BigScreenDetailsLayer`） |
| 详情层「详细」、信息浮层「更多」 | 打开**游戏操作菜单**（S4） |
| 游戏操作菜单「编辑信息」 | 才跳出大屏去完整详情页（手机端的 `jumpToTouchMode`） |

### 12.2 新增：通用浮层菜单（`BigScreenPanel.tsx`）

对齐手机端 `BigScreenPanel`：右侧滑入、遮罩**消费点击**（手机端 M16 用户投诉过
「你不就做了层透明布」）、条目支持图标 + 主文案 + 副文案 + 分隔线、标题、底部提示。
面板打开时吞掉除上下/确认/返回之外的输入。

- **S4 游戏操作菜单**：启动游戏 / 收藏 / 游玩状态循环 / 设置·更换·移除预告片 /
  编辑信息 / 打开游戏目录 / 在库中隐藏（二次确认）/ 从库中移除（二次确认）。
- **S5 主菜单**（顶栏 ☰、`Tab`）：设置 / 隐藏游戏管理 / 随机选一款 / 切换排序方式 /
  快捷键说明 / 退出大屏。
- **隐藏游戏管理**：后端没有「只看隐藏」的过滤，取一份含隐藏的列表再本地筛
  （手机端同样是内存里筛）。

### 12.3 新增：大屏内设置面板（`BigScreenSettingsLayer.tsx`）

对齐手机端 `BigScreenSettings` 的**左列分区 + 右列条目**两列焦点模型，四个分区：
常规 / 视觉 / 音频 / 预告片。`select` 类条目复用通用浮层选值，`switch` 就地翻转。

设置项的**单一事实来源**是 `bigscreen/settingsSchema.ts`，设置页的「大屏模式」分区
（`components/panel/BigScreenSettingsPanel.tsx`）与大屏内面板共用它，保证两处可调项
永远一致。文案在 schema 里就翻译好：项目的 i18n 提取器只认字面量翻译调用，如果存
labelKey 再由渲染层动态查表，`pnpm i18n:clean` 会把这些键当"未引用"删掉并报错。

### 12.4 新增：顶栏与入场动画

- `BigScreenTopBar.tsx`：时间 + 手柄连接状态 + ☰ 菜单入口。桌面端没有电量/网络/
  触摸模式的概念，只保留对客厅大屏有意义的两项。
- `BigScreenIntro.tsx`：内置入场动画（logo 淡入上浮 + 光带扫过 + 整层淡出），
  任意输入可跳过；由 `bigscreen_intro_enabled` 控制。手机端的自选入场视频不迁移
  （需要视频选择器与 SAF，桌面端无对应入口）。

### 12.5 卡片 / 侧栏 / 底栏对齐

- 新增大屏专用卡片 `BigScreenCard.tsx`，**不再复用** `GameCard`：游戏库那张卡带状态文字
  徽标、评分芯片、排序字段覆盖条与悬浮位移，是给鼠标精读用的；大屏卡片只要
  「封面 + 可选标题 + 状态点/收藏/R18 角标 + 未聚焦压暗」。
- 侧栏补**条目计数**（`limit=1` 逐分类取 `total`，只跑 COUNT）、支持「侧栏常驻展开」。
- 底栏提示改用手机端 `BigScreenKeys` 的按键字形（Xbox ⒶⓍⒷⓎ / PS ✕□○△ 可切换），
  并补上「← 进筛选栏」「☰ 菜单」；`bigscreen_hint_mode` 支持自动淡出 / 常显 / 隐藏。
- 排序不再是每个分类写死：主菜单可在「最近游玩 / 最近加入 / 按名称」之间切换。

### 12.6 新增配置项（`appconf`）

`bigscreen_show_titles` / `bigscreen_card_scale` / `bigscreen_focus_scale` /
`bigscreen_key_style` / `bigscreen_hint_mode` / `bigscreen_rail_expanded` /
`bigscreen_sound_volume` / `bigscreen_focus_ticks` / `bigscreen_intro_enabled` /
`bigscreen_trailer_enabled` / `bigscreen_trailer_muted` / `bigscreen_trailer_delay_ms` /
`bigscreen_pv_fit` / `bigscreen_pv_scrim` / `bigscreen_pv_scrim_percent`。

`NormalizeBigScreenPreferences` 统一收敛：枚举白名单 + 数值夹取（`card_scale=0` 会把
卡片宽度算成 0、整个货架消失，必须夹住）。默认值与手机端 `BigScreenPrefs` 对齐，
**其中 `trailer_muted` 默认 false**（手机端 M18 修过「PV 没声音」的坑）。

### 12.7 仍未对齐（记入 ROADMAP）

| 项 | 原因 |
| --- | --- |
| 详情层的 INTRODUCTION 截图画带 | 需要元数据截图列表，桌面端 `models.Game` 暂无对应字段 |
| 游戏操作菜单的「标题图 / 背景图」 | 手机端有 `logo_path` / `bg_path` 两列与私有目录，桌面端需要 schema 变更 |
| 入场动画自选视频、PV 占用与清理面板 | 依赖 SAF / 受管视频目录的等价能力 |
| 触摸模式（`touchUi`） | 桌面端以鼠标 + 手柄为主，不需要"不预选焦点"的触摸分支 |

## 十三、2026-10-07 第七轮：大屏交互与设置继续对齐（PC 化收紧）

用户要求「继续对齐、继续审计，以手机版为准，但 UI 要更 PC 一些（比手机版小）」。
对照手机端 `com.yuki.yukihub.bigscreen` 的 FocusEngine / InputRouter / BigScreenSizes /
BigScreenSettings / BigScreenPrefs / BigScreenBanner 逐项复核后，本轮修掉了一批
**交互语义错误**，补齐了设置分区与提示条，并把整体尺寸收紧一档。

### 13.1 修掉的焦点穿梭错误（真 bug）

| 症状 | 手机端行为 | 修复前 | 修复后 |
| --- | --- | --- | --- |
| 卡片排按 ↑ | 进**信息浮层按钮排**（`setInfoZone(true)`） | 进侧栏 | 进按钮排 |
| 卡片排按 ↓ | 到边界即停 | 进按钮排（按钮排在下方） | 到边界即停 |
| 卡片排按 ← | `col==0` 时进侧栏 | 进侧栏 ✓ | 不变 |
| 侧栏按 ←/→ | ← 不处理、→ 回内容区 | 被拦成「切分类」，焦点下标不动 → **焦点与当前分类脱节** | ← 吞掉、→ 回货架 |
| 侧栏按 Ⓐ | 应用当前聚焦的分类后回内容区 | 只回货架，**分类根本没切** | 应用分类再回货架 |

根因是 `BigScreenSizes` 的层级关系被搞反了：信息浮层（含按钮排）压在货架**上方**，
所以「上=按钮排、下=回货架」；而旧实现把 `FOCUS_ORDER` 写成 `[SHELF, ACTIONS]`。
另外 `focusEngine.verticalNeighbor` 会把**未登记在 `verticalOrder` 里的区域追加到末尾**，
于是「货架按 ↓」被引擎丢进侧栏 —— 现在只在该顺序内相邻穿梭（侧栏/详情层不参与）。

### 13.2 补 MENU 意图与手柄按键映射

- `BigScreenIntent` 新增 `menu`：键盘 `Tab`、手柄 `Start` 都走它，浮层里会被各自吞掉
  （对齐手机端「☰ 打开主菜单」；浮层打开时不再能穿透去开菜单）。
- 手柄映射补齐：按钮 8（Select/Back）与 Mode → back，按钮 9（Start）→ menu，
  扳机 6/7（L2/R2）与 4/5（LB/RB）同为翻分类。
- **连发只保留方向键**：手机端 `InputRouter.isDirection` 只覆盖上下左右，
  LB/RB 按住不连发（之前 PC 给 LB/RB 开了连发，按住会疯狂切分类）。

### 13.3 实现按分类的焦点记忆

`FocusEngine` 早有 `saveMemory/restoreMemory` 但从未被调用。现在：
切分类前把当前货架下标存进 `shelfMemoryRef`，新分类数据到位后恢复该分类上次的位置
（没有记忆才回第一张）。对齐手机端 `setMemoryKey(filter)` —— 以筛选 id 为记忆键。

### 13.4 新增大屏顶部提示条（`BigScreenBanner`）

对齐手机端 `BigScreenBanner`（spec §S7）：顶部滑入 + 淡入，停留后自动滑出，
`message.id` 变化即重播动画。手机端 M14-3 两轮反馈后回退到「只要一次干净的淡入」，
这里保持一致。接入点：**手柄连接/断开**（M18-2：入场动画结束后也提示一次）、
收藏、切换排序、清除筛选记忆、恢复默认设置。停留时长由 `bigscreen_banner_hold_ms`
控制（默认 2000ms，可配 1.2/2/3/4.5 秒），入场动画期间抑制。

### 13.5 设置分区补齐（4 → 5 个）

对齐手机端的分区结构（去掉桌面端无对应概念的「兼容」= KR 存档兜底）：

| 分区 | 条目 |
| --- | --- |
| 常规 | 性能档 / 入场动画 / 默认分类 / 显示已隐藏游戏 / **记住筛选** |
| 视觉 | 卡片大小 / 显示标题 / 焦点缩放 / **背景氛围** / **NSFW 封面模糊** / 侧栏常驻展开 / 按键提示条 / 按键图标风格 |
| 音频 | 界面音效 / 音效音量 / 焦点音 / PV 静音 |
| 布局 | **提示条时长** / 背景预告片 / PV 播放延迟 / **仅在详情层播放 PV** / PV 显示方式 / PV 遮罩 / PV 遮罩强度 |
| 菜单 | **清除筛选记忆** / **恢复默认设置**（二次确认） |

schema 新增 `action` 类条目（右侧按钮 + 状态文案）。「恢复默认设置」写回
`BIG_SCREEN_DEFAULT_CONFIG`（与 Go 侧 `defaultAppConfig()` 的大屏段一一对应），
**不碰主库设置**；「NSFW 封面模糊」直接读写主库共用的 `blur_nsfw_game_covers`。

### 13.6 UI 尺寸 PC 化收紧

用户要求「比手机版小」。手机端所有尺寸都由屏幕短边驱动，桌面端沿用同一套比例但
把系数压一档，让整体密度更高：

| 项 | 手机端锚点 | 修复前 | 现在 |
| --- | --- | --- | --- |
| 封面占内容区高度 | ~27% | 28%（夹 220–400） | **22%（夹 180–320）** |
| 封面高度下限 | 96dp | 130 | **110** |
| 信息浮层预留 | 34%（夹 100–150） | 30%（夹 120–190） | **24%（夹 96–150）** |
| 行标题高度 | 20%（夹 20–34） | 20%（夹 24–44） | **18%（夹 22–36）** |
| 卡片横向间距 | 9dp | 14 | **12** |
| 卡片宽上限 | 屏宽 17% | 12% | **10.5%** |
| 侧栏宽度 | 50–78 / 168–260 | 76 / 216 | **68 / 200** |
| 信息浮层标题 | 18.7sp | text-4xl (36px) | **text-3xl (30px)** |
| 详情层标题 | — | text-5xl (48px) | **text-4xl (36px)** |
| 面板宽度 | — | min(420px, 34vw) | **min(360px, 30vw)** |

字号、按钮内边距、面板/设置层的左列宽度也同步收了一档。

### 13.7 仍未对齐

| 项 | 原因 |
| --- | --- |
| 详情层的 INTRODUCTION 截图画带 | 需要元数据截图列表，桌面端 `models.Game` 尚无对应字段 |
| 游戏操作菜单的「标题图 / 背景图」（`BigScreenArt`） | 手机端有 `logo_path` / `bg_path` 两列与私有目录，桌面端需要 schema 变更 |
| 入场动画自选视频、PV 占用与清理面板 | 依赖 SAF / 受管视频目录的等价能力 |
| 触摸模式（`touchUi`）分支 | 桌面端以鼠标 + 手柄为主，不需要「不预选焦点」的触摸分支 |
