# Day 08: 自作Scoreプラグインの実装とベンチマーク

## 目的
Day 07で構築したKubernetes環境 + `scheduler-plugins`リポジトリ上に、独自のScoreプラグインを **マルチスケジューラ方式** で実装する。デフォルトスケジューラはそのまま残し、自作スケジューラを2つ目のスケジューラとして並走させる。デフォルトスケジューラとの配置分布の差を数値とグラフで比較し、「Score式がPod配置にどう作用するか」を定量評価する。

## 背景
- Kubernetesのスケジューリングは Filter → Score → NormalizeScore → Bind のフローで進む
- Scoreプラグインは各ノードに `[0, 100]` の整数スコアを付ける
- NormalizeScore で値域が揃わないと、他プラグインとの合算時に偏りが出る
- 本課題ではScore式の設計〜スコア正規化〜実クラスタでの挙動観測までを一気通貫で検証する

### なぜマルチスケジューラ方式か
- 単一スケジューラ方式（default-scheduler を置き換える）では、Score式の設計ミスで CoreDNS / kube-proxy 等のシステムPodが配置されずクラスタ全体が壊れる可能性がある
- マルチスケジューラ方式では default-scheduler に一切手を加えないため、システムPodや既存ワークロードへの影響なし
- 自作スケジューラの影響範囲は `spec.schedulerName: my-scheduler` を明示したPodだけに限定される
- ベンチマーク時も「default vs 自作」を同一クラスタ内で公平に比較できる

---

## タスク一覧

### T1. プラグイン実装
**成果物**: `day08/tasks/scheduler-plugins/pkg/myscheduler/myscheduler.go`

#### 命名規約（重要）
スケジューラまわりには4つの異なるレイヤーの識別子があり、それぞれ役割が異なるため混同しないこと。

| レイヤー | 値 | 使われる場所 |
|---|---|---|
| ① schedulerName | `my-scheduler` | `KubeSchedulerConfiguration.profiles[].schedulerName` / Pod の `spec.schedulerName` |
| ② プラグインName定数 | `GPUGenerationScore` | `plugins.score.enabled[].name` / ログ / メトリクス |
| ③ Goパッケージ名 | `myscheduler` | `pkg/myscheduler/` |
| ④ Go構造体名 | `GPUGenerationScore` | Goソース内の型名 |

- ①は「スケジューラ本体の名前」、②は「そのスケジューラに組み込むプラグインの名前」で別物
- ②〜④は機能を表す名前（`GPUGenerationScore`）で揃えると可読性が高い

要件:
- パッケージ名: `myscheduler`
- 構造体名: `GPUGenerationScore`
- プラグインName定数: `const Name = "GPUGenerationScore"`
- 実装すべきインターフェース:
  - `framework.ScorePlugin`（`Score`, `ScoreExtensions`, `Name`）
  - `framework.ScoreExtensions`（`NormalizeScore`）
- **Score式**:
  - ノードラベル `gpu-generation` の値（例: `v100`=1, `a100`=2, `h100`=3）を世代番号として扱う
  - 世代番号が新しいノードほど高スコア
  - ラベル未設定ノードは `0` 点
- **NormalizeScore**:
  - 全ノードの最大スコアで除算し `[0, 100]` の整数にスケール
  - 全ノード0点の場合は全ノード0のまま返す

### T2. スケジューラビルド & デプロイ（マルチスケジューラ方式）
**成果物**: `day08/tasks/manifests/` 配下のYAML一式

- `scheduler-plugins` の `cmd/scheduler` を流用し、`myscheduler` プラグインを登録
- コンテナイメージをビルド（ローカルレジストリ or `kind load docker-image`）
- **default-scheduler の static Pod (`/etc/kubernetes/manifests/kube-scheduler.yaml`) は触らない**。自作スケジューラは `kube-system` namespace に Deployment として独立に動かす
- マニフェスト一式:
  - **`ConfigMap`** … `KubeSchedulerConfiguration` を格納
    - `profiles[].schedulerName: my-scheduler`（default-scheduler とは別名）
    - `plugins.score.enabled: [{name: GPUGenerationScore}]`
    - `leaderElection.resourceName` も default と被らない名前に変更
  - **`ServiceAccount`** … `kube-system` namespace に作成
  - **`ClusterRole` / `ClusterRoleBinding`** … 既存の `system:kube-scheduler` を bind するか、必要権限のみ持つ Role を新規作成
  - **`Deployment`** … 自作スケジューライメージを起動。`--config=` で ConfigMap をマウント、`--kubeconfig` 不要（in-cluster config）、`--leader-elect-resource-name` を指定
- ノードに擬似ラベルを付与するスクリプト: `day08/tasks/setup/label-nodes.sh`
  - 例: ノード3台に `gpu-generation=v100`, `a100`, `h100` を割り当て

#### 動作確認
- `kubectl get pods -n kube-system` で **default-scheduler と自作 my-scheduler の両方** が Running になっていること
- `kubectl logs -n kube-system <my-scheduler-pod>` でリーダー選出ログが正常に出ていること

### T3. ベンチマーク設計
**成果物**: `day08/benchmark/`

#### T3-1. Pod生成スクリプト
- `benchmark/run.sh` または `benchmark/run.py`
- 同一スペックのPodを10個、指定されたschedulerで生成
- 第1引数でschedulerを切替可能（`default-scheduler` / `my-scheduler`）
- Pod spec:
  - イメージは軽量なもの（`pause` 等）
  - resources.requests は均一
  - **`spec.schedulerName` を必ず明示**（マルチスケジューラ方式では未指定だと default-scheduler が拾うため、`my-scheduler` 側のテストにならない）

#### T3-2. 結果集計スクリプト
- `benchmark/collect.py`
- `kubectl get pod -o json` からPod→Nodeのマッピングを収集
- 集計項目:
  - ノード別Pod数
  - 配置比率（%）
  - 各ノードのgpu-generationラベル値

#### T3-3. グラフ生成
- `benchmark/plot.py`（matplotlib想定）
- 出力: `benchmark/results/distribution.png`
- 内容: デフォルトscheduler vs 自作scheduler の2系列棒グラフ（横軸=ノード名、縦軸=Pod数）

### T4. 計測と再現性
各schedulerにつき **最低3試行** 実施し、平均と標準偏差を出す。
- 試行ごとに namespace を作り直す、または全Pod削除して初期化
- 生データは `benchmark/results/raw/trial_{N}_{scheduler}.json` として保存

### T5. レポート
**成果物**: `day08/report.md`

必須セクション:
1. **実装サマリ**
   - Score式の定義（数式 or 疑似コード）
   - NormalizeScore前後のスコア例（表）
2. **計測結果**
   - 配置分布表（scheduler × ノード）
   - 比較グラフの埋め込み
3. **考察**
   - Score式と実配置の一致/乖離
   - 乖離がある場合の原因仮説（他プラグインとの合算・TaintToleration・リソース不足など）
   - 期待と異なる挙動があれば原因分析
4. **限界と今後**
   - サンプルサイズ・環境の制約
   - 次に検証したい拡張（PreFilter追加、重み調整など）

---

## 受入基準（Definition of Done）
- [ ] `go build ./...` が `scheduler-plugins` 配下で成功する
- [ ] 自作スケジューラPod（Deployment）が `kube-system` で `Running` 状態で起動する
- [ ] **default-scheduler の static Pod が引き続き `Running` のまま**（マルチスケジューラ方式のため壊していないこと）
- [ ] `schedulerName: my-scheduler` のPodが `gpu-generation=h100` ノードに優先配置される（3試行中2回以上）
- [ ] `schedulerName` 未指定のPod（または `default-scheduler` 指定）は default-scheduler に処理されることを確認（`kubectl describe pod` の Events で `Scheduled by default-scheduler` を確認）
- [ ] `benchmark/results/distribution.png` が生成されている
- [ ] `report.md` に数値表とグラフが含まれ、考察が書かれている

---

## ディレクトリ構成（最終形）
```
day08/
├── day08.md                              # 本ファイル
├── report.md                             # 考察レポート
├── tasks/
│   ├── scheduler-plugins/                # クローンしたリポジトリ
│   │   └── pkg/myscheduler/
│   │       ├── myscheduler.go            # Scoreプラグイン本体
│   ├── manifests/
│   │   ├── configmap.yaml                # KubeSchedulerConfiguration を格納
│   │   ├── deployment.yaml               # 自作スケジューラ本体（kube-system）
│   │   └── rbac.yaml                     # ServiceAccount / ClusterRole / ClusterRoleBinding
│   └── setup/
│       └── label-nodes.sh
└── benchmark/
    ├── run.sh | run.py                   # Pod生成
    ├── collect.py                        # 結果集計
    ├── plot.py                           # グラフ生成
    ├── pod-template.yaml
    └── results/
        ├── raw/
        │   ├── trial_1_default.json
        │   ├── trial_1_my.json
        │   └── ...
        ├── summary.csv
        └── distribution.png
```

---

## 進め方の推奨順序
1. T1実装 → `go build` で型チェックのみ通す
2. T1単体テスト（Score関数に `framework.NodeInfo` を渡すテーブル駆動テスト）
3. T2デプロイ → `kubectl apply -f manifests/` で ConfigMap/RBAC/Deployment を一気に投入
4. `kubectl get pods -n kube-system` で **default + my-scheduler の両方** が Running なのを確認
5. シンプルPod1個に `spec.schedulerName: my-scheduler` を付けて動作確認
6. T3スクリプト整備 → 小規模（Pod=3）で試運転
7. T4本番計測（Pod=10 × 3試行 × 2scheduler）
8. T5レポート執筆

## 参考
- Scheduling Framework: https://kubernetes.io/docs/concepts/scheduling-eviction/scheduling-framework/
- scheduler-plugins: https://github.com/kubernetes-sigs/scheduler-plugins
- KubeSchedulerConfiguration v1: https://kubernetes.io/docs/reference/config-api/kube-scheduler-config.v1/
