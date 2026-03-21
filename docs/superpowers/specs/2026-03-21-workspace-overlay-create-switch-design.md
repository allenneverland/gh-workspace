# Workspace Overlay（Create / Switch + fzf-like Repo Picker）設計

日期：2026-03-21  
狀態：Approved（brainstorming 已確認）

## 1. 目標

在現有 TUI 內加入單一 `w` 入口的 Workspace Overlay，支援：

- 顯示 workspace list 並直接切換
- 進入 create flow 建立新 workspace
- 在 create flow 內用左側 fzf-like 搜尋 repo，加入右側新 workspace 草稿

關鍵體驗要求（已確認）：

- `w` 在 `folder mode` 與 `workspace mode` 都可用
- 進入 create 模式後左側上方有路徑輸入框
- 路徑輸入「即時掃描」（不需 enter）
- `c` 進入建立流程；`s` 一鍵儲存 + 關閉 + 切換
- 建立流程資料皆為暫存；`esc` 直接丟棄全部草稿
- 重複 workspace 名稱：阻擋並顯示錯誤
- 重複加入 repo：忽略並顯示 `already added`
- 允許建立空 workspace（`staged repos` 可為空）

## 2. 非目標

- 不整合外部 `fzf` binary（本版本採內建 fzf-like 搜尋體驗）
- 不新增跨 session 記住上次掃描路徑
- 不改動現有 `init` 子命令語意
- 不做 workspace import/export 或 DB schema 重構

## 3. 使用者流程

### 3.1 開啟與切換

1. 使用者按 `w` 開啟 overlay（任一 UI mode 皆可）
2. 預設只顯示 workspace list（不顯示 repo 掃描輸入框與 fzf 候選）
3. 在 workspace list 選中項目後按 `enter`：
   - 關閉 overlay
   - 切換到該 workspace
   - 若原本是 folder mode，直接切成 workspace mode

### 3.2 建立流程

1. 在 overlay 按 `c`，進入 create 模式
2. 輸入新 workspace name（右側輸入框）
3. 左側輸入掃描路徑，系統即時掃描並更新候選 repo
4. 在左側候選清單按 `enter`，把 repo 加進右側 staged repos
5. 按 `s`：
   - 驗證名稱與資料
   - 一次性寫入（建立 workspace + 加入 repos + 切換）
   - 成功後關閉 overlay 並切到新 workspace

### 3.3 取消

- 在 overlay 按 `esc`：
  - 若在 create 模式：丟棄 `workspace name + staged repos` 全部草稿
  - 關閉 overlay，回主畫面

## 4. 介面與鍵位規格

### 4.1 Overlay 版面

- switch 模式：
  - workspace list（無輸入框、無 fzf 候選）
- 右側（create 模式）：
  - workspace name 輸入框
  - staged repos list
- 左側（create 模式）：
  - `scan path` 輸入框（預設值：啟動時 cwd）
  - fzf-like 候選 repo list

### 4.2 鍵位

- `w`：開/關 overlay
- `c`：在 overlay 內進入 create 模式
- `s`：create 模式提交草稿（save）
- `esc`：關閉 overlay；create 模式會丟棄草稿
- `enter`：
  - switch 模式 + workspace list focus：切換 workspace
  - create 模式 + candidate list focus：加入 repo 到 staged list

註：焦點切換與清單游標移動沿用現有 Bubble Tea 慣例（`tab/shift+tab`、方向鍵）實作，不額外引入複雜新鍵組。

## 5. 架構與元件邊界

### 5.1 App State（`internal/app`）

在 `Model` 新增 overlay 子狀態（暫稱 `WorkspaceOverlayState`）：

- `Active bool`
- `Mode`：`switch` / `create`
- `Focus`：`scanPathInput` / `candidateList` / `workspaceList` / `createNameInput` / `stagedRepoList`
- `ScanPathInput`（文字輸入狀態）
- `CreateNameInput`（文字輸入狀態）
- `CandidateQuery`（在候選清單 focus 時的搜尋字串）
- `Candidates []RepoCandidate`
- `StagedRepos []RepoCandidate`
- `SelectedWorkspaceIndex int`
- `SelectedCandidateIndex int`
- `ScanInFlight bool`
- `ScanRevision int`
- `LastError string`

`RepoCandidate` 最小欄位：

- `Name`（顯示名）
- `Path`（絕對路徑）

### 5.2 App 服務抽象

新增 app 層 interface（由 runtime 注入）：

- `WorkspaceOverlayScanner`
  - `ScanRepos(ctx, rootPath) ([]RepoCandidate, error)`
- `WorkspaceDraftCommitter`
  - `CommitCreateAndSwitch(ctx, name string, repos []RepoCandidate) (workspace.State, error)`

目的：

- 讓 overlay 不直接依賴 domain/service 細節
- 維持 app 測試可用 fake/stub 驗證

### 5.3 Runtime 實作（`cmd/tui`）

- `WorkspaceOverlayScanner` runtime 版本：
  - 以輸入路徑作為掃描 root，遞迴找 git repo root 候選
  - 掃描結果在 app 層再依 `CandidateQuery` 做 fuzzy 過濾（fzf-like）
- `WorkspaceDraftCommitter` runtime 版本：
  - 檢查 name 是否重複
  - `CreateWorkspace(name)`
  - 逐一 `AddRepo(newWS.ID, RepoInput)`
  - 設定選中 workspace/repo
  - 載回最新 state 回傳給 app

## 6. 資料流與狀態轉換

### 6.1 即時掃描（debounce + revision）

1. `scan path` 變更 -> 更新 `ScanRevision += 1`
2. 以 debounce（例如 300ms）排程 `MsgOverlayScanRequested{Revision}`
3. command 執行掃描，回 `MsgOverlayScanCompleted{Revision, Candidates, Err}`
4. 僅當 `Revision == current ScanRevision` 才套用結果（避免舊結果覆蓋新輸入）

候選搜尋規則：

- `CandidateQuery` 只影響記憶體中的候選過濾，不重新觸發磁碟掃描

### 6.2 加入 staged repos

- 在 create 模式、focus 在候選清單按 `enter`：
  - 若 path 已存在於 staged repos：不加入，status=`already added`
  - 否則加入 staged repos

### 6.3 提交 `s`

提交前驗證：

- `CreateNameInput` 不可空白
- 名稱不可與現有 workspace 重複

提交步驟：

1. 呼叫 `WorkspaceDraftCommitter.CommitCreateAndSwitch`
2. 成功：
   - 以回傳 state 更新 `Model.State`
   - 設 `UIMode = ModeWorkspace`
   - 若建立時無 repo，僅切換到新 workspace（不選 repo）
   - 關閉 overlay 並清空草稿
3. 失敗：
   - overlay 保持開啟
   - 草稿保留
   - 顯示錯誤訊息

## 7. 錯誤處理

- 掃描根路徑不存在/不可讀：候選清空 + `scan path not accessible`
- 掃描失敗：保留上一版候選，顯示 `scan failed: ...`
- workspace 名稱空白：`workspace name is required`
- workspace 名稱重複：`workspace already exists`
- 提交寫入失敗：草稿保留，不關 overlay，允許修正後重試
- `esc` 永遠可安全離開，且 create 草稿完整丟棄

## 8. 測試策略

### 8.1 `internal/app`

- `w` 開 overlay、`w/esc` 關 overlay
- switch 模式 `enter` 切 workspace
- folder mode 經 switch 後切為 workspace mode
- `c` 進 create 模式
- create 模式 `esc` 丟棄草稿
- debounce + revision：只採用最新掃描結果
- candidate `enter` 加入 staged
- 重複加入 staged 顯示 `already added`
- `s` 成功：更新 state + 關 overlay + 切 workspace mode
- `s` 失敗：overlay 與草稿仍在

### 8.2 `cmd/tui` / runtime

- scanner：合法路徑掃描、非法路徑錯誤、query 過濾結果正確
- committer：重複名稱阻擋；成功建立+加入+切換

### 8.3 回歸

- 現有 folder/workspace launch 行為不退化
- 既有 `a` repo path flow、`[` `]`、sync/worktree/lazygit/diff 測試維持通過

## 9. 風險與控管

- 風險：大目錄即時掃描成本高  
  控管：debounce + 可中止舊掃描 + 限制單次掃描資源上限（例如最大候選數）

- 風險：overlay 鍵位攔截衝突既有主畫面鍵位  
  控管：overlay active 時優先走 overlay key handler

- 風險：提交多步寫入失敗造成部分資料寫入  
  控管：失敗時保留 UI 草稿並明確錯誤提示；後續可加 service 端交易化封裝（不屬本次範圍）

## 10. 驗收條件（DoD）

- 使用者在任一 mode 按 `w` 可打開 overlay
- 可從 workspace list 直接切換
- 可在 create 模式輸入名稱並用左側清單加入 repos
- `s` 可完成「建立 + 加 repo + 切換 + 關閉」
- `s` 在 staged repos 為空時，仍可完成「建立空 workspace + 切換 + 關閉」
- `esc` 可丟棄草稿並關閉
- 重複名稱與重複 repo 都有正確阻擋行為
- 測試覆蓋關鍵互動與失敗路徑
