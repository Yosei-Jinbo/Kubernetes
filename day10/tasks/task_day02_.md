# TopoGang: 3 軸構成で workload aware scheduling を理解する 1 週間プロジェクト (v5)

> LT46 問1-B「Kubernetes workload aware scheduling」を、**手を動かして**理解しきるための計画書 — v5。
> Day 1 (Foundation) は完了済み。Day 2-7 が本計画書のスコープ。

## v5 で何が変わったか

v4 では **Kubernetes 1.35 で実際に kube-scheduler に入った workload aware scheduling 機能 (Workload API + GangScheduling plugin + OpportunisticBatching)** に触れていなかった。問題文「Kubernetes で導入が進んでいる workload aware scheduling」が直接指している機能はこれ。reviewer もこれを期待する。

v5 では **3 軸構成** に組み替え:

- **軸 A: 自作 plugin** (gang + topology) — 自分で書く経験
- **軸 B: v1.35 / v1.36 alpha の Workload API と GangScheduling** — 公式実装を実機で観察し、自作と比較
- **軸 C: Kueue で AI 学習基盤** — JobSet + LeaderWorkerSet + Topology Aware Scheduling で実システム構築

各軸 2 日ずつの計 6 日 (Day 2-7) で展開。Day 7 後半でレポート完成。

### v1 → v2 → v3 → v4 → v5 の段階
- **v1**: 写経改造ベース、Design 判断曖昧
- **v2**: + Design Challenge と Design Doc
- **v3**: + Implementation Phase。ただしコード骨格を出してしまった
- **v4**: コード骨格削除、問い + リンクに置換
- **v5**: 3 軸構成 (A 自作 / B v1.35 公式 / C Kueue 実システム)、Day 1 完了済み前提

---

## 0. プロジェクトの位置付け

### 目的
1. Kubernetes の **Scheduling Framework の拡張点** を、書くことで身体に入れる (軸 A)
2. AI ワークロードの 2 大要件 — gang scheduling と topology awareness — を実装し、「default では足りない」を実例で説明できる (軸 A)
3. **Kubernetes 1.35 / 1.36 で導入が進む公式の workload aware scheduling** を実機で動かし、自作との設計比較を行う (軸 B) — これが LT46 reviewer の期待
4. Kueue で **実運用に近い AI 学習基盤** を構築し、scheduler / queue layer / workload controller の役割分担を体得 (軸 C)
5. これら 3 軸の比較から、in-tree 拡張 / queue layer / scheduler 全置換 / 自作 plugin の **4 戦略を 1 図で語れる** 状態にする
6. 自分の研究背景 (BlueField DPU、MoE routing、SmartNIC) と接続。「ハードウェア特性を scheduler に伝えるとは何か」を語れる
7. 各軸の設計判断を Design Doc として残し、面接で説明可能な記録を作る

### 1 週間で達成可能か
- 達成可能だが **タイト**。Day 1 を完了済みとして Day 2-7 の 6 日。各軸 2 日 + レポート Day 7 後半 4h。
- 詰め込み構成。Day 5 で「v1.35 GangScheduling 実機 + 自作との比較 + v1.36 alpha 観察」、Day 7 で「Kueue LWS/MultiKueue + Volcano + レポート」を集約する。

---

## 1. v5 における Claude (私) のサポート範囲

### やる
- エラー log を見せてもらえば疑うべき箇所の hint
- どのドキュメント / どのソースファイルを読めば判断材料が得られるかリンク
- 観察された動作と期待動作のギャップを整理する手伝い
- Design Doc のレビュー — 論理が破綻していれば指摘
- **ツール類 (observe.py / parse_events.py / run_experiment.sh / plot.py) の skeleton 程度**

### やらない
- Go plugin のコード提示 ("こう書け") は出さない
- 答えに直結する選択肢の列挙
- Self-check questions の模範解答

---

## 2. Design Doc テンプレート

各 plugin / システムについて、実装前に書き、実装後に追記。`docs/design/<topic>.md` として残す。

```markdown
# Design Doc: <Topic>

## 1. Motivation
## 2. Alternatives Considered
## 3. Chosen Design
## 4. Rationale
## 5. Trade-offs and Limitations
## 6. Comparison with Existing Implementation (実装後に追記)
## 7. Future Work
```

ルール: §1〜§5 は既存実装を読む *前* に書く。§6 は読んだ後に書く。

---

## 3. 環境とツールチェーン

### 必須
- Linux または macOS (Apple Silicon でも可)
- Docker, `kind` v0.24+, `kubectl` v1.31+, `helm` v3
- `go` 1.22+
- `make`, `git`, `python3`

### 推奨
- `k9s`, `stern`, `jq`, `yq`

### v5 で追加で必要なもの
- **Kubernetes v1.35 / v1.36 image を持つ kind**: Day 4 / 5 で使う。kind v0.27+ で v1.35 サポート、v1.36 は最新版を要確認
- **JobSet, LeaderWorkerSet (LWS)**: Day 6 / 7 で install
- **Volcano**: Day 7

### リソース見積
- CPU 4+ core, RAM 16+ GB (kind 2 cluster + 自作 scheduler + Kueue + JobSet/LWS controller)
- Disk 40+ GB

### ディレクトリ構成
```
topogang/
├── go.mod
├── cmd/scheduler/main.go
├── pkg/
│   ├── nodemask/                 # Day 2: Filter plugin
│   ├── coscheduling/             # Day 3: gang plugin
│   └── topologyaware/            # Day 5: topology plugin
├── manifests/
│   ├── scheduler-config.yaml
│   ├── deployment.yaml
│   ├── crds/
│   ├── kueue/                    # Day 6
│   └── workload-api/             # Day 4 (v1.35 cluster 用)
├── tools/
│   ├── observe.py                # Day 1 (完了済み)
│   ├── parse_events.py           # Day 5
│   ├── run_experiment.sh         # Day 5
│   └── plot.py                   # Day 7
├── examples/
│   ├── workload-bad-no-gang.yaml
│   ├── workload-good-gang-topogang.yaml
│   ├── workload-good-gang-v135.yaml         # 軸 B
│   ├── workload-jobset-tas.yaml             # 軸 C
│   ├── workload-lws-tas.yaml                # 軸 C
│   └── workload-volcano.yaml
└── docs/
    ├── README.md
    ├── design/                   # Design Doc 群
    │   ├── my-spec.md            # Day 1 完了済み
    │   ├── api-schema.md         # Day 2
    │   ├── gang-scheduling.md    # Day 3
    │   ├── workload-api-comparison.md  # Day 4 (新)
    │   ├── topology-aware.md     # Day 5
    │   ├── ai-platform.md        # Day 6 (新, Kueue 構築の設計)
    │   ├── three-strategies.md   # Day 7
    │   └── ideal-scheduler.md    # Day 7
    ├── predictions/
    │   ├── kueue-behavior.md     # Day 6
    │   └── v135-behavior.md      # Day 4
    └── experiments/
        ├── runs/
        └── plots/
```

---

## 4. 全体像と日程 (詰め込み構成 X)

| Day | 軸 | テーマ | 想定時間 |
|-----|----|--------|---------|
| (1) | — | Foundation (完了済み) | — |
| 2 | A | 自作 scheduler 最小構成 (Noop + Nodemask Filter) + Day 3 設計開始 | 9-10h |
| 3 | A | 自作 gang plugin (Permit + Reserve + PreFilter) | 11-12h |
| 4 | B | v1.35 cluster + Workload API + GangScheduling 実機 + 自作との比較 | 10-11h |
| 5 | B+A | v1.36 alpha 観察 (Workload Scheduling Cycle, KEP-5732) + 自作 topology plugin | 12-13h |
| 6 | C | Kueue で AI 学習基盤 (Topology, ClusterQueue, TAS, JobSet) | 11-12h |
| 7 | C+Vol+執筆 | LWS + MultiKueue + Volcano 比較 + 4 ページ tech note | 12-13h |

合計 ~70 h。週末 2 日を Day 3 / 5 に当てる前提。

各 day の構成:
- **Goal**
- **Why this matters for LT46**
- **Phase 1: 一次資料を読む** (≤ 1h)
- **Phase 2: Design Challenge** (1-2h, 該当日のみ)
- **Phase 3: 実装に入る前に答えるべき問い**
- **Phase 4: 実装中に参照すべき資料**
- **Phase 5: Setup**
- **Phase 6: Implementation** (★ 自分で書く)
- **Phase 7: Verification & Experiments**
- **Pitfalls** (症状ベース)
- **Self-check questions**
- **Stretch goals**

---

## 5. Day-by-Day 計画 (Day 2-7)

---

### Day 2 (軸 A): 自作 scheduler 最小構成 + Day 3 設計開始

#### Goal
- 自作 scheduler binary が container 化されて kind cluster で動いている
- Noop Score plugin + Nodemask Filter plugin が動く
- Deployment, RBAC, ConfigMap を手書き
- API schema 設計が `docs/design/api-schema.md` にある
- Day 3 (gang plugin) の Design Doc §1〜§3 を着手

#### Why this matters for LT46
- レポート §2「Scheduling Framework」を自分の binary で語れる
- Filter plugin の実装経験が、Day 3 の Permit plugin の足掛かり
- Day 3 を実装に集中できるよう、Design Doc を前倒し

#### Phase 1: 一次資料 (45 min)

1. **Scheduler Configuration 公式 docs** — profile, plugin, pluginConfig の関係
   <https://kubernetes.io/docs/reference/scheduling/config/>

2. **Configure Multiple Schedulers**
   <https://kubernetes.io/docs/tasks/extend-kubernetes/configure-multiple-schedulers/>

3. **scheduler-plugins README** — out-of-tree plugin の構造
   <https://github.com/kubernetes-sigs/scheduler-plugins>

4. **`framework` package godoc** — `Plugin`, `FilterPlugin`, `ScorePlugin` interface
   <https://pkg.go.dev/k8s.io/kubernetes/pkg/scheduler/framework>

5. **Lee Bernick "Writing a Kubernetes Scheduler Plugin"** — 体験記、ハマりどころが豊富
   <https://www.leebernick.com/posts/scheduler-plugin/>

#### Phase 2: Design Challenge — API schema 設計 (1h)

`docs/design/api-schema.md` に「ユーザーが gang/topology の要件を伝える方法」を 2-3 案比較。

**考えるべき軸**:
- ユーザーが書く YAML の冗長さ
- Server-side validation のしやすさ
- controller を別途実装する必要性
- elastic workload (動的に Pod 数が変わる) の表現
- heterogeneous PodGroup (異なる resource 要求の Pod を含む) の表現

書き終わったら scheduler-plugins の crd.yaml と末尾で差分を比較:
<https://github.com/kubernetes-sigs/scheduler-plugins/blob/master/manifests/coscheduling/crd.yaml>

**比較も忘れずに**: v1.35 公式 PodGroup API:
<https://kubernetes.io/docs/concepts/workloads/workload-api/policies/>

→ scheduler-plugins と公式 v1.35 で **PodGroup の field 名が違う** (e.g., `minMember` vs `minCount`)。
これを観察して「なぜ違うのか」を考える。Day 4 の比較の素材になる。

#### Phase 3: 実装に入る前に答えるべき問い (40 min)

**問い 1 (Noop Score)**: Score plugin の戻り値は int64。`framework.MaxNodeScore` の値は? 範囲はどう決まっているか?

**問い 2 (Nodemask Filter)**: Filter plugin が「不適格」と返す方法は? `framework.Status` のどの値? `Unschedulable` と `UnschedulableAndUnresolvable` の違いは?

**問い 3 (Plugin Args)**: PluginConfig で渡す `excludeLabel` 値を Go コード側でどう受け取るか? `runtime.Object` の中身はどんな型?

**問い 4 (Multi-extension-point)**: 1 plugin が Filter と Score の両方を実装するには、Go で どう書くか?

**問い 5 (RBAC)**: 自作 scheduler の ServiceAccount は最低限何の権限を持つべきか? `pods/binding` だけ? `events`, `nodes` も?

#### Phase 4: 実装中に参照すべき資料

- **`framework` package godoc**: <https://pkg.go.dev/k8s.io/kubernetes/pkg/scheduler/framework>
- **scheduler-plugins/cmd/scheduler/main.go**: plugin register の書き方
   <https://github.com/kubernetes-sigs/scheduler-plugins/blob/master/cmd/scheduler/main.go>
- **scheduler-plugins/pkg/capacityscheduling**: Args デコードの参考
   <https://github.com/kubernetes-sigs/scheduler-plugins/tree/master/pkg/capacityscheduling>
- **scheduler-plugins/manifests/install/**: RBAC の例
   - RBAC(Role-Based Access Control): role(そのNameSpace内での権限)の管理

#### Phase 5: Setup (1h)

scheduler-plugins を fork & clone:
```bash
git clone https://github.com/kubernetes-sigs/scheduler-plugins.git topogang
cd topogang
git checkout -b topogang-day2
```

#### Phase 6: Implementation (4h) ★

##### Sub-phase 6a: Noop Score plugin (30 min)
最小の Score plugin。`framework.ScorePlugin` を満たし、log に「呼ばれた」を残し、score 0 を返す。

##### Sub-phase 6b: Nodemask Filter plugin (1.5h)
設定で渡される `excludeLabel` を持つ node を Filter で reject。Args struct を自分で定義、`New()` で `runtime.Object` から decode。

##### Sub-phase 6c: Manifests を手書き (1.5h)
- `manifests/scheduler-config.yaml`: Noop / Nodemask を有効化、Nodemask の args を pluginConfig で渡す
- `manifests/deployment.yaml`: ServiceAccount, ClusterRole, ClusterRoleBinding, ConfigMap, Deployment

scheduler-plugins/manifests/install/ を参考にしながら、コピペでなく自分で書く。

##### Sub-phase 6d: cmd/scheduler/main.go 更新

**Lift OK**: `app.WithPlugin(...)` の register 行はボイラープレート。

##### Sub-phase 6e: Build & Deploy (30 min)
```bash
make local-image
kind load docker-image registry.k8s.io/scheduler-plugins/kube-scheduler:local --name topogang
kubectl apply -f manifests/
kubectl logs -n kube-system deploy/topogang-scheduler -f
```

#### Phase 7: Verification — 受け入れ条件

- [ ] テスト Pod (`schedulerName: topogang`) を投げると Noop scoring ログが node 数ぶん出る
- [ ] node に label `do-not-schedule=true` を付け、Pod を投げるとその node には行かない
- [ ] label を外すとその node にも行く
- [ ] Nodemask の Args (excludeLabel) を別の値に変えて再 deploy → 設定が反映される
- [ ] manifests/deployment.yaml の各リソースの役割を 30 秒で説明できる
- [ ] api-schema.md に 2-3 案の比較と採用理由が書かれている

#### Phase 8: Day 3 への前倒し (1.5h)

Day 3 (gang plugin) の **Design Doc §1〜§3 を着手**。これにより Day 3 で実装に集中できる。
- §1 Motivation, §2 Alternatives Considered, §3 Chosen Design (struct level まで) を書く
- §4-§7 は Day 3 で

#### Pitfalls (症状ベース)

- 自作 scheduler の Pod が起動しても何もしない → RBAC を疑う
- `kubectl logs` に何も出ない → ConfigMap mount, scheduler-config.yaml syntax
- Nodemask が常に reject / 常に通す → Args が Go コードに届いていない、`runtime.Object` decode を確認
- Image を kind に load し忘れている → `docker exec topogang-control-plane crictl images` で確認

#### Self-check questions

1. Noop plugin は Pod 1 つに対して何回呼ばれる? なぜ?
2. 各 extension point は scheduling cycle のどの順番で呼ばれるか?
3. 自分が選んだ API schema を選んだ理由を 30 秒で説明できるか?
4. ClusterRole に追加した権限を 1 つ抜いたら何が起きるか?
5. **(v5)** scheduler-plugins と v1.35 公式 PodGroup の field 名が違うのはなぜか? 仮説でいい

#### Stretch goals
- `klog -v 4` でログレベルを上げて scheduling cycle 内部を読む
- Nodemask を「複数 label の OR」に拡張

---

### Day 3 (軸 A): 自作 gang plugin

#### Goal
- gang plugin の Design Doc が `docs/design/gang-scheduling.md` に完成 (§1〜§5 はコード読む前)
- 自分で書いた plugin が動いている (gang admission が all-or-nothing で機能)
- 「4 Pod のうち 3 つだけ schedulable」で **default scheduler は 3 つ走らせるのに、自作は 0 つ** の挙動差を再現
- Design Doc §6 (既存実装との比較) が追記されている

#### Why this matters for LT46
- レポートの中核 (§3)。「私はこう設計し、こう実装した」を Design Doc + git log で説明
- Day 4 で v1.35 GangScheduling と比較するための **比較対象本体** を作る日

#### Phase 1: 一次資料 (1h)

1. **`framework.PermitPlugin` godoc** — Wait/Success/Unschedulable の戻り値、timeout
   <https://pkg.go.dev/k8s.io/kubernetes/pkg/scheduler/framework#PermitPlugin>

2. **`framework.Handle` godoc** — `IterateOverWaitingPods`, `GetWaitingPod`, `SharedInformerFactory`
   <https://pkg.go.dev/k8s.io/kubernetes/pkg/scheduler/framework#Handle>

3. **KEP-624 (Scheduling Framework) §gang-scheduling 例**
   <https://github.com/kubernetes/enhancements/blob/master/keps/sig-scheduling/624-scheduling-framework/README.md>

4. **scheduler-plugins/pkg/coscheduling README** (コードはまだ開かない)
   <https://github.com/kubernetes-sigs/scheduler-plugins/blob/master/pkg/coscheduling/README.md>

5. **scheduler-plugins/kep/42-podgroup-coscheduling** — PodGroup CRD ベースの設計
   <https://github.com/kubernetes-sigs/scheduler-plugins/blob/master/kep/42-podgroup-coscheduling/README.md>

#### Phase 2: Design Challenge — gang plugin Design Doc §4〜§5 (1h)

Day 2 末で §1〜§3 は書いてある前提。今日は **§4 (Rationale) と §5 (Trade-offs)** を書ききる。

書き終えてから scheduler-plugins/pkg/coscheduling のコードを開いて §6 を書く:
<https://github.com/kubernetes-sigs/scheduler-plugins/tree/master/pkg/coscheduling>

#### Phase 3: 実装に入る前に答えるべき問い (1h)

**問い 1 (extension point)**: なぜ Permit を使うのか? Filter で reject ではなぜダメ?

**問い 2 (state location)**: PodGroup の状態をどこに置くべきか? 候補の評価軸:
- 永続性 (scheduler restart で消えるか)
- 読み書き頻度
- 可視性 (kubectl から見えるか)
- 並行 access

**問い 3 (peer の探し方)**: 同 PodGroup の他 Pod の状態を知る方法。`framework.Handle` の API か、client-go Lister か?

**問い 4 (Approve のタイミング)**: quorum 揃った瞬間、自分の Pod は何を返す? 待っている peer はどう Approve する?

**問い 5 (timeout)**: `scheduleTimeoutSeconds` 経過時の振る舞い。Wait の `time.Duration` 戻り値の意味。

**問い 6 (PreFilter の役割)**: scheduler-plugins README の「preFilter is enhanced feature」とは?

**問い 7 (Unreserve)**: Reserve 成功 → Permit Wait 中に別理由で失敗 → スロット解放の責任は誰?

**問い 8 (並行性)**: 同 PodGroup の Pod が並行 cycle に入ったときの整合性。lock の取り方。

これら 8 つに自分で答えてから実装に入る。答えは Design Doc に書き残す。

#### Phase 4: 実装中に参照すべき資料

- **`framework` package godoc**
- **`framework.WaitingPod` godoc** — `Allow()`, `Reject()`
- **scheduler-plugins/pkg/coscheduling/coscheduling.go** — Design Doc §1〜§5 を書き終えた後なら見て良い
- **PodGroup CRD Go types**: scheduler-plugins/apis/scheduling/v1alpha1/types.go
- **client-go Lister**: <https://pkg.go.dev/k8s.io/client-go/listers/core/v1>

#### Phase 5: Setup (45 min)

**Lift OK**: PodGroup CRD YAML
```bash
curl -O https://raw.githubusercontent.com/kubernetes-sigs/scheduler-plugins/master/manifests/coscheduling/crd.yaml
cp crd.yaml manifests/crds/podgroup.yaml
kubectl apply -f manifests/crds/podgroup.yaml
```

**Lift OK**: PodGroup の generated typed client/lister/informer。

#### Phase 6: Implementation (6-7h) ★

実装の流れ (順序提案):

1. PodGroup の in-memory 表現を考えて書く
2. PodGroup registry / manager を考えて書く
3. PreFilter 実装 (PodGroup label 無し Pod は素通り)
4. Reserve / Unreserve 実装
5. Permit 実装 (quorum 判定 + Approve)
6. cmd/scheduler/main.go で plugin register
7. scheduler-config.yaml で各 extension point 有効化

詰まったら Phase 4 のリンクへ。**最初から scheduler-plugins/coscheduling のコードを写経しない**。

**Lift OK**: PodGroup CRD YAML, generated client/lister/informer, plugin register 1 行
**自分で書く**: 上記以外の plugin 本体 (推定 300-500 行)

#### Phase 7: Verification — 受け入れ条件 (抽象的)

- [ ] PodGroup `minMember=4`, capacity 不足時に **0 Pod が Running**
- [ ] capacity 増加 → 4 Pod 同時 Running
- [ ] `scheduleTimeoutSeconds` 経過で Pod が reject される
- [ ] 2 つの異なる PodGroup を同時投入で挙動が破綻しない
- [ ] 並行 cycle で quorum カウントが破綻しない
- [ ] observe.py で「PodGroup の 4 Pod が ±数秒以内に Running」が確認できる
- [ ] Design Doc §6 が埋まっており、scheduler-plugins/coscheduling との設計差が書かれている

#### Pitfalls (症状ベース)

- Pod が永遠に Pending → Permit が呼ばれているか、scheduler-config.yaml で plugin 登録、Reserve が動いているか
- 4 Pod 投げても 1 Pod ずつ Running → quorum 判定が「自分含む」でない可能性
- timeout 後も Wait のまま → Permit Wait の `time.Duration` 戻り値の意味再確認
- ロック関連でハング → `IterateOverWaitingPods` を lock 持ったまま呼んでいないか
- PodGroup label の key — `scheduling.x-k8s.io/pod-group` (新) か古い key か、master を確認

#### Self-check questions

1. なぜ Permit で hold するのが正解で、Filter で hold するのは正解ではないのか?
2. PodGroup の quorum を超える Pod が来たら? overflow の扱い
3. 自分の状態保存場所と scheduler-plugins の選択は同じか? 違うならなぜ?
4. **(v5)** Day 4 で同じ問題を v1.35 公式 GangScheduling で試した時、自分の plugin と何が違いそうか? 仮説を立てておく

#### Stretch goals
- Backoff の自前実装 — quorum 揃わない PodGroup を全 Pod まとめて backoff queue
- Eviction Controller — running PodGroup の 1 Pod が落ちたら全員再起動

---

### Day 4 (軸 B): v1.35 cluster で Workload API + GangScheduling を実機で試す + 自作との比較

#### Goal
- **kind v1.35 cluster** が立っていて、`GenericWorkload` と `GangScheduling` feature gate が有効
- `Workload` リソース と `PodGroup` リソース (v1.35 公式) を実機で動かしている
- Day 3 で書いた自作 plugin と **同じワークロードで挙動比較**したノートがある
- `docs/design/workload-api-comparison.md` が完成

#### Why this matters for LT46
- **問題文「Kubernetes で導入が進んでいる workload aware scheduling」が直接指す機能を、実機で動かした経験を持つ**
- レポート §3 で「自作はこう、公式 v1.35 はこう、設計判断はここが違った」と書ける
- 自作と公式の比較こそが「ちゃんと考えた」証拠

#### Phase 1: 一次資料 (1h)

1. **Kubernetes 公式 v1.35 Blog "Introducing Workload Aware Scheduling"** (Day 1 で読んだが再読)
   <https://kubernetes.io/blog/2025/12/29/kubernetes-v1-35-introducing-workload-aware-scheduling/>

2. **公式 docs: Workload API**
   <https://kubernetes.io/docs/concepts/workloads/workload-api/>

3. **公式 docs: PodGroup Scheduling Policies**
   <https://kubernetes.io/docs/concepts/workloads/workload-api/policies/>

4. **公式 docs: Gang Scheduling**
   <https://kubernetes.io/docs/concepts/scheduling-eviction/gang-scheduling/>

5. **KEP-4671: Gang Scheduling using Workload Object** §Proposal を **今度こそ全部読む**
   <https://github.com/kubernetes/enhancements/blob/master/keps/sig-scheduling/4671-gang-scheduling/README.md>

6. **Feature Gates docs** (`GenericWorkload`, `GangScheduling`, `OpportunisticBatching`)
   <https://kubernetes.io/docs/reference/command-line-tools-reference/feature-gates/>

7. **v1.35 changelog** — どの PR で何が入ったか
   <https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG/CHANGELOG-1.35.md>

#### Phase 2: Design Challenge — v1.35 挙動の事前予測 (1h)

`docs/predictions/v135-behavior.md` に install 前に予測を書く。

**シナリオ** (各々 ① 予測、② 自作 plugin との挙動差の予測、③ 後で実機結果と振り返り):

1. 4 Pod の Workload (gang minCount=4) で capacity 4 Pod 分 → どう動く?
2. 4 Pod gang minCount=4 で capacity 3 Pod 分 → どう動く? **自作と挙動差ある?**
3. 5 Pod 投げて minCount=4 → 5 Pod 全部走る? overflow はどう扱う?
4. PodGroup を 2 つ並行 → 互いに干渉する? しない?
5. OpportunisticBatching が「同型 Pod」をどう扱うか観察
6. **Design Doc §6 で予想した「自作 vs 公式の設計差」** が挙動として観察できるか

書き終えてから KEP-4671 §Proposal を読む。**読んだ後に予測を上書きしないこと** (= 自分の理解度の素直な記録になる)。

#### Phase 3: 実装に入る前に答えるべき問い (45 min)

**問い 1 (kind config)**: kind cluster で Kubernetes v1.35 を使い、kube-apiserver と kube-scheduler の両方に feature gate を渡すには? kind config の `kubeadmConfigPatches` の使い方は?
   - kind docs: <https://kind.sigs.k8s.io/docs/user/configuration/#feature-gates>

**問い 2 (alpha API group)**: `scheduling.k8s.io/v1alpha1` API group は kind v1.35 image でデフォルトで有効か? `--runtime-config` の指定が要るか?

**問い 3 (Workload と PodGroup の関係)**: 公式 docs を読んで、`Workload` resource と `PodGroup` resource はどう違うか? どちらに `schedulingPolicy.gang.minCount` を書く? Pod は `workloadRef` (v1.35) か `schedulingGroup` (v1.36 alpha) で何を参照?

**問い 4 (PreEnqueue vs Permit)**: 公式 GangScheduling plugin は **PreEnqueue + Permit** を使っている。自作は **PreFilter + Reserve + Permit**。なぜ公式は PreEnqueue を選んだか? PreEnqueue extension point の意味と利点は?
   - PreEnqueue docs: <https://kubernetes.io/docs/concepts/scheduling-eviction/scheduling-framework/>

**問い 5 (比較計測)**: 自作と v1.35 公式で「同じワークロード」を投げる時、何を測ると比較になる? scheduling latency / locality / overflow 挙動 / livelock 耐性 / etc.

#### Phase 4: 実装中に参照すべき資料

- **kind feature gates 設定**: <https://kind.sigs.k8s.io/docs/user/configuration/#feature-gates>
- **kind kubeadm patches**: <https://kind.sigs.k8s.io/docs/user/configuration/#kubeadm-config-patches>
- **kind release ノート** (v1.35 サポートバージョン確認): <https://github.com/kubernetes-sigs/kind/releases>
- **v1.35 Workload API の YAML 例** (公式 docs と Kubesimplify ブログに豊富)
   <https://blog.kubesimplify.com/kubernetes-v135-whats-new-whats-changing-and-what-you-should-know>

#### Phase 5: Setup — kind v1.35 cluster + feature gates (2h)

新しい kind cluster を立てる (Day 1 cluster とは別)。既存 cluster を delete するか、別名で作る:

```bash
# kind config (例の skeleton 程度): featureGates と runtimeConfig を kubeadm patch で渡す
# 詳細は kind docs を参照
kind create cluster --name topogang-v135 --config kind-config-v135.yaml \
  --image kindest/node:v1.35.x  # 最新の patch version を確認
```

**自分で書く** (skeleton は kind docs から):
- `kind-config-v135.yaml`: featureGates の設定、API group enable

**Lift OK**: kind 公式の feature gate 設定例

その後:

1. Day 1 で立てた node label と GPU capacity patch を **新 cluster でも適用**
2. CRD は v1.35 image に標準で含まれているはず (Workload, PodGroup は scheduling.k8s.io/v1alpha1)
3. `kubectl get crd` で `workloads.scheduling.k8s.io` `podgroups.scheduling.k8s.io` を確認

#### Phase 6: 実機検証 (4-5h)

##### Sub-phase 6a: Hello world Workload (1h)

`examples/workload-good-gang-v135.yaml` を自分で書く:
- `Workload` resource (apiVersion: scheduling.k8s.io/v1alpha1)
- 中で `podGroups` に `policy: gang: minCount: 4` を指定
- Pod を 4 つ作る (Job でも plain Pod でも可)
- 各 Pod の spec で `workloadRef` を参照

apply して挙動観察。Day 3 と同じ kind cluster ではなく、新しい v1.35 cluster で。

```bash
kubectl --context kind-topogang-v135 apply -f examples/workload-good-gang-v135.yaml
kubectl --context kind-topogang-v135 get workload,podgroup,pods -w
```

##### Sub-phase 6b: 5 シナリオ実機検証 (2h)

Phase 2 で書いた predictions/v135-behavior.md の 5 シナリオを実行。
- 各シナリオの実機結果を記録
- 予測との差分を記録
- observe.py を v1.35 cluster でも動かす (selector が同じなら使い回し可)

##### Sub-phase 6c: 自作との挙動比較 (1h)

同じ 4 Pod gang ワークロードを 2 つの cluster で投げて比較:
- kind-topogang-day3 (自作 scheduler)
- kind-topogang-v135 (v1.35 公式 GangScheduling)

比較項目:
- 4 Pod が Running になるまでの時間 (observe.py で計測)
- 3 Pod 分しか capacity が無い時の挙動
- 5 Pod 投げた時の overflow 挙動
- timeout 動作

##### Sub-phase 6d: Design Doc 作成 (1h)

`docs/design/workload-api-comparison.md` に:

1. v1.35 公式 GangScheduling の設計概要 (PreEnqueue + Permit)
2. 自作 plugin の設計 (PreFilter + Reserve + Permit)
3. 設計差の表
4. 実機で観察した挙動差
5. なぜ公式は PreEnqueue を選んだか自分の理解
6. 自作の弱点と公式の弱点をそれぞれ

#### Phase 7: Verification — 受け入れ条件

- [ ] kind v1.35 cluster で Workload + PodGroup が `kubectl get` で見える
- [ ] 4 Pod gang ワークロードで全 Pod が Running になる
- [ ] capacity 不足で全 Pod が Pending のまま (gang が効いている)
- [ ] timeout 経過で reject される
- [ ] `docs/predictions/v135-behavior.md` の 5 シナリオに実機結果と振り返りが書かれている
- [ ] `docs/design/workload-api-comparison.md` に自作と公式の設計差が書かれている
- [ ] Day 3 cluster と Day 4 cluster で同じワークロードを比較計測した結果がある

#### Pitfalls (症状ベース)

- `kubectl apply -f workload.yaml` で `no kind "Workload"` エラー → API group が enabled になっていない、`runtime-config` 確認
- Workload を作っても何も起きない → `GenericWorkload` feature gate が kube-apiserver と kube-scheduler の **両方で** 有効か
- Pod が `workloadRef` を持っていても gang が効かない → `GangScheduling` feature gate が kube-scheduler で有効か、Workload の `schedulingPolicy.gang` が指定されているか
- PreEnqueue で永遠に待たされる → minCount 個の Pod がまだ作成されていない、これは PreEnqueue の正しい挙動
- kind v1.35 image が pull できない → kind release で対応 image tag を確認

#### Self-check questions

1. v1.35 公式 GangScheduling は PreEnqueue + Permit。なぜ PreEnqueue が要るか? PreFilter で代用できないか?
2. 自作 plugin と公式の挙動差で、自分が一番驚いたのは?
3. `Workload` resource と `PodGroup` resource を分離した設計意図は何か? KEP-4671 の v1.36 改訂 (workloadRef → schedulingGroup) はなぜ?
4. OpportunisticBatching は何を最適化するか? 自作にも応用できるか?
5. **(v5)** 「自作 plugin はもう要らないのでは?」と聞かれたら、何と答えるか? (= 自作の存在意義をどう正当化するか)

#### Stretch goals
- OpportunisticBatching を OFF にして同じワークロードを試し、ON との挙動差を観察
- v1.35 公式 GangScheduling のソースコードを読む: kubernetes/kubernetes/pkg/scheduler/framework/plugins/gangscheduling/

---

### Day 5 (軸 B + A): v1.36 alpha 観察 + 自作 topology plugin

#### Goal
- v1.36 alpha の方向性を理解した: Workload Scheduling Cycle, KEP-5732 (Topology-Aware Workload Scheduling), KEP-5710 (Workload-aware Preemption)
- 自作 topology plugin の Design Doc が完成 (3 案以上比較)
- 自分で書いた Score plugin が動く
- 4 Pod の gang Job で複数試行で同 rack 寄せが観測される (default scheduler との比較で有意差)

#### Why this matters for LT46
- KEP-5732 が **あさんの研究 (BlueField + MoE inter-node) と直結する KEP**
- レポート §5 (ハードウェア接続) で Kubernetes の topology-aware 進化方向を語れる
- 自作 topology plugin は §3 のもう一つの主役

#### Phase 1: 一次資料 (1h)

##### v1.36 alpha 関連
1. **KEP-5710: Workload-aware Preemption**
   <https://github.com/kubernetes/enhancements/blob/master/keps/sig-scheduling/5710-workload-aware-preemption/README.md>

2. **KEP-5732: Topology-Aware Workload Scheduling**
   <https://github.com/kubernetes/enhancements/tree/master/keps/sig-scheduling/5732-topology-aware-workload-scheduling>
   - **これがあさんの研究と直結**

3. **KEP-4671 §v1.36 revisions** (Workload Scheduling Cycle, schedulingGroup への rename)
   <https://github.com/kubernetes/enhancements/blob/master/keps/sig-scheduling/4671-gang-scheduling/README.md>

4. **Alpha features in Kubernetes 1.36**
   <https://platformengineering.org/blog/alpha-features-in-kubernetes-1-36>

##### 自作 topology plugin 用
5. **Score plugin godoc** — `Score`, `NormalizeScore`, `ScoreExtensions`
   <https://pkg.go.dev/k8s.io/kubernetes/pkg/scheduler/framework#ScorePlugin>

6. **scheduler-plugins/pkg/networkaware** README — Design Challenge 後に
   <https://github.com/kubernetes-sigs/scheduler-plugins/tree/master/pkg/networkaware>

7. **NVIDIA DRA Driver - ComputeDomain** (概念のみ)
   <https://github.com/NVIDIA/k8s-dra-driver-gpu>

#### Phase 2: Design Challenge — scoring 関数 3 案比較 (1.5h)

`docs/design/topology-aware.md` をテンプレートに沿って書く。**最低 3 案**:

**§2 Alternatives**:
- 候補 A: 同 rack peer 数 (素朴)
- 候補 B: peer 数 / rack 残容量
- 候補 C: leader pinning + follower attraction
- 候補 D: 階層的 (block > rack > host)

**考えるべき軸**:
- 計算量
- 最初の Pod (peer 0) の扱い
- rack 容量を超えた要求での振る舞い
- 並行 race condition
- 階層的 topology への拡張性

**ヒント** (これは答えではなく問題の難しさの指摘): 単純に同 rack peer 数を数えるだけだと、最初の Pod の score が全 node 0 でランダム配置される。これをどう解くかが設計の核。

#### Phase 3: 実装に入る前に答えるべき問い (45 min)

**問い 1**: Score plugin の戻り値 int64 は、別 plugin (NodeResourcesFit) と weighted sum で dominant にならないようにするには?

**問い 2**: `NormalizeScore` の役割は? `ScoreExtensions()` が nil なら呼ばれない、これは何を意味する?

**問い 3 (peer の発見)**: 同 PodGroup の Pod が **どの node に居るか** を知るには、どんな lister / informer を使うか? Day 3 の gang plugin と同じ仕組みを使い回せるか?

**問い 4 (rack の発見)**: 候補 node の rack 情報は、Score の中でどう取得する? `framework.Handle.SnapshotSharedLister().NodeInfos()` か、client-go Node lister か?

**問い 5 (Score plugin の負荷)**: 1000 node で PodGroup 64 Pod を相互参照したら計算量は? 最適化余地?

**問い 6 (KEP-5732 比較)**: KEP-5732 は「Workload 単位で Placements (候補ドメイン) を評価する」設計。自分の plugin の「Pod ごとに per-node score」設計と何が根本的に違うか? どちらが効率的? どちらが正確?

#### Phase 4: 実装中に参照すべき資料

- **`framework.SnapshotSharedLister`** godoc
- **client-go Node Lister / Pod Lister**
- **scheduler-plugins/pkg/networkaware** — 帯域・遅延考慮の実装 (Design Challenge 後)
- **Day 3 の自作 coscheduling plugin** — peer Pod の探し方は同じ informer で良いか確認

#### Phase 5: Setup (30 min)
node label が Day 1 から残っているはず。確認のみ。

#### Phase 6: Implementation — 自作 topology plugin (5-6h) ★

実装の流れ:

1. Score plugin の skeleton (Score / NormalizeScore / ScoreExtensions / New)
2. rack label と peer 情報の取得 helper
3. 採用した scoring 案 (Design Doc §3) を実装
4. NormalizeScore で 0-100 範囲スケール
5. scheduler-config.yaml に Score 追加 (gang plugin と同じ profile)

詰まったら Phase 4 のリンクへ。

**Lift OK**: なし
**自分で書く**: 推定 200-400 行

#### Phase 7: Verification — 受け入れ条件

- [ ] 4 Pod gang Job を投げると、複数試行で「全 Pod が rack=A に揃う」が default scheduler より有意に多い
- [ ] default scheduler (control 実験) で同じ Job を投げると 3+ Pod が rack=A 寄せにならない
- [ ] エッジケース 1: 同時に 2 PodGroup でも破綻しない
- [ ] エッジケース 2: rack=A 容量 3 GPU 制限の挙動が Design Doc §5 の予想通り
- [ ] エッジケース 3: 最初の Pod (peer 0) が想定 node に行く
- [ ] observe.py の rack_distribution が期待通り
- [ ] Design Doc §6 が埋まっており、scheduler-plugins/networkaware との設計差が書かれている

#### Phase 8: v1.36 alpha 観察 (2h)

KEP-5732 は **alpha 未実装** (本計画書執筆時点で kubernetes/kubernetes に PR は未マージの可能性高)。
ただし KEP の design を **読んで** 理解することは可能。

タスク:
1. KEP-5732 の §Proposal を読み、「Workload-oriented scheduling lifecycle」と「Placements」の概念を整理
2. 自分の topology plugin の設計と比較 (Pod ごと vs Workload 全体)
3. `docs/design/workload-api-comparison.md` (Day 4) に追記:
   - v1.35: gang のみ in-tree、topology は scheduler-plugins / Kueue
   - v1.36 alpha: Workload Scheduling Cycle で複数 Pod を 1 batch 処理、KEP-5732 で Workload 全体 placement 評価
4. KEP-5710 (workload-aware preemption) も読み、「自作 plugin で preemption は扱っていない」を Design Doc に明記

##### Stretch (やれたら) — v1.36 master を build

Kubernetes master ブランチを build して KEP-5732 のテスト実装を試す。
リスクが高いので scope 外推奨だが、「触ってみた」だけでもレポートに書ける。
- master からの kind image 作成: <https://kind.sigs.k8s.io/docs/user/quick-start/#building-images>

#### Pitfalls (症状ベース)

- Score plugin が呼ばれない → scheduler-config.yaml の `score.enabled`、weight 0
- 全 node の score が同じ → leader 検出 / peer 集計が空、log で trace
- score 出ているが rack 寄せ起きない → 他 plugin の score が dominant、NormalizeScore 範囲確認
- 並行で複数「leader」立候補 → 決定論的な leader 選出基準を再考
- rack label 無し node でクラッシュ → fallback path、nil チェック

#### Self-check questions

1. Score plugin で「rack を寄せる」のと、Filter plugin で「rack=A 以外を弾く」の違いは?
2. 自分の scoring 関数は、PodGroup 内の最初の Pod (peer 0) でどう振る舞うか?
3. 階層的 topology を表現したい場合、どこに手を入れるか?
4. **(v5)** KEP-5732 の Workload-level placement 評価と、自分の per-Pod scoring の違いを 1 分で説明できるか?
5. **(v5)** あさんの研究 (BlueField + MoE) は KEP-5732 のどの部分と接続するか? (placement 層 vs flow steering 層)

#### Stretch goals
- Hierarchical topology — block, rack, host の 3 階層
- Bandwidth-aware — 想定通信量を annotation から読み配置に反映
- v1.36 master の KEP-5732 テスト実装を実機で試す (scope 外、やれたら)

---

### Day 6 (軸 C): Kueue で AI 学習基盤 — ClusterQueue + TAS + JobSet

#### Goal
- Kueue が install され、本格的な AI 学習基盤の前半が動いている:
  - **Topology** CRD で rack 階層を表現
  - **ResourceFlavor** で rack ごとに分離
  - **ClusterQueue** で 複数チームの quota
  - **JobSet** で multi-Job training を 1 unit に
  - **Topology Aware Scheduling (TAS)** で rack 局所性
- 挙動の事前予測ファイルがあり、検証で当たり/外れを分析
- 計測 harness (`tools/parse_events.py`, `tools/run_experiment.sh`) を自分で書いた

#### Why this matters for LT46
- レポート §4 で「scheduler 内で解く」vs「queue layer で解く」の差を実機で説明
- AI 学習基盤として **Kueue が要求される機能の総体** を体得
- 計測 harness が Day 7 のすべての実験を支える

#### Phase 1: 一次資料 (1.5h)

1. **Kueue Overview**
   <https://kueue.sigs.k8s.io/docs/overview/>

2. **Kueue Concepts** (Workload, ClusterQueue, LocalQueue, Cohort, ResourceFlavor)
   <https://kueue.sigs.k8s.io/docs/concepts/>

3. **Kueue Topology** (Topology CRD)
   <https://kueue.sigs.k8s.io/docs/concepts/topology/>

4. **Kueue Topology Aware Scheduling**
   <https://kueue.sigs.k8s.io/docs/concepts/topology_aware_scheduling/>

5. **Kueue GitHub README** — 最新 release 確認
   <https://github.com/kubernetes-sigs/kueue>

6. **Kueue for AI: The Power of Atomic Admission & Topology Awareness** (Medium, 2026)
   <https://medium.com/google-cloud/kueue-for-ai-the-power-of-atomic-admission-topology-awareness-59c2fd1f86ed>

7. **JobSet README**
   <https://github.com/kubernetes-sigs/jobset>

8. **Kueue + JobSet integration**
   <https://kueue.sigs.k8s.io/docs/tasks/run/jobsets/>

#### Phase 2: Design Challenge — AI 学習基盤の設計 (1.5h)

`docs/design/ai-platform.md` を書く。

**§1 Motivation**: 想定する組織と要件
- 例: 3 チーム (TeamA / TeamB / Researcher) が共有
- TeamA は LLM training (重い、長時間)、TeamB は RAG fine-tuning、Researcher は実験的小規模 job
- ハードウェア: 4 rack、各 rack に node 2-3 台、混在 GPU
- 求める性質: 公平性、TAS で training の rack 局所性、preemption で公平調整

**§2 Alternatives Considered**: どう Kueue を構成するか
- 1 ClusterQueue で全チーム共有 (シンプル、公平調整は Fair Sharing)
- 3 ClusterQueue + Cohort で borrowing (チーム別 quota)
- ResourceFlavor の切り方 (rack 別 / GPU 型別 / 両方)

**§3 Chosen Design**: 採用構成
- どんな Topology / ResourceFlavor / ClusterQueue / LocalQueue を作るか
- どこに preemption や Fair Sharing を効かせるか
- TAS の `podset-required-topology` と `podset-preferred-topology` をどう使い分けるか

**§4-§5**: Rationale, Trade-offs

#### Phase 3: 実装に入る前に答えるべき問い (45 min)

**問い 1**: ClusterQueue の `nominalQuota` を計算する基準。実 hardware capacity と一致させる? 余裕を持たせる? Borrowing 前提で過小に設定する?

**問い 2**: ResourceFlavor を rack ごとに切るか、GPU 型ごとに切るか、両方か? それぞれ何が嬉しい?

**問い 3 (TAS の二段階)**: Kueue TAS は「topology constraint をもつ Workload を admit する時に topology assignment を計算」する。これは自作 topology plugin の per-Pod scoring と何が違うか? 計算量は? 失敗時の振る舞いは?

**問い 4 (gang via Kueue)**: Kueue 自体には gang plugin は無い。Job suspend → admit → unsuspend の流れで「全 Pod が同時に admission される」を保証する。これは自作 Permit と挙動が違うはず。どう違うか予測。

**問い 5 (JobSet を使う理由)**: 単純な Job ではなく JobSet を使う理由。multi-Job training (例: leader + worker, parameter server + workers) を 1 unit で扱うとは具体的にどういう挙動か。

#### Phase 4: 実装中に参照すべき資料

- **Kueue install ガイド** (最新 release)
- **JobSet クイックスタート**
- **Kueue + TAS + JobSet 例**: Kueue docs に複合例あり
- **GKE TAS guide** (実運用例として参考)
   <https://docs.cloud.google.com/ai-hypercomputer/docs/workloads/schedule-gke-workloads-tas>
- jq syntax: <https://jqlang.github.io/jq/manual/>

#### Phase 5: Setup — Kueue + JobSet install (1.5h)

##### Sub-phase 5a: Day 1 の cluster (Day 3 で gang plugin を入れたもの) を使う
**注意**: Day 4 の v1.35 cluster ではなく、Day 1-3 の cluster を使う。Kueue は Kubernetes v1.29+ ならどれでも動く。

##### Sub-phase 5b: Kueue install
**Lift OK**: 公式 install マニフェスト
```bash
# Kueue 公式 release ページで最新 version を確認 (v0.17.x など)
VERSION=v0.17.1  # 実行前に最新を確認
kubectl apply --server-side -f \
  "https://github.com/kubernetes-sigs/kueue/releases/download/${VERSION}/manifests.yaml"
```

##### Sub-phase 5c: JobSet install
```bash
# 最新 release を確認
JOBSET_VERSION=v0.7.0
kubectl apply --server-side -f \
  "https://github.com/kubernetes-sigs/jobset/releases/download/${JOBSET_VERSION}/manifests.yaml"
```

##### Sub-phase 5d: TAS feature gate 確認
Kueue v0.17 では TAS は default で有効化済み (beta)。Kueue Deployment の args を確認。

#### Phase 6: Implementation (5-6h) ★

##### Sub-phase 6a: AI 学習基盤の Kueue 設定 (3h)

**自分で書く**:
- `manifests/kueue/topology.yaml` (Topology CRD: zone > block > rack > host の階層、kind の node label に合わせる)
- `manifests/kueue/resource-flavors.yaml` (rack 別 flavor、必要なら GPU 型別)
- `manifests/kueue/cluster-queues.yaml` (3 チーム分、Cohort で借り合い)
- `manifests/kueue/local-queues.yaml` (チーム別 namespace の queue)
- 各 ClusterQueue の preemption policy (LowerPriority, LowerOrNewerEqualPriority, etc.)

##### Sub-phase 6b: JobSet workload (1.5h)

`examples/workload-jobset-tas.yaml`:
- JobSet で leader (1 Pod) + workers (3 Pod) の 2 ReplicatedJob
- `kueue.x-k8s.io/queue-name` label で LocalQueue 参照
- TAS annotation: `kueue.x-k8s.io/podset-required-topology` または `podset-preferred-topology`
- 各 Pod が rack に揃うよう要求

##### Sub-phase 6c: 計測 harness を自分で書く (1.5h)

ツール類なので skeleton 程度の OK レベル。中身は自分で詰める。

`tools/parse_events.py` (自分で書く):
```python
"""kubectl get events と get pods を combine して PodGroup の timeline を CSV で出力"""
import subprocess, json, sys, csv
# 関数を自分で書く

def fetch_events(namespace="default"):
    pass  # subprocess で kubectl get events

def fetch_pods(label_selector, namespace="default"):
    pass

def extract_timeline(pods, events):
    pass  # 各 Pod について Scheduled / Started などの event timestamp を抽出

def main():
    pass

if __name__ == "__main__":
    main()
```

`tools/run_experiment.sh` (自分で書く):
```bash
#!/bin/bash
# 4 scheduler × N trial で同じワークロードを実行
set -euo pipefail
N_TRIALS=${1:-5}
RESULTS_DIR=docs/experiments/runs/$(date +%Y%m%d-%H%M%S)
mkdir -p "${RESULTS_DIR}"

run_one_trial() {
    local scheduler=$1
    local trial=$2
    local manifest=$3
    # 既存 Job 削除 → apply → 全 Pod Running まで待つ → observe.py 呼ぶ → CSV 追記
    pass  # 中身は自分で
}

# 各 scheduler × trial をループ
```

#### Phase 7: Verification — 受け入れ条件

- [ ] `kubectl get topology,resourceflavor,clusterqueue,localqueue` で全リソースが見える
- [ ] JobSet を投げると Kueue が admit するまで suspended、admit 後に Pod が作られる
- [ ] TAS が効いて全 Pod が同 rack に揃う (`kubectl get pods -o wide` で確認)
- [ ] ClusterQueue の `nominalQuota` を超える要求は admission queue で待つ
- [ ] Cohort 経由で空いている ClusterQueue から borrowing が効く (テストシナリオを作る)
- [ ] `bash tools/run_experiment.sh 3` で 3 trial × 3 scheduler が走る
- [ ] `docs/design/ai-platform.md` の §6 にKueue 公式例 (Medium 記事 etc) との比較が書かれている

#### Pitfalls (症状ベース)

- Job が即座に走り出す → Kueue 用 Job は `suspend: true` 必要、JobSet なら自動付与のはず
- Workload が `Pending`/`PodsReady=False` のまま → ResourceFlavor の `nodeLabels` が node の実 label と一致するか確認
- TAS が効かず Pod が分散する → Pod template の annotation が正しく `kueue.x-k8s.io/podset-required-topology` になっているか
- Topology CRD の `levels` 順序を間違える → 高層 (zone) から低層 (host) の順
- Borrowing が効かない → Cohort name が一致するか、`borrowingLimit` 設定確認

#### Self-check questions

1. Kueue が「scheduler を置き換えない」とはどういう意味か?
2. ClusterQueue の `nominalQuota` を超える要求が来たら何が起きるか?
3. TAS の topology assignment 計算は scheduler の Score とどう違うか?
4. JobSet vs 単純な Job vs Volcano vcjob の違いを 30 秒で説明できるか?
5. **(v5)** Kueue + JobSet + TAS で構築した基盤で、自作 plugin が出る幕はあるか? 何のために?

#### Stretch goals
- Fair Sharing を有効化、2 LocalQueue から並行 submit
- KueueViz で UI 観察: <https://kueue.sigs.k8s.io/docs/tasks/manage/visibility/>
- Kueue admission check (custom AdmissionCheck) を試す

---

### Day 7 (軸 C + Volcano + 執筆): LWS + MultiKueue + Volcano 比較 + レポート

#### Goal
- LeaderWorkerSet (LWS) で AI inference workload を Kueue + TAS で動かしている
- MultiKueue dispatcher 構成を試した (or 観察)
- Volcano が install され vcjob を 1 つ動かした
- 4 戦略 (in-tree v1.35 / 自作 / Kueue / Volcano) の比較表が `docs/design/three-strategies.md` に
- `docs/design/ideal-scheduler.md` 完成
- `tools/plot.py` で比較図 3 枚以上
- **4 ページ tech note 完成**

#### Why this matters for LT46
- LWS は inference 系の API。あさんの MoE 推論研究と直結
- Volcano は 4 戦略目として比較に必要
- レポートを書ききらないと意味が無い

#### Phase 1: 一次資料 (1h)

1. **LeaderWorkerSet (LWS) docs**
   <https://lws.sigs.k8s.io/>

2. **Topology Aware Scheduling with Kueue + LWS** (vLLM 例)
   <https://lws.sigs.k8s.io/docs/examples/tas/>

3. **Kueue MultiKueue**
   <https://kueue.sigs.k8s.io/docs/concepts/multikueue/>

4. **Volcano README + Architecture**
   <https://github.com/volcano-sh/volcano>
   <https://volcano.sh/en/docs/architecture/>

5. **KubeCon EU 2025 比較プレゼン** (YuniKorn vs Volcano vs Kueue, スライドが公開されていれば)

#### Phase 2: Design Challenge — 自分の "理想 scheduler" 像 (1h)

`docs/design/ideal-scheduler.md` を書く。

最重要 §6: あさんの研究 (BlueField + 動的 learned index による MoE routing) と Kubernetes scheduler の階層分担。
- scheduler が担当する placement 層
- DPU が担当する flow steering 層
- 両者の API 境界
- KEP-5732 がカバーする層 / カバーしない層
- Kueue TAS がカバーする層 / カバーしない層

#### Phase 3: 実装に入る前に答えるべき問い (30 min)

**問い 1 (LWS の役割)**: LWS が leader / worker pattern を扱うのは、JobSet と何が違うか? なぜ inference には LWS、training には JobSet が向くか?

**問い 2 (MultiKueue の使い所)**: Multi cluster dispatch で何が嬉しい? 1 cluster でやれない事は?

**問い 3 (Volcano の独自性)**: Volcano が kube-scheduler を **置き換える** 理由。in-tree GangScheduling (v1.35) が出た今、Volcano の存在意義は?

**問い 4 (4 戦略の表)**: in-tree v1.35 / 自作 / Kueue / Volcano の比較表で、どんな軸が浮き彫りになるか? 軸候補:
   - 「scheduler を置き換えるか」
   - 「gang を実装する場所」
   - 「topology の扱い」
   - 「fair sharing / preemption」
   - 「multi-cluster」
   - 「コミュニティ・成熟度」

**問い 5 (plot.py の図 3 枚)**: レポートに載せるなら何の 3 枚?

#### Phase 4: 実装中に参照すべき資料

- LWS quickstart
- MultiKueue tutorial
- Volcano quickstart
- matplotlib gallery (box plot, bar plot)

#### Phase 5: Setup (1.5h)

##### Sub-phase 5a: LWS install (Day 6 cluster で続けて)
```bash
LWS_VERSION=v0.6.0  # 最新確認
kubectl apply --server-side -f \
  "https://github.com/kubernetes-sigs/lws/releases/download/${LWS_VERSION}/manifests.yaml"
```

##### Sub-phase 5b: Volcano install
```bash
VOLCANO=v1.10.0  # 最新確認
kubectl apply -f \
  "https://raw.githubusercontent.com/volcano-sh/volcano/${VOLCANO}/installer/volcano-development.yaml"
```

##### Sub-phase 5c: MultiKueue (任意 stretch)
2 つ目の kind cluster を立てて MultiKueue dispatcher 設定。重いので時間あれば。

#### Phase 6: Implementation (4h) ★

##### Sub-phase 6a: LWS workload + Kueue + TAS (1.5h)

**自分で書く**:
- `examples/workload-lws-tas.yaml`: LWS で leader + workers、TAS annotation
- 推論っぽいワークロード (例: nginx で代替、コンテンツ通信を sleep で擬似)

apply して Kueue が admit、LWS が leader/worker を起動、TAS が rack 局所性を効かせるかを確認。

##### Sub-phase 6b: Volcano vcjob (45 min)

**自分で書く**: `examples/workload-volcano.yaml` (vcjob)

`schedulerName: volcano` を指定。capacity 不足時の挙動を確認 (gang が効くか)。

##### Sub-phase 6c: 4 戦略の比較計測 (1h)

`tools/run_experiment.sh` を 4 scheduler 対応に拡張:
- default
- topogang (自作)
- kueue (Kueue + JobSet)
- volcano

同じワークロードで N trial 走らせる。`docs/experiments/runs/.../elapsed.csv` に集計。

##### Sub-phase 6d: tools/plot.py (45 min)

ツール類なので skeleton 程度:
```python
"""run_experiment.sh の出力 CSV と JSON を読んで図を生成"""
import os, json, sys, glob, csv
import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt
# 関数: load_elapsed, load_locality, plot_completion_time, plot_locality, plot_concept
# 中身は自分で
```

3 つの図を生成:
- **Figure 1**: 4 scheduler の completion time box plot
- **Figure 2**: 4 scheduler の locality 比率 bar plot
- **Figure 3**: 4 戦略の概念図 (in-tree / 自作 / queue layer / 全置換)

##### Sub-phase 6e: 比較表 (`docs/design/three-strategies.md`) (30 min)

| 戦略 | 代表 | 何を置き換える | gang 実装場所 | topology 実装 | preemption | multi-cluster |
|------|------|---------------|---|---|---|---|
| in-tree | v1.35 GangScheduling | 何も置き換えない | PreEnqueue + Permit | KEP-5732 (1.36 alpha) | KEP-5710 (1.36 alpha) | なし |
| in-scheduler 拡張 | 自作 / scheduler-plugins | 何も置き換えない、追加 | Permit | Score plugin | preemption plugin | なし |
| 上流 admission | Kueue | 何も置き換えない、上流 | Job suspend で hold | TAS (Workload 単位) | あり | MultiKueue |
| scheduler 全置換 | Volcano | kube-scheduler 自身 | 自前 actions | NUMA/network plugin | あり | なし (volcano native) |

#### Phase 7: レポート執筆 (4-5h) ★

**§1 はじめに** (0.3 page)
- AI workload と default scheduling の不適合
- Kubernetes 1.35 で workload aware scheduling が in-tree 化した経緯
- 自分版 spec (Day 1) の要約 1 段落

**§2 Kubernetes scheduler の構造** (0.5 page)
- kube-scheduler と Scheduling Framework
- 各 extension point の役割
- default で出来ない 2 つの典型: gang / topology

**§3 自作 plugin と公式実装による検証** (1.0 page) — コア
- 自作 gang plugin (Permit + Reserve + PreFilter) と v1.35 公式 GangScheduling (PreEnqueue + Permit) の **設計差**
- 自作 topology plugin (3 案比較→ leader pinning) と Kueue TAS / KEP-5732 の **設計差**
- 図 1: 4 scheduler の挙動差
- 図 2: rack locality 比率

**§4 設計戦略の俯瞰** (1.0 page)
- 4 戦略の比較表
- 各戦略の強み/弱み
- v1.35 in-tree GangScheduling と Volcano が共存する理由
- 図 3: 4 戦略の概念図

**§5 ハードウェアとの接続** (0.5 page)
- DRA, NVLink ComputeDomain, MIG, RDMA NIC alignment
- KEP-5732 (Topology-Aware Workload Scheduling) の方向性
- ideal-scheduler.md §6 を圧縮: BlueField + MoE の placement vs flow steering 階層分担

**§6 まとめと展望** (0.3 page)

#### Phase 8: Final polish (30 min)
- proofread, 冗長削減
- Figure caption 確認
- 用語の一貫性
- 参考文献 (KEP, Kueue/Volcano docs)

#### Pitfalls (症状ベース)

- LWS で leader が起動しない → role label が leaderTemplate / workerTemplate で正しいか
- Volcano が default scheduler と競合 → `schedulerName: volcano` 指定 Pod のみ
- plot.py が空図 → load_* 関数が空リストを返している、glob pattern 確認
- レポートで時間切れ → 図を先に固定して figure-driven writing
- 4 戦略の比較表が冗長 → 1 行 1 戦略で軸を 5-6 個に絞る

#### Self-check (deliverable check)

- [ ] LWS で leader+workers が動いた
- [ ] Volcano vcjob が動いた
- [ ] 4 scheduler 比較計測の CSV と図がある
- [ ] 比較表 (`three-strategies.md`) が完成
- [ ] `ideal-scheduler.md` の §6 (BlueField/MoE 接続) が抽象論で終わっていない
- [ ] 4 ページの technical note が完成
- [ ] BlueField / MoE への接続が 1 段落以上
- [ ] GitHub repo が public で README に再現手順
- [ ] `docs/design/` に 7 ファイル (my-spec, api-schema, gang-scheduling, workload-api-comparison, topology-aware, ai-platform, three-strategies, ideal-scheduler)
- [ ] 各 Design Doc の §6 (既存実装との比較) が埋まっている
- [ ] `pkg/coscheduling/` と `pkg/topologyaware/` のコード行数を記録

#### Stretch goals
- MultiKueue を 2 cluster で構築 (重い、scope 外 stretch)
- KubeCon プレゼン視聴・引用

---

## 6. 提出レポートに使うべきキーフレーズ

LT46 reviewer (PFN 計算基盤チーム) が「ちゃんと考えた」と感じるキーワード群:

**v5 で増えたもの (公式実装の経験)**:
- **Workload API (`scheduling.k8s.io/v1alpha1`)** — v1.35 alpha
- **GangScheduling plugin (in-tree)** — PreEnqueue + Permit
- **OpportunisticBatching** (Beta, 同型 Pod の scheduling 高速化)
- **Workload Scheduling Cycle** (1.36 alpha) — PodGroup を batch 処理
- **KEP-5732 Topology-Aware Workload Scheduling**
- **KEP-5710 Workload-aware Preemption**
- **Kueue Topology CRD と TAS** (`podset-required-topology`, `podset-preferred-topology`)
- **JobSet と LeaderWorkerSet** — training 用 / inference 用の Workload controller
- **MultiKueue** — multi-cluster dispatch
- **Cohort, ResourceFlavor, ClusterQueue, LocalQueue** — Kueue の admission 構造

**v4 から継承**:
- Scheduling Framework の extension points (PreEnqueue / PreFilter / Filter / Score / Reserve / Permit / Bind)
- DRA, ResourceClaim, ResourceSlice
- NVLink ComputeDomain, MIG / Time-Slicing
- inter-node bandwidth-sensitive な all-to-all (MoE)

---

## 7. 自分の研究 (BlueField / MoE) への接続 — レポート §5 の 1 段落の素材

- 自作 scheduler plugin と Kueue TAS が今表現しているのは「rack 局所性」など static な topology
- KEP-5732 (Topology-Aware Workload Scheduling, 1.36 alpha) は **Workload 全体を 1 unit として physical placement 評価** する方向。これは自作の per-Pod scoring より AI 学習の本質に近い
- AI 推論 / 学習の現実は **dynamic** — load 変化、token 分布、expert 利用率で最適配置が動く
- 自分の研究 (BlueField DPU 上の動的 learned index による MoE routing) は **dynamic な配置最適化を、データプレーンに近い場所で行う** 試み
- Kubernetes の階層分担: **scheduler (KEP-5732) は cluster-level の粗い placement を、DPU は node-pair-level の細かい flow steering を**
- Kubernetes の DRA / Workload API と SmartNIC 上の動的 routing は **互いに補完的** — 前者は **placement**、後者は **flow steering**
- このスタックの完成形が PFN の「次世代計算インフラの計測・制御技術」の一断面

---

## 8. もし詰まったら (症状 → 確認すべきこと)

| Day | 症状 | 確認すべきこと |
|-----|------|---------------|
| 2 | scheduler 起動しない | RBAC, image load, leaderElection: false |
| 2 | Nodemask Args 届かない | scheduler-plugins/pkg/capacityscheduling の Args デコード参考 |
| 3 | Design Doc 時間足りない | §2 を 2 案、§3-§5 を厚く |
| 3 | gang 動かない | Permit 呼ばれているか、Reserve 有無、PodGroup label |
| 3 | sync 関連 dead lock | lock 範囲、callback 内の別 lock |
| 4 | `no kind "Workload"` | API group enabled か、`runtime-config` |
| 4 | Workload 作っても無反応 | feature gate が apiserver と scheduler **両方** で有効か |
| 4 | kind v1.35 image 取れない | kind release で対応 image 確認 |
| 5 | 3 案比較書けない | 単純案と高度案の 2 案でも OK、各案の弱点を真面目に |
| 5 | score 反映されない | weight, NormalizeScore, dominant な他 score |
| 5 | KEP-5732 alpha 動かない | master ブランチ build は scope 外、ペーパー読みで代替 |
| 6 | Workload Pending のまま | ResourceFlavor の nodeLabels が実 label と一致 |
| 6 | TAS 効かない | Pod template annotation の key |
| 7 | LWS leader 起動しない | role label の leaderTemplate / workerTemplate |
| 7 | レポート書けない | 図先固定 figure-driven writing、Design Doc から引用 |
| 7 | 時間切れ | MultiKueue を諦め、Volcano は install + 1 vcjob のみ |

---

## 9. 締め切り感覚 (v5)

| Day | 終了時刻 | 時間 (h) | 累積 (h) |
|-----|---------|---------|---------|
| 2 | 24:00 | 9-10 | 10 |
| 3 | 25:00 | 11-12 | 22 |
| 4 | 24:00 | 10-11 | 33 |
| 5 | 25:00 | 12-13 | 46 |
| 6 | 24:00 | 11-12 | 58 |
| 7 | 25:00 | 12-13 | 71 |

合計 ~71 h。**Day 3, 5, 7 が山** (各 12-13h)。Day 3 に週末土曜、Day 5 に週末日曜を当てるのが王道。

v4 (66h) → v5 (71h) と +5h。これは v1.35 cluster 構築 + v1.36 alpha 観察 + LWS / MultiKueue / Volcano の 4 戦略目分。

---

## 10. 終わりに

v4 から v5 への変更で何が起きたか:

- **問題文「Kubernetes で導入が進んでいる workload aware scheduling」が直接指す機能 (v1.35 GangScheduling, Workload API)** に正面から取り組む構成に
- 「自作だけ書く」「Kueue を表面的に触る」では reviewer の期待に届かない。**4 戦略 (in-tree / 自作 / Kueue / Volcano) を実機で経験し、設計差を語る** 構成
- **AI 学習基盤の Kueue 構築 (JobSet + LWS + TAS + Topology + ResourceFlavor + ClusterQueue + Cohort)** で、Kueue を実運用に近い深さで触る
- v1.36 alpha (KEP-5732 Topology-Aware Workload Scheduling) が **あさんの研究と直結する KEP** として §5 で参照される

このプロジェクトを Day 7 まで完走すれば:

- 自作 plugin (gang + topology) のコード
- v1.35 GangScheduling との比較ノート
- v1.36 alpha (KEP-5732, KEP-5710) の理解
- Kueue で構築した AI 学習基盤
- Volcano との比較
- 4 戦略の俯瞰図
- レポート 4 ページ
- 自分の研究との接続 1 段落

これら全てが手元に残る。**面接 30 分を支配する材料に加えて、PFN の AI 基盤チームと同じ語彙で対話できる素地** が出来る。

詰まったら相談を。エラー log か kubectl 出力を見せてくれれば、どこを疑うかの hint を返します。Go plugin のコードは出しません。
