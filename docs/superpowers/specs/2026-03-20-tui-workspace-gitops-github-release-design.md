# Bubble Tea 多 Repo Workspace TUI 設計（v1）

日期：2026-03-20  
狀態：Draft（已完成腦力激盪確認）

## 1. 目標與範圍

建立一個以 `Bubble Tea` 為核心的終端 TUI，整合以下工作流：

- 本地：workspace、repo 選擇、worktree 建立/切換
- Git 操作：在 app 內嵌 `lazygit`
- Diff 體驗：整合 `delta` 輸出能力
- GitHub 觀測：PR、CI、Release publish 狀態

v1 採「端到端最小骨架」策略，先打通完整流程：

`workspace 選 repo -> 建/切 worktree -> 內嵌 git 操作 -> 建 PR -> 看 CI -> 看 release publish`

## 2. 已確認決策

- 架構路線：模組化核心 + Adapter（Approach 2）
- 平台：macOS + Linux
- repo 加入方式：手動加入（輸入路徑）
- workspace：支援多 workspace（建立/切換）
- 中心區：多 Tab；其中一個 Tab 為 `Lazygit`
- Lazygit 行為：切到 Lazygit Tab 顯示「當前選中 repo」；未選 repo 顯示提示
- Lazygit session 記憶：v1 best-effort，不保證完整還原
- Release 追蹤來源：GitHub Release workflow
- Release 顯示位置：右側 Pane，且僅顯示「當前選中 repo」狀態
- 更新策略：手動 refresh + 自動輪詢（預設啟用，可關閉）
- GitHub 認證：只重用 `gh auth`（不做內建 OAuth）
- PR 建立路徑：v1 僅透過 `Lazygit`（或其 custom command 觸發 `gh`）；不做原生 PR 表單

## 3. 非目標（v1 明確不做）

- 不做 popup terminal
- 不做 Windows 支援
- 不保證每個 repo 的 Lazygit 完整 session 持久化
- 不做多種類 publish（先專注 GitHub Release workflow）
- 不在 v1 自建完整 lazygit/delta 替代品

## 4. 架構總覽

### 4.1 Core 模組

- `AppShell`：Bubble Tea 主事件循環、全域 keymap、三欄佈局、Center Tab 管理
- `StateStore`：UI 狀態、workspace/repo 選擇、同步快照持久化
- `SyncEngine`：手動與自動輪詢調度、錯誤退避與狀態標記

### 4.2 Adapter 模組

- `WorkspaceAdapter`：workspace/repo 手動管理
- `WorktreeAdapter`：worktree 建立、列舉、切換（對齊 worktrunk/lazyworktree 常用流）
- `EmbeddedLazygitAdapter`：在 Center Lazygit Tab 內啟動 PTY + lazygit
- `DiffAdapter`：用 `delta` 呈現 diff（供 diff 視圖/操作入口使用）
- `GitHubAdapter`：透過 `gh auth` 上下文讀取 PR、CI、Release workflow 狀態

### 4.3 模組邊界

- UI 不直接呼叫 GitHub API，只透過 `GitHubAdapter`
- Lazygit 與 GitHub 輪詢隔離，避免互相阻塞
- Center Pane 是主要操作區；Right Pane 為所選 repo 的狀態檢視

## 5. UI 與互動設計

### 5.1 版面

- Left Pane：workspace 列表、repo 列表、worktree 快速操作
- Center Pane（Tabs）：
  - `Overview`：目前 repo 的摘要、近期活動與入口
  - `Worktrees`：worktree 清單與建立/切換操作
  - `Lazygit`：內嵌 lazygit（同一 TUI 內，不跳出）
  - `Diff`：唯讀 diff 視圖，使用 `delta` 呈現當前 repo/worktree 差異
- Right Pane：當前 repo 的 `PR / CI / Release` 狀態

### 5.2 Lazygit Tab 行為

- 有選 repo：啟動或重用該 repo 的 lazygit PTY session
- 無選 repo：顯示「請先選擇 repo」
- session 還原：best-effort（v1）

### 5.3 PR 建立行為（v1 邊界）

- v1 不提供原生 PR 建立表單
- PR 建立由 `Lazygit` Tab 內流程完成（含 lazygit 既有能力或呼叫 `gh` custom command）
- App 只負責觀測 PR/CI/Release 狀態，不改寫 PR 建立 UX

### 5.4 Diff 行為（v1 邊界）

- `Diff` Tab 只做唯讀顯示，不做 hunk 編輯/stage 互動
- 只針對當前選中 repo/worktree 執行 diff
- 呈現來源固定為 `git diff` 管線至 `delta`，不自建差異演算法

## 6. 資料模型（v1）

### 6.1 Workspace

- `id`
- `name`
- `repos[]`
- `selectedRepoId`
- `createdAt / updatedAt`

### 6.2 Repo

- `id`
- `name`
- `path`
- `defaultBranch`
- `selectedWorktreeId`

### 6.3 Status Snapshot

- `repoId`
- `prStatus`
- `ciStatus`
- `releaseStatus`
- `lastSyncedAt`
- `isStale`
- `error`（nullable）

## 7. 主要資料流

1. 啟動：讀本地持久化 -> 還原上次 workspace/repo -> 啟動輪詢器  
2. 使用者操作：選 workspace/repo -> 切 Center Tab -> 執行 worktree/lazygit/diff  
3. 手動 refresh：立即觸發「當前選中 repo」GitHub 同步  
4. 自動輪詢：僅輪詢「當前選中 repo」的 PR/CI/Release  
5. 同步結果：寫入 Store -> UI 重繪 -> Right Pane 更新  
6. 切換 repo：立即觸發一次新 repo 的同步，再進入週期輪詢  

Release 追蹤規則（v1）：

- 每個 repo 需在設定中提供 `releaseWorkflowRef`（workflow 檔名或 workflow id）
- 以 `releaseWorkflowRef` 對應到單一 workflow，避免多 workflow 歧義
- 追蹤該 workflow 在 repo 預設分支脈絡下的最新執行狀態（`queued/in_progress/success/failure`）
- 不因 PR merge 完成而停止追蹤 publish

若 repo 未設定 `releaseWorkflowRef`：

- release 狀態顯示 `unconfigured`
- 提示使用者補上 workflow 設定

## 8. 錯誤處理

- `gh auth` 不可用：顯示引導（先執行 `gh auth login`），遠端卡片停用但本地操作可用
- GitHub API 暫時失敗/rate limit：保留最後成功快照，標記 `stale`，下輪自動重試
- lazygit 不存在或啟動失敗：在 Lazygit Tab 顯示修復提示，不影響其他功能
- repo 路徑失效：在清單標記異常，提供移除或修正路徑
- repo 缺少 `releaseWorkflowRef`：Right Pane 顯示 `unconfigured` 並提供設定入口提示

## 9. 測試策略

### 9.1 單元測試

- workspace/repo CRUD 與選擇狀態
- Store 載入與持久化
- GitHub 回應到 UI model 的映射（PR/CI/Release）

### 9.2 整合測試

- 手動 refresh + 自動輪詢的狀態更新
- Lazygit Tab 三情境：有 repo、無 repo、lazygit 缺失
- Worktree 建立/切換流程與狀態同步

### 9.3 E2E Smoke

- 建 workspace -> 加 repo -> 建/切 worktree -> 切 Lazygit -> refresh -> 看到 release 狀態

## 10. 後續擴充方向（v2+）

- repo 自動掃描加入 workspace
- 更完整 lazygit session 還原
- publish 類型擴充（deploy/package）
- Windows 支援
