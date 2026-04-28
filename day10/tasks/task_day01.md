# TopoGang: 自作 Workload-Aware Scheduler を書いて理解する 1 週間プロジェクト (v4)

> LT46 問1-B「Kubernetes workload aware scheduling」を、**手を動かして**理解しきるための計画書 — v4。
> 最終成果物はそのまま提出レポート (A4 2 枚) の下書きになる。

## v4 で何が変わったか

v3 は実装フェーズを明示したが、**Go plugin のコード骨格 (struct 定義、scoring 式、Permit の実装) を本文中に書いてしまっていた**。これだと「書く前に目に入った」状態になり、自分で設計したとは言えない。

v4 ではこれを修正:

1. **Go plugin (Day 2 / 3 / 4) のコードは一切示さない。** 代わりに「実装に入る前に答えるべき問い」と「参照すべき一次資料リンク」だけ提示する。
2. **ツール類 (observe.py / parse_events.py / run_experiment.sh / plot.py) は、テンプレート程度の skeleton は OK** とする。これらは主体ではなく計測の足回りなので、ハマって時間を溶かしても得るものが少ない。
3. **受け入れ条件は残すが、実装方法に触れない**。「PodGroupInfo を sync.Map で持つ」ではなく「同 PodGroup の Pod が並行に scheduling cycle に入っても破綻しない」のように、観測可能な振る舞いだけで書く。
4. **debug 中の Claude のサポート方針は明文化**。エラーメッセージを見せたら hint OK、しかし `こう書け` は出さない。
5. **「実装に入る前に答えるべき問い」セクションを各日に追加**。これが v4 の核。問いに自分で答えてから実装に入る。

### v1 → v2 → v3 → v4 の段階
- **v1**: 写経改造ベース、Design 判断は曖昧
- **v2**: + Design Challenge と Design Doc 追加 (設計判断を残す)
- **v3**: + Implementation Phase 追加 (実装判断を残す) → ただしコード骨格を出してしまった
- **v4**: コード骨格を削除し、問い + リンクに置き換える (考える主体を返す)

---

## 0. プロジェクトの位置付け

### 目的
1. Kubernetes の **Scheduling Framework の拡張点** (PreFilter / Filter / Score / Reserve / Permit / PostBind) を、**読む**のではなく **書く** ことで身体に入れる。
2. AI ワークロードの 2 大要件 — **gang scheduling** と **topology awareness** — を、自分の plugin で最低限動く形にする。
3. それを Kueue / Volcano / KEP-4671 (Workload API) と比較し、設計空間 (in-scheduler 拡張 vs job queue layer vs scheduler 丸ごと差し替え) を実機で体感する。
4. 最終的に、自分の研究背景 (BlueField DPU、MoE routing、SmartNIC) に接続して **「ハードウェア特性を scheduler に伝えるとはどういうことか」** を語れるようにする。
5. 各設計判断について「なぜこの選択をしたか」を Design Doc として残す。
6. **設計も実装も自分のものとして残す**。コード行数が少なくても、**自分の頭で考えた跡** が git log と Design Doc にある状態を作る。

### なぜ自作するのか
- Kueue や Volcano の README を読んでも、scheduler の中で何が起きているかは分からない。
- PFN の選考で「workload aware scheduling とは何か」と聞かれたとき、自分の手で書いた plugin の挙動を例に説明できる人と、KEP の引用しかできない人の差は決定的。
- あさんの BlueField 研究は「データプレーンに関わる制御を、できるだけハードウェアに近い場所でやる」話。**その視点で scheduler を見る** には自分でデータパスを書く経験が要る。

---

## 1. v4 における Claude (私) のサポート範囲

このプロジェクトを進める途中で詰まることは必ずある。その時の私のサポートは以下の線引きで動く:

### やる
- **エラーメッセージや log 出力を見せてもらえば、何を疑うべきかの hint** を出す
- **どのドキュメント / どのソースファイルを読めば判断材料が得られるか** をリンクで返す
- **観察された動作と期待動作のギャップ** を整理する手伝い
- **Design Doc のレビュー** — 論理が破綻していれば指摘
- **設計判断の選択肢の比較** — 候補を出すのは自分、Pros/Cons の整理は手伝う

### やらない
- **コードの提示** ("こう書け") は出さない。たとえ詰まっても出さない
- **答えに直結する選択肢の列挙** をしない (例: 「状態保存場所は CycleState か sync.Map か CRD status のどれにすべきか?」とは聞かれても、私はその 3 つを列挙しない。代わりに「永続性、再起動耐性、可視性、書き込み頻度…の軸で考えてみて」と返す)
- **Self-check questions に対する模範解答** を示さない

### debug の典型フロー
1. あさん: `Permit が呼ばれていないっぽいです。kubectl logs はこれ → [log貼り付け]`
2. 私: `この log だと PreFilter で reject されている可能性が高いです。PreFilter の return status を確認できますか? あと、podgroup label は pod template の metadata.labels に入っているか?`
3. あさん: 確認 → 問題発見

`こう書けば動きますよ` という応答は **意図的に出さない**。

---

## 2. Design Doc テンプレート (v2 から継承)

各 plugin について、実装前に書き、実装後に追記する。`docs/design/<plugin>.md` として残す。

```markdown
# Design Doc: <Plugin Name>

## 1. Motivation
- 解こうとしている問題 (default scheduler では何ができないか)
- 想定するワークロード (具体例 1-2 個)
- Non-goals

## 2. Alternatives Considered
最低 2 つ、できれば 3 つ。各案について Pros / Cons / Why rejected を書く。

## 3. Chosen Design
- 採用したアプローチ
- 使う extension point とその理由
- データ構造 (struct 定義レベルで)
- 状態遷移
- 並行性

## 4. Rationale
- なぜ Option X を選んだか
- 何を最大化し、何を諦めたか

## 5. Trade-offs and Limitations
- 既知の問題、スケール限界、未対応ケース

## 6. Comparison with Existing Implementation (実装後に追記)
- scheduler-plugins/coscheduling など既存実装と比べてどう違うか

## 7. Future Work
```

**重要なルール**:
- §1〜§5 は既存実装を読む *前* に書く
- §6 は読んだ後に書く

---

## 3. 環境とツールチェーン

### 必須
- Linux または macOS (Apple Silicon でも可)
- Docker, `kind` v0.24+, `kubectl` v1.31+, `helm` v3
- `go` 1.22+
- `make`, `git`, `python3`

### 推奨
- `k9s`, `stern`, `jq`, `yq`

### リソース見積
- CPU 4+ core, RAM 16+ GB, Disk 30+ GB

### ディレクトリ構成
```
topogang/
├── go.mod
├── cmd/
│   └── scheduler/
│       └── main.go
├── pkg/
│   ├── nodemask/                 # Day 2: Filter plugin (ウォームアップ)
│   ├── coscheduling/             # Day 3: gang plugin
│   └── topologyaware/            # Day 4: topology plugin
├── manifests/
│   ├── scheduler-config.yaml
│   ├── deployment.yaml
│   └── crds/
│       └── podgroup.yaml         # Lift OK
├── tools/
│   ├── observe.py                # Day 1
│   ├── parse_events.py           # Day 5
│   ├── run_experiment.sh         # Day 5
│   └── plot.py                   # Day 6
├── examples/
│   ├── workload-bad-no-gang.yaml
│   ├── workload-good-gang.yaml
│   └── workload-topology.yaml
└── docs/
    ├── README.md
    ├── design/                   # Design Doc
    ├── predictions/              # 予測 → 検証
    └── experiments/
```

---

## 4. 全体像と日程

| Day | テーマ | Design Challenge | 実装の核 (自分で書く) | 主な成果物 |
|-----|--------|------------------|--------------------------|-----------|
| 1 | Foundation + 観察 | 自分版 spec | observe.py | playground, my-spec.md |
| 2 | 自作 scheduler の最小構成 | API schema 設計 | Noop + Nodemask Filter, manifests | scheduler が動く + 2 plugin |
| 3 | Gang scheduling | gang plugin Design Doc | gang plugin (PodGroup state, quorum, Permit) | gang plugin, Design Doc |
| 4 | Topology-aware scoring | scoring 関数 3 案比較 | topology plugin (leader 検出, score) | topology plugin, Design Doc |
| 5 | Kueue 比較 | 挙動の事前予測 | parse_events.py, run_experiment.sh | Kueue 設定 + 計測 harness |
| 6 | Volcano / Workload API | 自分の "理想 scheduler" 像 | plot.py で 3 戦略の比較図 | 比較ノート, plot 群 |
| 7 | レポート執筆 | (なし) | 計測 fix-up + 4 ページ tech note | LT46 提出版 |

各 day の構成:
- **Goal**
- **Why this matters for LT46**
- **Phase 1: 一次資料を読む** (≤ 1h)
- **Phase 2: Design Challenge** (1-2h, 該当日のみ)
- **Phase 3: 実装に入る前に答えるべき問い** ← v4 の核
- **Phase 4: 実装中に参照すべき資料** (godoc, GitHub のファイルパス, KEP)
- **Phase 5: Setup** (lift OK な準備)
- **Phase 6: Implementation** (★ 自分で書く、私はコードを示さない)
- **Phase 7: Verification & Experiments** (受け入れ条件)
- **Pitfalls** (症状ベース、解法は出さない)
- **Self-check questions**
- **Stretch goals**

---

## 5. Day-by-Day 計画

### Day 1: Foundation — playground, 観察, 自分版 spec, observe.py

#### Goal
- 4 node の kind cluster が動いていて、各 worker に rack label と GPU capacity がある
- KEP-4671 (Gang Scheduling using Workload Object), KEP-5278 (NominatedNodeName), Scheduling Framework, DRA を 1 周読み終えた
- default scheduler の限界を実例で観察した
- 自分版 spec を 1 ページに書き、KEP-4671 と差分比較した
- observe.py が動いていて、Day 7 で再利用可能

#### Why this matters for LT46
- レポート §1 を「自分の観察として」書ける
- observe.py は Day 7 の計測でそのまま使うので、初日に instrumentation を整える

#### Phase 1: 一次資料 (1h)

**読むもの (順番にこの通りで)**

1. **Kubernetes 公式 v1.35 Blog "Introducing Workload Aware Scheduling"**
   <https://kubernetes.io/blog/2025/12/29/kubernetes-v1-35-introducing-workload-aware-scheduling/>
   - なぜ workload aware が必要か。motivation を抑える

2. **KEP-4671: Gang Scheduling using Workload Object** の §Summary と §Motivation のみ (まだ Proposal は読まない)
   <https://github.com/kubernetes/enhancements/blob/master/keps/sig-scheduling/4671-gang-scheduling/README.md>
   - これが Workload API の本体 KEP。1.35 は最初の deliverable
   - **重要**: Design Challenge が終わるまで §Proposal 以降は読まない

3. **Scheduling Framework concept docs**
   <https://kubernetes.io/docs/concepts/scheduling-eviction/scheduling-framework/>
   - 各 extension point を図で確認。後の plugin 実装の地図になる

4. **DRA concept docs**
   <https://kubernetes.io/docs/concepts/scheduling-eviction/dynamic-resource-allocation/>
   - device plugin との違い、ResourceClaim/ResourceSlice モデルだけ把握

#### Phase 2: Design Challenge — 自分版 spec (1h)

`docs/design/my-spec.md` に書く構造 (中身はあさんの判断):

```markdown
# 自分版 Workload Aware Scheduling Spec (KEP を読む前の試案)

## 1. 観察された問題
Day 1 の観察 A/B/C で見えた default scheduler の限界

## 2. 解くべきもの (must / nice-to-have / out-of-scope)

## 3. API 案
ユーザーはどう要件を伝えるべきか? 候補を 2-3 個出して比較

## 4. Backward compatibility

## 5. (後で追記) KEP-4671 との比較
```

書き終えてから KEP-4671 の §Proposal 以降を読み、§5 を追記。

**追記する観点**:
- 一致した点
- 自分が見落としていた点
- KEP の選択に違和感を感じた点 (あれば)
- KEP-4671 が Workload と PodGroup を分けている理由を、自分の試案と比べて理解できたか

#### Phase 3: 実装に入る前に答えるべき問い (Day 1 は observe.py 用、20 min)

**問い 1**: observe.py が出力する JSON の最低限のスキーマは何か? Day 7 で matplotlib に直接食わせられる構造とは?
ヒントの軸: pod 単位 vs PodGroup 単位の集計、時系列を残すか / サマリだけか、rack 分布をどう表現するか

**問い 2**: Pod の状態を「初めて Pending を観測した時刻」と定義するか、「Pod が API server に作成された時刻」と定義するか? 両者はどう違うか?

**問い 3**: kubectl get -w (watch) を使うか、定期 polling するか? それぞれの利点と落とし穴は?

これに自分で答えてから実装に入る。

#### Phase 4: 実装中に参照すべき資料

- `kubectl get` の JSON 出力構造 (`kubectl get pods -o json` を実行して確認)
- Pod の status fields: <https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/pod-v1/#PodStatus>
- Python `subprocess` の使い方 (これは標準ライブラリなので公式 docs)

#### Phase 5: Setup — kind cluster (1h)

1. `kind-config.yaml` を書いて 4 node 立てる (1 control-plane + 3 worker)。kind 公式 docs: <https://kind.sigs.k8s.io/docs/user/configuration/>

2. Topology label 付与:
   ```bash
   kubectl label node topogang-worker  topology.example.io/rack=A topology.example.io/block=1
   kubectl label node topogang-worker2 topology.example.io/rack=A topology.example.io/block=1
   kubectl label node topogang-worker3 topology.example.io/rack=B topology.example.io/block=2
   ```

3. **Fake GPU capacity を patch**。kind には GPU が無いので、capacity だけ「自称」させる。
   - Status subresource を patch する API は次の docs で確認: <https://kubernetes.io/docs/reference/using-api/server-side-apply/>
   - 自分の kubectl version で動く方法を試行錯誤で見つける (`kubectl proxy + curl` 方式 / `kubectl patch --subresource=status` 方式のどちらか)

#### Phase 6: Implementation — observe.py を自分で書く (2h) ★

**自分で書く** (テンプレート程度の skeleton は OK としているので、この程度のスケルトンだけ示す):

```python
import subprocess, json, time, sys
from collections import defaultdict

def get_pods(selector, namespace="default"):
    # subprocess で kubectl get pods -l <selector> -o json
    # JSON parse して items を返す
    pass

def get_node_rack(node_name):
    # node の topology.example.io/rack label を返す
    pass

def main(selector, max_seconds=300):
    # 1 秒間隔で Pod 状態を観測
    # 各 Pod について最初に Pending / Running を観測した時刻を記録
    # 全 Pod が Running になったら終了
    # 結果を JSON で出力
    pass

if __name__ == "__main__":
    main(sys.argv[1])
```

これより詳細はあさんが Phase 3 の問い 1〜3 に答えてから自分で書く。

**Lift OK**: なし

#### Phase 7: Verification — 受け入れ条件

- [ ] kind cluster の 3 worker がすべて rack label と GPU capacity を持つ
- [ ] 試行 A: 普通の Pod 1 つを投げると schedule される
- [ ] 試行 B: 4 Pod の Job (各 Pod が 2 GPU 要求) を投げると、capacity 不足のため 1 Pod が Pending のまま残る
- [ ] 試行 C: topologySpreadConstraints の挙動と、「全 Pod を rack A に寄せる」要件の差を 1 段落で説明できる
- [ ] observe.py が試行 B で「3 Pod は Running、1 Pod は Pending」「rack 分布」を JSON で出す
- [ ] my-spec.md が完成し、§5 (KEP との比較) に最低 2 点の差分が書かれている

#### Pitfalls (症状ベース)

- kind の Node は同じ Docker host 上の container。**スケジューラ的には別 node、物理的には同居** という二重構造を意識
- `kubectl patch` の `--subresource=status` 互換性が version で違う。手元で動く方を採用
- observe.py で UID をキーにすること (Pod 名は使い回されることがある)

#### Self-check questions

1. PodGroup を「全員揃ってから schedule」させる仕組みを default scheduler で書けるか?
2. 同一 PodGroup の 4 Pod を「全員 rack A」に寄せる仕組みを default scheduler で書けるか?
3. KEP-4671 が言う「workload aware」とは具体的に何のことか? 自分の言葉で 3 行で
4. 自分版 spec と KEP-4671 で一番大きく違ったのはどこか? その違いはなぜ生じたか?

#### Stretch goals
- kwok を試して 100 node の sim を作る: <https://kwok.sigs.k8s.io/>
- `node-feature-discovery` を入れて、もう少し本格的な node label を出す

---

### Day 2: 自作 scheduler の最小構成 — Noop + Nodemask Filter, manifests 手書き

#### Goal
- 自作 scheduler binary が container 化されて kind cluster で動いている
- "Noop" Score plugin が呼ばれるログが見える
- **Nodemask Filter plugin** (設定可能な node 除外) を自分で実装した。これが Day 3 の本格実装のウォームアップ
- Deployment, RBAC, ConfigMap 全てを手書きした
- API schema 設計が docs/design/api-schema.md にある

#### Why this matters for LT46
- レポート §2 の「Scheduling Framework」を自分の binary で動かしたという経験で語れる
- Filter plugin の実装経験が、Day 3 の Permit plugin 実装の足掛かり

#### Phase 1: 一次資料 (45 min)

1. **Scheduler Configuration 公式 docs**
   <https://kubernetes.io/docs/reference/scheduling/config/>
   - profile, plugin, pluginConfig の関係。自分の scheduler を component config でどう定義するか

2. **Configure Multiple Schedulers**
   <https://kubernetes.io/docs/tasks/extend-kubernetes/configure-multiple-schedulers/>
   - second scheduler の deploy パターン、RBAC

3. **scheduler-plugins README**
   <https://github.com/kubernetes-sigs/scheduler-plugins>
   - out-of-tree plugin の構造、`cmd/scheduler/main.go` の register 方法

4. **`framework` package godoc** — `Plugin`, `FilterPlugin`, `ScorePlugin` interface だけ確認
   <https://pkg.go.dev/k8s.io/kubernetes/pkg/scheduler/framework>

5. **Lee Bernick "Writing a Kubernetes Scheduler Plugin"** (2023) — 体験記、ハマりどころが豊富
   <https://www.leebernick.com/posts/scheduler-plugin/>

#### Phase 2: Design Challenge — API schema 設計 (1h)

`docs/design/api-schema.md` に「ユーザーが gang/topology の要件を伝える方法」を 2-3 案比較。

**考えるべき軸** (これらに 1 つずつ答えながら案を比較する):
- ユーザーが書く YAML の冗長さ
- Server-side validation のしやすさ (CRD なら schema 検証が効く、label/annotation だと弱い)
- controller を別途実装する必要性
- Pod が動的に増減するワークロード (elastic) を表現できるか
- 1 PodGroup 内で異なる resource 要求を持つ Pod (heterogeneous) を表現できるか

これらに対する答えで案が決まる。

書き終わったら scheduler-plugins の crd.yaml を見て自分の schema との差分を末尾に書く:
<https://github.com/kubernetes-sigs/scheduler-plugins/blob/master/manifests/coscheduling/crd.yaml>

#### Phase 3: 実装に入る前に答えるべき問い (40 min)

**問い 1 (Noop Score)**: Score plugin の戻り値は int64。範囲はどう決まっているか? `framework.MaxNodeScore` は何の値?

**問い 2 (Nodemask Filter)**: Filter plugin が「この node は不適格」と返す方法は? `framework.Status` のどの値を使うか? `Unschedulable` と `UnschedulableAndUnresolvable` の違いは?

**問い 3 (Plugin Args)**: PluginConfig で `Nodemask` に渡す `excludeLabel` 値は、Go コード側でどう受け取るか? `runtime.Object` の中身はどんな型?

**問い 4 (Multi-extension-point)**: 1 つの plugin が Filter と Score の両方を実装するには、Go 上でどう書くか? インターフェースの埋め込み? 型 assertion?

**問い 5 (RBAC)**: 自作 scheduler の ServiceAccount は最低限どの権限を持つべきか? `pods/binding` だけで足りる? `events`, `nodes` も要る? それぞれ何のために?

これらに自分で答えてから実装に入る。**問い 3, 4 は scheduler-plugins の他 plugin (例: capacityscheduling, nodenumaresource) を読みながら考える**:
- `pkg/capacityscheduling`: <https://github.com/kubernetes-sigs/scheduler-plugins/tree/master/pkg/capacityscheduling>

#### Phase 4: 実装中に参照すべき資料

- **`framework` package godoc** (各 interface のシグネチャ): <https://pkg.go.dev/k8s.io/kubernetes/pkg/scheduler/framework>
- **scheduler-plugins/cmd/scheduler/main.go** (plugin register の書き方): <https://github.com/kubernetes-sigs/scheduler-plugins/blob/master/cmd/scheduler/main.go>
- **scheduler-plugins の任意の Filter plugin** (例: `pkg/nodenumaresource`) — Args デコードの例
- **scheduler-plugins/manifests/install/** — RBAC の例 (コピペするのではなく見ながら自分で書く)

#### Phase 5: Setup (1h)

1. scheduler-plugins を fork & clone:
   ```bash
   git clone https://github.com/kubernetes-sigs/scheduler-plugins.git topogang
   cd topogang
   git checkout -b topogang-day2
   ```

2. Go module を確認、Makefile を眺める

#### Phase 6: Implementation (3-4h) ★

##### Sub-phase 6a: Noop plugin (30 min)
最小の Score plugin。`framework.ScorePlugin` interface を満たし、log に「呼ばれた」を残し、score 0 を返すだけ。
- Phase 3 問い 1 の答えを踏まえて書く
- 実装は 30-50 行程度に収まるはず

##### Sub-phase 6b: Nodemask Filter plugin (1.5h)
設定で渡される `excludeLabel` を持つ node を Filter で reject。
- Phase 3 問い 2, 3 の答えを踏まえて書く
- Args struct を自分で定義、`New()` で `runtime.Object` から decode
- 実装は 80-120 行程度

##### Sub-phase 6c: Manifests を手書き (1.5h)
- `manifests/scheduler-config.yaml`: KubeSchedulerConfiguration で Noop / Nodemask を有効化、Nodemask の args を pluginConfig で渡す
- `manifests/deployment.yaml`: ServiceAccount, ClusterRole, ClusterRoleBinding, ConfigMap, Deployment

scheduler-plugins/manifests/install/ を見ながら、コピペでなく**自分で書きながら何のための権限か理解する**。

##### Sub-phase 6d: cmd/scheduler/main.go を更新

**Lift OK**: scheduler-plugins の `cmd/scheduler/main.go` の plugin register 構造はほぼボイラープレート。`app.WithPlugin(noop.Name, noop.New)` のような形を真似て自分の plugin を追加する。

##### Sub-phase 6e: Build & Deploy
```bash
make local-image
kind load docker-image registry.k8s.io/scheduler-plugins/kube-scheduler:local --name topogang
kubectl apply -f manifests/
kubectl logs -n kube-system deploy/topogang-scheduler -f
```

#### Phase 7: Verification — 受け入れ条件

- [ ] テスト Pod (`schedulerName: topogang`) を投げると Noop scoring ログが node 数ぶん出る
- [ ] node に label `do-not-schedule=true` を付け、Pod を投げると **その node には行かない** (Nodemask が効く)
- [ ] label を外すとその node にも行く
- [ ] Nodemask の Args (excludeLabel) を別の値に変えて再 deploy → 設定が反映される
- [ ] manifests/deployment.yaml の各リソースの役割を 30 秒で説明できる
- [ ] api-schema.md に 2-3 案の比較と採用理由が書かれている

#### Pitfalls (症状ベース)

- 自作 scheduler の Pod が起動しても何もしない → RBAC を疑う (`pods/binding` 等の権限)
- `kubectl logs` に何も出ない → ConfigMap の mount, scheduler-config.yaml の syntax
- Nodemask が常に reject する / 常に通す → Args が Go コードに届いていない、`runtime.Object` decode を確認
- Image を kind に load し忘れている → `kind load docker-image` 後に `docker exec topogang-control-plane crictl images` で確認

#### Self-check questions

1. Noop plugin は Pod 1 つに対して何回呼ばれる? なぜ?
2. 各 extension point は scheduling cycle のどの順番で呼ばれるか?
3. 自分が選んだ API schema (PodGroup CRD / label / annotation のどれか) を選んだ理由を 30 秒で説明できるか?
4. ClusterRole に追加した権限を 1 つ抜いたら何が起きるか?

#### Stretch goals
- `klog -v 4` でログレベルを上げて scheduling cycle 内部を読む
- Nodemask を「複数 label の OR」に拡張する

---

### Day 3: Gang Scheduling — PodGroup state, quorum, Permit を自分で書く

#### Goal
- gang plugin の Design Doc が `docs/design/gang-scheduling.md` に書かれている (§1〜§5 はコード読む前)
- **自分で書いた** plugin が動いている (gang admission が all-or-nothing で機能)
- 「4 Pod のうち 3 つだけ schedulable な状況」で **default scheduler は 3 つ走らせるのに、自作 gang plugin は 0 つしか走らせない** という挙動の差を再現
- Design Doc §6 (既存実装との比較) が追記されている

#### Why this matters for LT46
- レポートの中核。「私はこう設計し、こう実装した」と Design Doc + git log で説明できる状態
- 面接で「なぜ Permit を選んだか? 状態をどこに持ったか?」のあらゆる質問に自分の Design Doc + 自分のコードで答えられる

#### Phase 1: 一次資料 (1h)

1. **`framework.PermitPlugin` の godoc**
   <https://pkg.go.dev/k8s.io/kubernetes/pkg/scheduler/framework#PermitPlugin>
   - 戻り値の `Status.Code` (Wait, Success, Unschedulable) と timeout の意味

2. **`framework.Handle` の godoc** — `IterateOverWaitingPods`, `GetWaitingPod`, `SharedInformerFactory`
   <https://pkg.go.dev/k8s.io/kubernetes/pkg/scheduler/framework#Handle>
   - 別 Pod の Permit から waiting Pod を Approve する API がどこにあるか

3. **KEP-624 (Scheduling Framework) の §gang-scheduling 例**
   <https://github.com/kubernetes/enhancements/blob/master/keps/sig-scheduling/624-scheduling-framework/README.md>
   - 「Functionality similar to kube-batch (sometimes called gang scheduling) could be implemented as a plugin」のセクション。Permit と Reserve の組み合わせの設計意図

4. **scheduler-plugins/pkg/coscheduling の README** (コードはまだ開かない)
   <https://github.com/kubernetes-sigs/scheduler-plugins/blob/master/pkg/coscheduling/README.md>
   - どの extension point を有効化する必要があるか、どんな label を使うか

5. **KEP-42 (Lightweight Coscheduling)** の Design Challenge 後に読む
   <https://github.com/kubernetes-sigs/scheduler-plugins/blob/master/kep/2-lightweight-coscheduling/README.md>

6. **scheduler-plugins/kep/42-podgroup-coscheduling** — PodGroup CRD ベースの設計
   <https://github.com/kubernetes-sigs/scheduler-plugins/blob/master/kep/42-podgroup-coscheduling/README.md>

#### Phase 2: Design Challenge — gang plugin Design Doc §1〜§5 (2h)

`docs/design/gang-scheduling.md` をテンプレートに沿って書く。**コードは scheduler-plugins/pkg/coscheduling/ をまだ開かない状態で書ききる**。

書きながら自分で答えるべき設計判断 (これらが §3 / §4 / §5 の中身になる):
- どの extension point を使うか
- quorum 判定のタイミングと方法
- 状態の保持場所
- timeout 処理
- 並行性の扱い
- overflow (minMember を超えた Pod) の扱い

書き終えてから scheduler-plugins/pkg/coscheduling/ のコードを開いて §6 を書く:
<https://github.com/kubernetes-sigs/scheduler-plugins/tree/master/pkg/coscheduling>

#### Phase 3: 実装に入る前に答えるべき問い (1h)

これは Design Doc §3 と重なるが、**実装に必要な解像度で**答えること。

**問い 1 (extension point)**: なぜ Permit を使うのか? Filter で reject ではなぜダメなのか? 「Reserve した状態でブロック」という性質を Filter は実現できるか?

**問い 2 (state location)**: PodGroup の状態を、scheduler の中のどこに置くべきか? 候補には次のような性質の違いがある:
- 永続性 (scheduler restart で消えるか / 残るか)
- 読み書き頻度
- 可視性 (kubectl から見えるか)
- 並行 access の扱い

これらの性質を自分で評価して 1 つ選ぶ。

**問い 3 (peer の探し方)**: 同一 PodGroup の他の Pod が「今どの状態か (Pending / Reserved / Waiting / Running)」を知る方法は何か? `framework.Handle` が提供する API は? client-go の Lister は使えるか? どちらが scheduling cycle の中で安全か?

**問い 4 (Approve のタイミング)**: quorum が揃った瞬間、自分の Pod は Permit から何を返すか? その時、まだ Wait 中の peer Pod たちはどうやって Approve するのか? `framework.Handle` のどの method?

**問い 5 (timeout)**: `scheduleTimeoutSeconds` を超えたとき、Permit はどう振る舞うべきか? Wait の戻り値の `time.Duration` パラメータは何のためにある? timeout 後に Pod は backoff queue に戻るか reject されるか?

**問い 6 (PreFilter の役割)**: scheduler-plugins/pkg/coscheduling の README を見ると `preFilter is enhanced feature to reduce the overall scheduling time` とある。PreFilter で何をチェックすると性能が上がるか? なぜ PreFilter が「必須ではなく enhanced」と書かれているか?

**問い 7 (Unreserve)**: Reserve に成功して Permit Wait 中の Pod が、別の理由で失敗したとき、Reserve したスロットを開放する責任は誰にあるか? `Unreserve` plugin はいつ呼ばれるか?

**問い 8 (並行性)**: 同 PodGroup の Pod が並行に scheduling cycle に入ったとき、quorum カウントの整合性をどう取るか? `IterateOverWaitingPods` を呼び出している間に、別 cycle が状態を変更したら? lock の取り方は?

これら 8 つに自分で答えてから実装に入る。**答えは Design Doc に書き残しておくと、後で面接の素材になる**。

#### Phase 4: 実装中に参照すべき資料

- **`framework` package godoc** (Permit, Reserve, PreFilter の interface): <https://pkg.go.dev/k8s.io/kubernetes/pkg/scheduler/framework>
- **`framework.WaitingPod` の godoc** — `Allow()`, `Reject()` の使い方
- **scheduler-plugins/pkg/coscheduling/coscheduling.go** — 自分が詰まったら見て良い (Design Doc を書き終えた後)
- **scheduler-plugins/pkg/coscheduling/core/core.go** — PodGroupManager の実装。比較対象として
- **PodGroup CRD の Go types**: scheduler-plugins の `apis/scheduling/v1alpha1/types.go`
- **client-go Lister** の使い方: <https://pkg.go.dev/k8s.io/client-go/listers/core/v1>

#### Phase 5: Setup (1h)

**Lift OK**: PodGroup CRD の YAML をそのまま使う
```bash
curl -O https://raw.githubusercontent.com/kubernetes-sigs/scheduler-plugins/master/manifests/coscheduling/crd.yaml
cp crd.yaml manifests/crds/podgroup.yaml
kubectl apply -f manifests/crds/podgroup.yaml
```

**Lift OK**: PodGroup の generated typed client/lister/informer (codegen 出力なのでボイラープレート)。`go.mod` に依存追加して使う。

#### Phase 6: Implementation (5-6h) ★

ここが本プロジェクト最大の山。Design Doc §3 と Phase 3 の問いに対する自分の答えに沿って書く。

実装の流れ (順序の提案):

1. **PodGroup の in-memory 表現** を考えて書く (Phase 3 問い 2, 8 の答えに対応)
2. **PodGroup registry / manager** を考えて書く (state 全体の管理)
3. **PreFilter** を実装 (Phase 3 問い 6 に対応、まずは「PodGroup label が付いてない Pod は素通り」だけ書く)
4. **Reserve / Unreserve** を実装 (Phase 3 問い 7 に対応、状態追加 / 取り消し)
5. **Permit** を実装 (Phase 3 問い 1, 4, 5 に対応、quorum 判定 + Approve)
6. **cmd/scheduler/main.go** で plugin register
7. **scheduler-config.yaml** で各 extension point を有効化

詰まったら Phase 4 のリンクへ戻る。**最初から scheduler-plugins/pkg/coscheduling のコードを写経しないこと**。Design Doc §6 (比較) を書く価値が消える。

**Lift OK**:
- PodGroup CRD YAML
- Generated client/lister/informer
- cmd/scheduler/main.go の plugin register 行 (1 行追加するだけ)

**自分で書く**: 上記以外の plugin 本体 (推定 300-500 行)

#### Phase 7: Verification — 受け入れ条件 (抽象的に)

- [ ] PodGroup `minMember=4` で、4 Pod が capacity 不足 (3 Pod 分しか Reserve できない状況) で投げると **0 Pod が Running**
- [ ] capacity を 4 Pod 分に増やすと 4 Pod 同時 Running
- [ ] `scheduleTimeoutSeconds` 経過で Pod が reject される
- [ ] **2 つの異なる PodGroup を同時に投げても挙動が破綻しない**
- [ ] 同 PodGroup の Pod が並行に scheduling cycle に入っても quorum カウントが破綻しない (= 5 Pod 投げて minMember=4 なら、4 Pod が Running、1 Pod は overflow 処理されるか pending)
- [ ] observe.py の出力で「PodGroup の 4 Pod が ±数秒以内に Running になった」が確認できる
- [ ] Design Doc §6 が埋まっており、scheduler-plugins/coscheduling との設計差が書かれている (差は何点でも良いが、書ける差があるはず)

#### Pitfalls (症状ベース)

- Pod が永遠に Pending → Permit が呼ばれているか kubectl logs で確認、Permit が登録されているか scheduler-config.yaml 確認
- Permit が呼ばれているのに gang が機能しない → Reserve が同じ plugin で行われているか、scheduling cycle の順序を確認
- 4 Pod 投げても 1 Pod ずつ Running になる → quorum 判定が「自分含む」でない可能性
- timeout 後も Pod が Wait のまま → Permit Wait の `time.Duration` 戻り値の意味を再確認
- ロック関連でハング → `IterateOverWaitingPods` が lock を取った状態で呼ばれていないか、callback の中で別の lock を取ろうとしていないか
- PodGroup label を付けた Pod が PreFilter で reject される → label の `key` が `scheduling.x-k8s.io/pod-group` (新) か `pod-group.scheduling.sigs.k8s.io` (旧) か、scheduler-plugins の master を確認

#### Self-check questions

1. なぜ Permit で hold するのが正解で、Filter で hold するのは正解ではないのか? (Design Doc を見ずに答えられるか)
2. PodGroup の quorum を超える Pod が来たら? overflow の扱いを実装でどうしたか
3. PodGroup `minMember=4`, `parallelism=8` の Job を投げたら何が起きるか?
4. 自分の状態保存場所と scheduler-plugins の選択は同じか? 違うなら、なぜ?

#### Stretch goals

- **Backoff の自前実装** — quorum が揃わない PodGroup を全 Pod まとめて backoff queue に戻す
- **Eviction Controller** — running PodGroup の 1 Pod が落ちたら全員再起動 (controller を別に書く)
- **PodGroup status 更新** — registry の状態を CRD status に書き戻す

---

### Day 4: Topology-aware Score — 自分で scoring 関数を設計し実装する

#### Goal
- topology plugin の Design Doc が `docs/design/topology-aware.md` に書かれている (§1〜§5 はコード読む前)
- **自分で書いた** Score plugin が動いている (3 案以上を比較し、1 案を採用)
- 4 Pod の gang Job を投げると、複数試行で同 rack 寄せが観測される (default scheduler との比較で有意差)
- エッジケース 3 種で挙動を観察済み

#### Why this matters for LT46
- レポート §3 の topology-aware セクションを **3 案比較→ 1 案採用→ 実装→ 検証** の流れで書ける
- AI 分散学習の locality 要件を実機で語れる。あさんの研究 (BlueField + MoE inter-node) に直結

#### Phase 1: 一次資料 (1h)

1. **Score plugin の godoc** — `Score`, `NormalizeScore`, `ScoreExtensions`
   <https://pkg.go.dev/k8s.io/kubernetes/pkg/scheduler/framework#ScorePlugin>

2. **Kueue Topology Aware Scheduling**
   <https://kueue.sigs.k8s.io/docs/concepts/topology_aware_scheduling/>
   - hierarchical な node label モデル、Workload 単位での placement

3. **Kueue Topology object docs**
   <https://kueue.sigs.k8s.io/docs/concepts/topology/>
   - `levels` の表現、`spec.topologyName` で ResourceFlavor から参照

4. **KEP-5732 Topology-Aware Workload Scheduling** (kube-scheduler 内の topology の話)
   <https://github.com/kubernetes/enhancements/tree/master/keps/sig-scheduling/5732-topology-aware-workload-scheduling>
   - Workload 単位で「Placements」(候補ドメイン) を評価する設計

5. **scheduler-plugins/pkg/networkaware** (load-aware / network-aware scheduling) の README — Design Challenge 後
   <https://github.com/kubernetes-sigs/scheduler-plugins/tree/master/pkg/networkaware>

6. **NVIDIA DRA Driver - ComputeDomain** — multi-node NVLink を Kubernetes resource model に持ち込む先端実装、概念だけ
   <https://github.com/NVIDIA/k8s-dra-driver-gpu>

#### Phase 2: Design Challenge — scoring 関数 3 案比較 (2h)

`docs/design/topology-aware.md` をテンプレートに沿って書く。

**§2 Alternatives で最低 3 案出す**。各案について自分で考えるべき軸:
- 計算量 (per node, per Pod)
- PodGroup 内の最初の Pod (peer がまだ居ない Pod) の扱い
- rack の容量を超えた要求にどう振る舞うか
- 並行に scheduled されている peer の race condition
- 階層的 topology (block > rack > host) への拡張可能性

**ヒント**: 単純に同 rack peer 数を数えるだけだと、最初の Pod の score が全 node 0 になり、ランダム配置されてしまう。これをどう解くかが設計の核。

書き終えてから scheduler-plugins/pkg/networkaware を読み、§6 に差分を追記:
<https://github.com/kubernetes-sigs/scheduler-plugins/tree/master/pkg/networkaware>

#### Phase 3: 実装に入る前に答えるべき問い (1h)

**問い 1**: Score plugin の `Score()` 戻り値の int64 は、どの範囲を取るべきか? 別 plugin (NodeResourcesFit など) と weighted sum される時に、自分の plugin の score が dominant にならないようにするには?

**問い 2**: `NormalizeScore` は何のためにあるか? 自分の plugin で実装しないとどうなるか? `ScoreExtensions()` の戻り値が nil なら呼ばれない、これは何を意味するか?

**問い 3 (peer の発見)**: 同 PodGroup の Pod が **どの node に居るか** を知るために、自分の plugin はどんな lister / informer を使うべきか? Day 3 の gang plugin と同じ仕組みを使い回せるか?

**問い 4 (rack の発見)**: 候補 node の rack 情報は、Score の中でどう取得するか? `framework.Handle.SnapshotSharedLister().NodeInfos()` か、client-go の Node lister か? どちらが scheduling cycle の中で正しいか?

**問い 5 (Score plugin の負荷)**: Score plugin はすべての Filter を通った node について呼ばれる。1000 node のクラスタで PodGroup 内の 64 Pod を相互参照したら、計算量は? 最適化の余地は?

**問い 6 (3 案のうちどれを採用するか)**: Design Doc §4 (Rationale) に書いた採用理由は、実装の難易度と scheduling 品質のどちらを重視したか? 1 週間プロジェクトの制約を素直に書いたか?

#### Phase 4: 実装中に参照すべき資料

- **`framework.SnapshotSharedLister`** godoc — node の snapshot 取得
- **client-go Node Lister / Pod Lister** — `informer.Core().V1().Pods().Lister()` のような形
- **scheduler-plugins/pkg/networkaware** — 帯域・遅延考慮の実装
- **Day 3 で書いた自分の coscheduling plugin** — peer Pod の探し方は同じ informer で良いか確認

#### Phase 5: Setup (30 min)
- node label が Day 1 から残っているはず
- 実験用の Job manifest を `examples/workload-topology.yaml` として用意

#### Phase 6: Implementation (5-6h) ★

実装の流れ (順序の提案):

1. **Score plugin の skeleton** (Score / NormalizeScore / ScoreExtensions / New)
2. **rack label と peer 情報の取得 helper** を書く
3. **採用した scoring 案** (Design Doc §3 で選んだもの) を実装
4. **NormalizeScore** で 0-100 範囲にスケール
5. **scheduler-config.yaml** に Score を追加 (gang plugin と同じ profile)

詰まったら Phase 4 のリンクへ戻る。

**Lift OK**: なし (この plugin は完全自作)

**自分で書く**: 推定 200-400 行

#### Phase 7: Verification — 受け入れ条件

- [ ] kind の node 配置 (worker/worker2 が rack=A, worker3 が rack=B) で 4 Pod gang Job を投げると、複数試行で「全 Pod が rack=A に揃う」が default scheduler より有意に多い
- [ ] default scheduler (TopologyAware なし) で同じ Job を投げると、3+ Pod が rack=A 寄せにならない (control 実験)
- [ ] エッジケース 1: 同時に 2 PodGroup を投げても挙動が破綻しない
- [ ] エッジケース 2: rack=A の容量を 3 GPU に絞った時の挙動が、Design Doc §5 で予想していた通り (or 想定外なら追記)
- [ ] エッジケース 3: PodGroup 内の最初の Pod (peer 0) が、想定通りの node に行く
- [ ] observe.py の rack_distribution が期待通り
- [ ] Design Doc §6 が埋まっており、scheduler-plugins/networkaware との設計差が書かれている

#### Pitfalls (症状ベース)

- Score plugin が呼ばれない → scheduler-config.yaml の `score.enabled` リスト確認、weight 0 になっていないか
- 全 node の score が同じ → leader 検出 / peer 集計のどこかが空を返している、log で trace
- score を出しているのに rack 寄せが起きない → 他 plugin (NodeResourcesFit) の score が dominant、weight を上げる、または NormalizeScore の範囲を確認
- 並行で複数の「leader」が立候補する → 決定論的な leader 選出基準を再考 (CreationTimestamp 等)
- rack label が無い node でクラッシュ → fallback path がない、nil チェック

#### Self-check questions

1. Score plugin で「rack を寄せる」のと、Filter plugin で「rack=A の node 以外を弾く」の違いは?
2. 自分の scoring 関数は、PodGroup 内の最初の Pod (peer 0) でどう振る舞うか?
3. rack の容量が足りない場合の挙動は Design Doc §5 と一致したか?
4. 階層的 topology (block > rack > host) を表現したい場合、どこに手を入れるか?

#### Stretch goals

- **Hierarchical topology** — `block`, `rack`, `node` の 3 階層で score を加算 (Kueue TAS の思想)
- **Bandwidth-aware** — 想定通信量を annotation から読み、配置に反映
- **Filter plugin 版** — rack 強制 (緩い preference ではなく hard constraint) の実装と挙動の違い

---

### Day 5: Kueue 比較 + 計測 harness を自分で書く

#### Goal
- Kueue が install され、ClusterQueue / LocalQueue / ResourceFlavor が宣言されている
- TAS (Kueue v0.16+ では Topology CRD ベース) を有効化し、自作 plugin との挙動を比較した
- 挙動の事前予測ファイル (5 シナリオ) があり、検証で当たり/外れを分析した
- **計測 harness** (`tools/parse_events.py`, `tools/run_experiment.sh`) を自分で書いた

#### Why this matters for LT46
- 「scheduler 内で解く」vs「queue layer で解く」の差を実機で説明できる
- 計測 harness が Day 6/7 のすべての実験を支える基盤

#### Phase 1: 一次資料 (1h)

1. **Kueue Overview**
   <https://kueue.sigs.k8s.io/docs/overview/>

2. **Kueue Concepts** (Workload, ClusterQueue, LocalQueue, Cohort, ResourceFlavor)
   <https://kueue.sigs.k8s.io/docs/concepts/>

3. **Kueue Topology** (Topology CRD)
   <https://kueue.sigs.k8s.io/docs/concepts/topology/>

4. **Kueue Topology Aware Scheduling**
   <https://kueue.sigs.k8s.io/docs/concepts/topology_aware_scheduling/>

5. **Kueue GitHub README** — install version は最新を確認
   <https://github.com/kubernetes-sigs/kueue>

6. **Kueue for AI: The Power of Atomic Admission & Topology Awareness** (Medium, 2026)
   <https://medium.com/google-cloud/kueue-for-ai-the-power-of-atomic-admission-topology-awareness-59c2fd1f86ed>
   - 解説記事、TAS の使い方の感覚を掴むのに良い

#### Phase 2: Design Challenge — Kueue 挙動の事前予測 (1h)

`docs/predictions/kueue-behavior.md` に **install 前に** 5 シナリオの予測を書く。各シナリオ:
- 単一 Job の admission (quota 内 / 超え)
- 並行 Job (合計が quota 超え)
- Borrowing (Cohort 経由)
- 自作 plugin との共存 (gang は誰が責任を持つ?)
- TAS (自作 TopologyAware と二重に効くか?)

予測を書いてから実機検証 (Phase 7)。

#### Phase 3: 実装に入る前に答えるべき問い (30 min)

これらは parse_events.py / run_experiment.sh の設計用:

**問い 1**: kubectl events のタイムスタンプフィールドは何が信頼できるか? `lastTimestamp` / `eventTime` / `firstTimestamp` のどれを使うか? Kubernetes バージョンで違う可能性は?

**問い 2**: 「Pod の scheduling latency」を「Pod 作成時刻 → NodeName が bind された時刻」で測るか、「Pending event → Scheduled event」で測るか? Kueue を経由する場合、それぞれは何を含む / 含まないか?

**問い 3**: 4 scheduler (default, topogang, kueue, volcano) を 5 trial 回す harness で、各 trial 間にどんな cleanup が必要か? Job を残したまま次の trial に行くと何が起きるか?

#### Phase 4: 実装中に参照すべき資料

- `kubectl get events` の JSON 出力構造 — 試しに 1 度実行して確認
- **Kueue install マニフェストの最新 URL**: <https://github.com/kubernetes-sigs/kueue/releases>
- jq の query syntax: <https://jqlang.github.io/jq/manual/>
- bash の `set -euo pipefail` の意味

#### Phase 5: Setup — Kueue install + 設定 (1h)

**Lift OK**: Kueue install マニフェスト
```bash
# Kueue 公式 release ページで最新 version を確認 (v0.17.x など)
VERSION=v0.17.1
kubectl apply --server-side -f \
  "https://github.com/kubernetes-sigs/kueue/releases/download/${VERSION}/manifests.yaml"
```

**自分で書く**:
- `manifests/kueue/topology.yaml` (Topology CRD: rack 階層)
- `manifests/kueue/resource-flavors.yaml`
- `manifests/kueue/cluster-queue.yaml`
- `manifests/kueue/local-queue.yaml`

**ヒント**: Kueue 公式 docs の YAML 例があるが、kind cluster の自分の node label に合わせて書き換える必要がある。ただ写すだけだと動かない。

#### Phase 6: Implementation (3h) ★

ツール類なのでテンプレート程度の skeleton OK。中身は自分で詰める。

##### parse_events.py (1.5h)

```python
"""
parse_events.py: kubectl get events と get pods を combine して
PodGroup の timeline を CSV で出力する。
"""
import subprocess, json, sys, csv
# ... 以下は自分で書く

def fetch_events(namespace="default"):
    # subprocess で kubectl get events
    pass

def fetch_pods(label_selector, namespace="default"):
    # subprocess で kubectl get pods
    pass

def extract_timeline(pods, events):
    # 各 Pod について Scheduled / Started などの event timestamp を抽出
    # rows のリストを返す
    pass

def main():
    # CLI 引数を取って CSV を stdout に
    pass
```

Phase 3 問い 1 (timestamp field) と問い 2 (latency 定義) に自分が出した答えを反映。

##### run_experiment.sh (1h)

```bash
#!/bin/bash
# run_experiment.sh: 4 scheduler × N trial で同じワークロードを実行
set -euo pipefail
N_TRIALS=${1:-5}
RESULTS_DIR=docs/experiments/runs/$(date +%Y%m%d-%H%M%S)
mkdir -p "${RESULTS_DIR}"

run_one_trial() {
    local scheduler=$1
    local trial=$2
    local manifest=$3
    # 既存 Job を削除 (Phase 3 問い 3)
    # apply
    # 全 Pod が Running になるまで待つ (timeout 付き)
    # observe.py を呼んで結果を保存
    # elapsed time を CSV に追記
    pass  # 中身は自分で書く
}

# 各 scheduler / trial をループ
for trial in $(seq 1 $N_TRIALS); do
    run_one_trial default $trial examples/workload-default.yaml
    run_one_trial topogang $trial examples/workload-topogang.yaml
    run_one_trial kueue    $trial examples/workload-kueue.yaml
done
```

##### workload manifests (30 min)
- `examples/workload-default.yaml`: schedulerName 指定なし
- `examples/workload-topogang.yaml`: schedulerName: topogang + PodGroup
- `examples/workload-kueue.yaml`: suspend: true + queue label

#### Phase 7: Verification — 受け入れ条件

- [ ] `bash tools/run_experiment.sh 3` で 3 trial × 3 scheduler = 9 試行が走る
- [ ] `docs/experiments/runs/.../elapsed.csv` に completion time が記録される
- [ ] 各試行の rack_distribution JSON が個別ファイルで残る
- [ ] `docs/predictions/kueue-behavior.md` の 5 シナリオ各々に「実機結果」「振り返り」が追記されている
- [ ] 比較表 (`docs/comparison.md`) が完成

#### Pitfalls (症状ベース)

- Kueue で投げた Job が即座に走る → `suspend: true` 忘れ
- Kueue で Job が永遠に Pending → ResourceFlavor の nodeLabels が node の実際の label と一致しているか確認
- run_experiment.sh が前回の Pod を引きずる → Job 削除後の sleep / wait が不十分
- parse_events.py の timestamp が空 → Phase 3 問い 1 の答えが間違っていた、別フィールドへフォールバック

#### Self-check questions

1. 「Kueue は scheduler を置き換えない」とはどういう意味か? Kueue が admission した後、Pod は誰が node に bind しているのか?
2. ClusterQueue の `nominalQuota` を超える要求が来たら何が起きるか? `borrowingLimit` は誰の使ってない quota を借りるのか?
3. 5 つの予測のうち、何個外したか? 一番大きく外したシナリオは何か? その原因は?
4. 自作 plugin と Kueue は co-exist できるか?

#### Stretch goals

- Kueue Fair Sharing を有効化、2 LocalQueue から並行 submit
- MultiKueue — もう 1 つの kind cluster を立てる

---

### Day 6: Volcano / Workload API + 3 戦略の比較図

#### Goal
- Volcano が install され、`vcjob` を 1 つ以上 submit
- Volcano の gang scheduling と自作 Permit plugin の設計差をノート
- 3 戦略 (in-tree / queue layer / 全置換) の比較表
- `docs/design/ideal-scheduler.md` が完成
- `tools/plot.py` で比較図 (latency, locality, completion time) が PNG 出力

#### Why this matters for LT46
- レポート §4 のクライマックス。3 戦略を 1 図で見せられる

#### Phase 1: 一次資料 (1h)

1. **Volcano README** — install 手順、scheduler の置き換え方
   <https://github.com/volcano-sh/volcano>

2. **Volcano Architecture**
   <https://volcano.sh/en/docs/architecture/>

3. **KEP-4671: Gang Scheduling using Workload Object** (再読、3 戦略の比較視点で)
   <https://github.com/kubernetes/enhancements/blob/master/keps/sig-scheduling/4671-gang-scheduling/README.md>

4. **KEP-5710: Workload-aware Preemption** (1.36 で alpha、Workload API の進化方向)
   <https://github.com/kubernetes/enhancements/blob/master/keps/sig-scheduling/5710-workload-aware-preemption/README.md>

5. **JobSet README**
   <https://github.com/kubernetes-sigs/jobset>

6. **YuniKorn の K8s Scheduler Plugin design** (4 番目の戦略例として)
   <https://yunikorn.apache.org/docs/design/scheduler_plugin/>

#### Phase 2: Design Challenge — 自分の "理想 scheduler" 像 (1.5h)

`docs/design/ideal-scheduler.md` を書く。

**最重要セクション §6**: あさんの研究 (BlueField + 動的 learned index による MoE routing) と Kubernetes scheduler の階層分担。
- scheduler が担当する placement 層
- DPU が担当する flow steering 層
- 両者の API 境界

#### Phase 3: 実装に入る前に答えるべき問い (30 min, plot.py 用)

**問い 1**: 提出レポートに載せる図は最大 3 枚。どの 3 つを選ぶか?
- 候補: completion time の box plot, locality 比率, scheduling latency 累積分布, gang あり/なしの timeline, 3 戦略の concept 図 など
- 選択基準: 「この図が無いと文章が成立しない」もの優先

**問い 2**: matplotlib の出力 DPI / フォントサイズはどうするか? A4 2 ページに収まるサイズで読めるか?

#### Phase 4: 実装中に参照すべき資料

- matplotlib pyplot reference: <https://matplotlib.org/stable/api/pyplot_summary.html>
- box plot tutorial: matplotlib gallery
- KubeCon EU 2025 "A Comparative Analysis of Kueue, Volcano and YuniKorn" のスライド (公開されていれば、3 戦略比較の図として参考)

#### Phase 5: Setup — Volcano install (1h)

**Lift OK**: Volcano install マニフェスト (公式の development.yaml をそのまま)

**自分で書く**: `examples/workload-volcano.yaml` (vcjob)

#### Phase 6: Implementation (2-3h) ★

##### plot.py (2-3h)
ツール類なので skeleton OK:

```python
"""
plot.py: run_experiment.sh の出力 CSV と JSON を読んで図を生成
"""
import os, json, sys, glob, csv
import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt

# 関数: load_elapsed, load_locality, plot_completion_time, plot_locality, ...
# 中身は自分で書く

def main():
    # CLI 引数で results dir を取り
    # 図を 2-3 枚生成
    pass
```

#### Phase 7: Verification — 受け入れ条件

- [ ] `python3 tools/plot.py docs/experiments/runs/<latest>` で 2-3 個の PNG が出る
- [ ] PNG が docs/experiments/plots/ に保存される
- [ ] 各図にタイトル、軸ラベル、必要なら凡例
- [ ] 4 scheduler のデータが揃う (default, topogang, kueue, volcano)
- [ ] `docs/design/three-strategies.md` に比較表が完成
- [ ] `docs/design/ideal-scheduler.md` の §6 (BlueField/MoE 接続) が抽象論で終わっていない

#### Pitfalls (症状ベース)

- Volcano が default scheduler と競合 → `schedulerName: volcano` 指定 Pod のみ
- Volcano CRD と scheduler-plugins coscheduling CRD が衝突 → apiGroup が違う、namespace 切り分け
- plot.py が空図を出す → load_* 関数が空リストを返している、glob pattern を確認
- 図が見にくい → DPI を上げる (120-150)、フォントサイズ大きく

#### Self-check questions

1. なぜ Volcano は kube-scheduler を replace するのか? in-scheduler 拡張で十分ではない理由?
2. なぜ Kueue は scheduler に手を加えないのか? それで何が嬉しい?
3. KEP-4671 が in-tree でやろうとしている理由は? (Volcano / Kueue の経験を踏まえて)
4. 自分の ideal-scheduler.md を 3 分で口頭説明できるか? §6 (MoE/DPU 接続) が抽象論で終わっていないか?

#### Stretch goals

- YuniKorn install — 4 戦略目を加える
- NVIDIA KAI Scheduler を eyeball: <https://github.com/NVIDIA/KAI-Scheduler>
- JobSet + Kueue + 自作 scheduler の 3 段重ね

---

### Day 7: 計測 fix-up + 4 ページ tech note 完成

#### Goal
- 計測スクリプトと図 3 枚以上が `docs/experiments/` 以下にある
- 4 ページの technical note が完成
- Design Doc 群がレポート §3 の素材として整理されている
- BlueField / MoE への接続が 1 段落

#### Why this matters for LT46
- 提出物がないと意味がない。執筆を最初から確保

#### Phase 1: 計測 fix-up (2h)
- Day 5-6 の計測でバグや欠損があれば修正
- 試行回数を増やす (5 → 10 など) で統計性向上
- plot を最終版に整える

#### Phase 2: 執筆 (5h)

**§1 はじめに** (0.3 page)
- AI workload と default scheduling の不適合
- workload aware scheduling の登場
- 自分版 spec (Day 1) の要約 1 段落

**§2 Kubernetes scheduler の構造** (0.5 page)
- kube-scheduler と Scheduling Framework
- 各 extension point の役割
- default で出来ない 2 つの典型: gang / topology

**§3 自作 plugin による検証** (1.0 page) — コア
- Design Doc §1〜§5 から動機 / 採用案 / トレードオフを引用
- gang plugin (Permit + Reserve + PreFilter) の概要と挙動差
- topology plugin (3 案比較→ 採用案→ 結果)
- 図 1: gang あり/なしの挙動差
- 図 2: rack locality 比率

**§4 設計戦略の俯瞰** (1.0 page)
- 3 戦略の比較表
- 各戦略の強み/弱み
- KEP-4671 の立ち位置
- Day 5 の予測検証から「Kueue を実機で触って分かったこと」 2 行
- 図 3: 3 戦略の completion time 分布

**§5 ハードウェアとの接続** (0.5 page)
- DRA, NVLink ComputeDomain, MIG, RDMA NIC alignment
- 「scheduling decision に物理特性を持ち込む」流れ
- ideal-scheduler.md §6 を圧縮: BlueField + MoE の placement vs flow steering 階層分担

**§6 まとめと展望** (0.3 page)

#### Phase 3: Final polish (1h)
- proofread, 冗長削減
- Figure caption の確認
- 用語の一貫性
- 参考文献 (KEP, Kueue/Volcano docs)

#### Pitfalls

- 数値を盛らない。N=3 trial の平均が 4.2s なら 4.2 と書く。kind cluster は本番を代表しないことを脚注で
- 「自作したから自作が一番」になりがち → 正直に書く
- Design Doc を「コピペすれば §3 が埋まる」と過信しない → レポート向けに圧縮する作業が必ず要る
- 自分のコード行数を勘定して footnote に書くと reviewer に響く

#### Self-check (deliverable check)

- [ ] 自作 scheduler 動作デモが録画 / GIF で残っている
- [ ] 比較表が 1 枚にまとまっている
- [ ] 4 ページの technical note PDF / md が完成
- [ ] BlueField / MoE への接続が 1 段落以上
- [ ] GitHub repo が public で README に再現手順あり
- [ ] `docs/design/` に 6 ファイル (my-spec, api-schema, gang-scheduling, topology-aware, ideal-scheduler, three-strategies)
- [ ] 各 Design Doc の §6 (既存実装との比較) が埋まっている
- [ ] `pkg/coscheduling/` と `pkg/topologyaware/` のコード行数を記録 (面接素材)
- [ ] `tools/` 配下の 4 スクリプトが動く
- [ ] 3 つの図が PNG で揃う

---

## 6. 提出レポートに使うべきキーフレーズ

LT46 reviewer (PFN 計算基盤チーム) が「ちゃんと考えた」と感じるキーワード群:

- **Scheduling Framework の extension points** (PreFilter / Filter / Score / Reserve / Permit / Bind)
- **Permit plugin による all-or-nothing admission**
- **PodGroup の minMember**
- **Reserve 段階のスロット保持と Permit Wait の併用**
- **Topology label による rack/block 階層表現**
- **Dynamic Resource Allocation (DRA) の ResourceClaim/ResourceSlice モデル**
- **NVLink ComputeDomain, MIG / Time-Slicing**
- **Workload API (KEP-4671) と Workload/PodGroup の役割分離**
- **Kueue の二段階 admission**
- **ClusterQueue / LocalQueue / Cohort, ResourceFlavor, Topology**
- **Volcano の scheduler 全置換戦略**
- **inter-node bandwidth-sensitive な all-to-all (MoE)**

---

## 7. 自分の研究 (BlueField / MoE) への接続のヒント

§5 で書く 1 段落のための材料 (中身はあさんの判断):

- 自作 scheduler が今 plugin で表現しているのは「rack 局所性」など static な topology
- AI 推論 / 学習の現実は **dynamic** — load の変化、token 分布の変化、expert 利用率の変化で最適配置が動く
- 自分の研究 (BlueField DPU 上の動的 learned index による MoE routing) は、まさにこの **dynamic な配置最適化を、データプレーンに近い場所で行う** 試み
- Kubernetes scheduler から見ると **「scheduler が cluster-level で粗く配置し、DPU が node-pair-level で細かくルーティングを最適化する」階層分担** として位置付けられる
- Kubernetes の DRA / Workload API と、SmartNIC 上の動的 routing は **互いに補完的** — 前者は **placement** を、後者は **flow steering** を担当
- このスタックの完成形が PFN の「次世代計算インフラの計測・制御技術」の一断面

---

## 8. もし詰まったら (症状 → 確認すべきこと)

| Day | 症状 | 確認すべきこと |
|-----|------|---------------|
| 1 | observe.py が空を返す | kubectl の selector 構文、Pod の label が template に付いているか |
| 1 | 自分版 spec が書けない | KEP §Motivation だけ読み、Proposal は読まない |
| 2 | 自作 scheduler が起動しない | RBAC, image load, leaderElection: false |
| 2 | Nodemask の Args が届かない | scheduler-plugins/pkg/capacityscheduling の Args デコードを参考 |
| 3 | Design Doc を書く時間が足りない | §2 を 2 案だけにして §3-§5 を厚く |
| 3 | gang が動かない | Permit が呼ばれているか log で確認、Reserve の有無、PodGroup label |
| 3 | sync 関連で dead lock | lock 範囲を確認、callback の中で別 lock を取らないか |
| 4 | 3 案比較が書けない | 単純案と高度案の 2 案でも OK、各案の弱点を真面目に書く |
| 4 | score が反映されない | weight, NormalizeScore, 他 plugin の dominant score |
| 5 | 予測が全部当たる | シナリオ 4 (自作 + Kueue 共存) を真面目に予測 |
| 5 | parse_events.py が空 | event timestamp の field 名 |
| 6 | 時間が無い | Volcano は install + 1 vcjob のみ、ideal-scheduler.md と plot.py に集中 |
| 7 | 書けない | 図を先に並べて figure-driven writing。Design Doc から引用 |

---

## 9. 締め切り感覚 (v4)

| Day | 終了時刻 | 時間 (h) | うち実装 (h) | 累積 (h) |
|-----|---------|---------|--------------|---------|
| 1 | 23:00 | 7 | 2 | 7 |
| 2 | 24:00 | 9 | 4 | 16 |
| 3 | 25:00 | 12 | 6 | 28 |
| 4 | 25:00 | 12 | 6 | 40 |
| 5 | 23:00 | 9 | 3 | 49 |
| 6 | 23:00 | 9 | 3 | 58 |
| 7 | 23:00 | 8 | — (執筆) | 66 |

合計 ~66 h。Day 3-4 が山 (各 12h)。**週末を Day 3-4 に当てる調整を強く推奨**。

---

## 10. 終わりに

このプロジェクトは「Kubernetes scheduling を理解する」よりも一段深い目的がある — **「自分の研究 (DPU, MoE, 動的 routing) を、cluster-level の制御階層の中に位置付ける視点を持つ」**こと。

v4 で意識的にコード骨格を消したのは、**実装の中身まで自分で考えた跡を残す** ためです。Design Doc + git log + コード本体が三位一体で「私はこう設計し、こう実装した」を物語る状態が、面接 30 分を支配する材料になる。

詰まったら相談してください。エラーメッセージか log を見せてもらえれば、どこを疑うかの hint を返します。コードは出しません — 出した瞬間にこのプロジェクトの価値が消えるので。
