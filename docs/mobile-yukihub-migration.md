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
| `favorite` | 归类到"收藏"系统分类 | 复用 `categories` 的 `is_system`（`system:favorites`）；导入方向已于 2026-09-28 补齐 |
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

**⑤ "想玩"在往返后消失（契约层面的信息损失）**

桌面端 6 态、Android 5 态：导入方向 `unplayed` 无法区分"想玩"与"未开始"
（落 `not_started`）；导出方向 `want_to_play` 降级为 `unplayed`。
双向都无法还原。若要保守语义，快照需额外携带对端原始状态。暂不处理。

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
- 条数上限：每个游戏只导出最新的 30 条会话；`total_play_time` 仍按全部（清零之后）会话统计。
- **`root_uri` 恒为空串**，不写 Windows 绝对路径。对端 Android 会走
  `findByTitleForEmptyRoot` 按标题匹配；这与导入方向对称（桌面端导入 Android 备份时
  同样不把对端的 `content://` 路径写进 `path` / `game_directory`）。
  *遗留*：Android 源生条目（本地 `root_uri` 非空）经桌面端回导时仍可能在对端产生重复，
  彻底解决需新增 `legacy_root_uri` 列保存对端原始路径并在导出时回填。
- 无标题条目（`name` 去掉空白后为空）跳过导出：对端会把空标题落成"未命名游戏"，
  只会制造无法匹配的占位记录。
- `local_id` 由 `games.legacy_local_id` 还原（保留的 Android 整数 ID），非法或缺失时为 0。
- `original_title` 取第一个非空别名；`tags` 用英文逗号拼接。
- `play_status` 做 6 → 5 态映射：Android 没有"想玩"，`want_to_play` 降级为 `unplayed`。
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
