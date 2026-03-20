# Folder-First + Optional Workspace 設計（v2）

日期：2026-03-21  
狀態：Approved（brainstorming 已確認）

## 1. 目標

把產品調整為「預設像 VSCode 開資料夾就能用」，workspace 變成可選能力：

- `gh-workspace`：直接用當前資料夾
- `gh-workspace -f <path>`：指定資料夾
- `gh-workspace -w <name>`：打開既有 workspace

同時保留既有多 repo workspace 能力與 DB 儲存方式（不改為 workspace 檔案）。

## 2. 已確認決策

- Workspace 是可選，不是必填
- 儲存形式：DB-only（沿用現有 BoltDB）
- 採「單一模型 + 隱藏 local workspace」方案
- Folder Mode 為單一 repo（不是 recent list）
- 無參數 `gh-workspace` 永遠走 Folder Mode，不自動跳命名 workspace
- `gh-workspace -w <name>` 找不到 workspace 時報錯退出，不自動建立
- `gh-workspace` / `-f` 指向非 git repo 時，不退出；進入空狀態
- `-f <non-git>` 會清掉 Folder Mode 目前 repo，避免保留舊資料造成混淆

## 3. 非目標

- 不新增 workspace 檔案匯入/匯出
- 不新增 Folder Mode 多 repo
- 不改動 GitHub status、worktree、lazygit、diff 的核心邏輯
- 不在本階段改動跨機同步/分享機制

## 4. 模式與資料模型

### 4.1 執行模式

- `Folder Mode`：以單一 repo 操作；UI 隱藏 workspace 管理
- `Workspace Mode`：使用命名 workspace；可含多 repo

### 4.2 資料層策略

沿用現有 `workspace.State` 結構，不新增第二套 schema。

引入內部保留 workspace：`__local_internal__`（ID）

- 僅供 Folder Mode 使用
- 不在一般 workspace UI 中顯示
- 儲存 Folder Mode 的單一 repo 選擇

補充保留規則：

- 系統 workspace 使用固定 `ID` 與固定 `name`：`__local_internal__`
- `gh-workspace -w <name>` 僅以 `name` 解析，而且解析時排除系統 workspace
- `gh-workspace -w __local_internal__` 視為保留名稱，直接報錯
- 若偵測到舊資料已存在同 ID 的使用者 workspace，啟動時先自動改 ID（例如加 `-legacy` 後綴）再建立系統 workspace

### 4.3 Folder Mode 單一 repo 不變式

`__local_internal__` 在任何時刻最多保留一個 repo：

- 開新 folder 成功：覆蓋為新 repo
- 開非 git folder：清空 repo，進空狀態

## 5. CLI 契約與行為

### 5.1 命令

- `gh-workspace`
  - 等價於 `open-folder($PWD)`
- `gh-workspace -f <path>`
  - `open-folder(<path>)`
- `gh-workspace -w <name>`
  - `open-workspace(<name>)`

### 5.2 參數規則

- `-f` 與 `-w` 互斥
- `-f ""` 或 `-w ""` 視為參數錯誤

### 5.3 錯誤與退出規則

- `-w <name>` 找不到：印 `workspace not found: <name>`，非 0 退出
- `-f/-w` 參數錯誤：印 usage，非 0 退出
- `gh-workspace` 或 `-f <path>` 指向非 git repo：不退出，啟動 TUI 空狀態並顯示提示
- `-f <path>` 路徑不存在或不可讀：同樣視為 non-git 情境（進空狀態 + 提示），不直接崩潰退出

## 6. 狀態轉換

### 6.1 open-folder(path)

1. 路徑正規化（絕對路徑）
2. 若可進入 git repo，將路徑正規化為 repo root（`git rev-parse --show-toplevel`）
3. 判斷是否 git repo
4. 若是 git repo：
   - 確保 `__local_internal__` 存在
   - 用該 repo 覆蓋 `__local_internal__` repo 清單（只留一個）
   - 選中 `__local_internal__` + 該 repo
   - 啟動 `Folder Mode`
5. 若非 git repo（含路徑不存在/不可讀）：
   - 確保 `__local_internal__` 存在（first-run 也成立）
   - 清空 `__local_internal__` repo
   - 選中 `__local_internal__`
   - 啟動 `Folder Mode` 空狀態

### 6.2 open-workspace(name)

1. 依名稱尋找 workspace
2. 找到：選中該 workspace，啟動 `Workspace Mode`
3. 找不到：回傳錯誤並退出（不 fallback）

## 7. UI 與互動

### 7.1 Left Pane

- `Folder Mode`：顯示 `Repos` + `Worktrees`；隱藏 `Workspaces`
- `Workspace Mode`：顯示 `Workspaces` + `Repos` + `Worktrees`

### 7.2 快捷鍵

- `Folder Mode`：`[` `]` workspace 切換鍵停用
- `Folder Mode`：`a` 新增/切換 repo 時必須維持單一 repo 不變式（有效 git 路徑則替換；無效路徑則清空並提示）
- `Workspace Mode`：`[` `]` 正常工作
- 其他既有鍵（tab、1..4、r、p、lazygit/diff 相關）保持一致

### 7.3 空狀態文案

Folder Mode 空狀態提供明確訊息：

- `current folder is not a git repo`
- 引導：可用 `a` 加入 repo，或改用 `-w <name>` 開 workspace

## 8. 一致性與相容性

- 不破壞既有 workspace 資料
- 既有命名 workspace 流程維持不變
- Right Pane 的 PR/CI/Release 仍只跟隨「當前選中 repo」
- Worktree/Lazygit/Diff tab 不依模式分叉邏輯

## 9. 測試策略（驗收導向）

### 9.1 CLI 驗收

- `gh-workspace` 在 git repo 目錄 -> Folder Mode + 選中 `$PWD` 所屬 repo root
- `gh-workspace` 在非 git repo 目錄 -> Folder Mode 空狀態
- `gh-workspace -f <git-repo-or-subdir>` -> Folder Mode 單一 repo 為該路徑所屬 repo root
- `gh-workspace -f <non-git>` -> 清空 Folder repo + 空狀態
- `gh-workspace -w <existing>` -> Workspace Mode
- `gh-workspace -w <missing>` -> 報錯退出
- `-f` + `-w` -> 參數錯誤退出
- 首次啟動於 non-git 目錄 -> 自動建立 `__local_internal__`，並進空狀態
- 舊資料若已使用 `__local_internal__` ID -> 啟動時先執行 ID 遷移，再建立系統 workspace

### 9.2 TUI 驗收

- Folder Mode 不顯示 Workspaces 區塊
- Folder Mode 下 `[` `]` 無效果
- Workspace Mode 顯示 Workspaces 且 `[` `]` 正常
- Folder Mode 永遠只顯示單一 repo

### 9.3 回歸測試

- 現有 sync/worktree/lazygit/diff 測試維持通過
- 既有 workspace CRUD 與選擇邏輯不退化

## 10. 實作邊界（供後續 planning）

- 以最小改動落地：CLI 入口解析 + runtime 初始狀態注入 + View/Key 行為分支
- 禁止順便擴充為多本地 repo 或 workspace file 格式
- 如需後續「Folder/Workspace 互相移動 repo」能力，另開變更，不併入此範圍
