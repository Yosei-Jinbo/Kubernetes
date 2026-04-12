# Day 3: Affinity / Taints / Topology Spread Constraints

## 目的

Kubernetesスケジューラの「入口」となる5つのAPIを手で動かして体得する。

- nodeSelector
- nodeAffinity (required / preferred)
- taints / tolerations
- podAffinity / podAntiAffinity
- topologySpreadConstraints

## 到達目標

このDayが終わったとき、以下を自分の言葉で説明できる状態になる。

1. Podがどのノードに配置されるかは、誰が・いつ・何を見て決めているか
2. 上記5つのAPIの違いと使い分け
3. `requiredDuringScheduling` と `preferredDuringScheduling` の違い、および `IgnoredDuringExecution` という命名の意味
4. スケジューリングに失敗したPodがどういう状態になり、`kubectl describe` のどこを見れば理由が分かるか

## 所要時間

合計 4〜5時間。1つの実験で30分以上ハマったら飛ばし、`actual.md` に "TODO: 後日再挑戦" と記録して次へ進む。

---

## 事前準備（30分）

### クラスタ再構築

`kind-config.yaml` を以下の内容に更新する。worker 4ノード構成にすることが重要（3ノードだとtopology spreadの実験で味が出ない）。

```yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
  - role: control-plane
  - role: worker
    labels:
      zone: a
      gpu-type: a100
      tier: high-perf
  - role: worker
    labels:
      zone: a
      gpu-type: v100
      tier: standard
  - role: worker
    labels:
      zone: b
      gpu-type: v100
      tier: standard
  - role: worker
    labels:
      zone: b
      tier: low-perf
```

クラスタ起動後、worker4にtaintを付ける。

```bash
kind delete cluster
kind create cluster --config kind-config.yaml
kubectl get nodes --show-labels
kubectl taint nodes <worker4の名前> dedicated=experimental:NoSchedule
```

### ディレクトリ構成
day03/
├── README.md                          # このファイル
├── kind-config.yaml
├── scheduling-constraints-cheatsheet.md
├── troubleshooting.md
├── scripts/
│   └── check.sh
└── experiments/
├── exp01-nodeSelector/
├── exp02-nodeAffinity-required/
├── exp03-nodeAffinity-preferred/
├── exp04-taint-toleration/
├── exp05-podAntiAffinity/
├── exp06-topologySpread/
└── exp07-composite-scenario/

各実験ディレクトリに以下を置く。

- `pod.yaml` または `deployment.yaml`
- `expected.md`: 実験前に書く予想（3〜5行）
- `actual.md`: 実行結果と考察（3〜10行）

**予想を先に書くのが重要。** 当てに行くゲームではなく、自分の理解とAPIの実挙動のズレを発見するための仕組み。

---

## 実験タスク

### 実験1: nodeSelector の基本（30分） OK!

**やること**

Pod を1個、`gpu-type: a100` のノードにだけ配置されるよう nodeSelector で指定。

**確認ポイント**

- worker1 にスケジュールされたか
- worker1 のリソースを意図的に埋めて（`resources.requests.cpu` を大きく）、もう1個同じPodを作ったらどうなるか → Pendingになるはず
- なぜ他のworkerに行かないかを `kubectl describe pod` で確認
- 結論: nodeSelectorは「条件を満たすノードのみ」を候補にする。リソース不足でも他に行かない

---

### 実験2: nodeAffinity required の基本（30分の前半） OK!

**やること**

実験1と同じことを `requiredDuringSchedulingIgnoredDuringExecution` で書く。`matchExpressions` で `In` 演算子を使う。

**確認ポイント**

- nodeSelectorとの挙動の違いはあるか（基本ない、表現力だけが違う）
- `In: [a100, v100]` のように複数指定するとどうなるか
- `NotIn` を使うと？

---

### 実験3: nodeAffinity preferred の挙動（30分）OK!!

**やること**

`preferredDuringSchedulingIgnoredDuringExecution` で「`tier: high-perf` を weight 100 で優先、`tier: standard` を weight 10 で優先」と書く。Podを5個作って分布を見る。

**確認ポイント**

- 5個全部 high-perf に行くか、それとも分散するか
- worker1 (high-perf) のリソースを埋めた状態でPodを作ると、どこに行くか
- preferred は「行けたら行く、行けなくても他で妥協する」というソフト制約だと体感する

---

### 実験4: Taint と Toleration（40分） OK!

**やること**

事前準備で worker4 に `dedicated=experimental:NoSchedule` のtaintを付けた。これを toleration で許可するPodを作る。

**確認ポイント**

- toleration なしのPodは worker4 に絶対に行かない（`describe pod` のEventsで確認）
- toleration ありのPodは行ける**可能性がある**。ただしtolerationは「許可」であって「優先」ではない
- toleration付きPodを5個作って分布を見る → worker1〜3 にも普通に配置される
- worker4 専用にしたいなら、toleration + nodeAffinity の組み合わせが必要、という結論を自分で導く

---

### 実験5: podAntiAffinity で別ノードに散らす（40分） OK!

**やること**

`app: web` ラベルを持つPodを3個、`requiredDuringSchedulingIgnoredDuringExecution` の podAntiAffinity で「同じノードに同じappを置かない」と指定。`topologyKey: kubernetes.io/hostname`。

**確認ポイント**

- 3個が3つの異なるworkerに散ったか（worker4にはtoleration無いので行かない）
- replicas を 5 に増やすとどうなる → 4ノードしかないので Pending が出るはず
- これが podAntiAffinity required の罠
- preferred に変えて 5 にすると → 配置はされるが偏る。Eventsで確認

---

### 実験6: topologySpreadConstraints で均等分散（50分・Day 3の山場） OK

**やること**

`app: balanced` ラベルを持つPodを6個、`topologyKey: zone` で `maxSkew: 1` を設定。zone aとzone bにそれぞれworker2台ずつあるので、均等に3個ずつ配置されることを期待。

**確認ポイント**

- zone a に 3個、zone b に 3個になったか
- `whenUnsatisfiable: DoNotSchedule` と `ScheduleAnyway` で挙動がどう違うか
- さらに `topologyKey: kubernetes.io/hostname` に変えて maxSkew=1 にすると、4ノードに6個をどう配置するか（2,2,1,1のはず）

挑戦タスク「Pod 6個を2ノードに均等分散する」はこの最初の設定で達成。

---

### 実験7: 複合シナリオ（卒論との接続を意識）（30分）OK

**やること**

「GPUがa100のノードを優先しつつ、同じ学習ジョブのPodは異なるzoneに散らしたい」という、分散学習っぽいシナリオを1つYAMLで表現する。

**確認ポイント**

- 自分の設計判断を `expected.md` に書く（なぜこの組み合わせか、どのworkerに何個配置されると予想するか）
- 実行結果との差分を考察
- これは面接で「k8sでGPU分散学習を表現するならどう書く？」と訊かれた時の回答素材になる

---

## 成果物の詳細

### `day03/README.md`

冒頭に書く内容:

- このDayの目的（1段落）
- 学んだことの要約（5項目程度の箇条書き）
- 実験一覧へのリンク
- 詰まった点とその解決（採用担当が見るのは正直ここ。ハマりログは誠実さの証拠）

### `day03/scheduling-constraints-cheatsheet.md`

以下の表を埋める。

| API | 種類 | 効果 | 失敗時の挙動 | 主な用途 |
|---|---|---|---|---|
| nodeSelector | hard | … | … | … |
| nodeAffinity required | hard | … | … | … |
| nodeAffinity preferred | soft | … | … | … |
| taints/tolerations | hard | … | … | … |
| podAffinity required | hard | … | … | … |
| podAntiAffinity required | hard | … | … | … |
| topologySpreadConstraints (DoNotSchedule) | hard | … | … | … |
| topologySpreadConstraints (ScheduleAnyway) | soft | … | … | … |

表の下に、「同じことを違うAPIで書ける場合の選び方」のメモを書く。

- 単純なノード絞り込みは nodeSelector で十分か、nodeAffinityを使うべきか
- 「Podを散らす」を podAntiAffinity と topologySpreadConstraints のどちらで書くべきか
- それぞれの選択の根拠（公式ドキュメントが何を推奨しているか）

これは Day 4以降のスケジューラ深掘りの土台になり、面接でもそのまま使える。

### `day03/troubleshooting.md`

実験中に出会ったエラーと解決方法を残す。

- Pending のままになる原因の切り分けフロー
- `describe node` で Allocatable を確認する手順
- Eventsに出るスケジューラのメッセージの読み方

1日後の自分も助かるし、ブログ記事の素材にもなる。

### `day03/scripts/check.sh`

各実験のapply→確認→cleanupを自動化する。

```bash
#!/bin/bash
set -e
EXP=$1
kubectl apply -f experiments/$EXP/
sleep 5
kubectl get pods -o wide -l "experiment=$EXP"
echo "---"
kubectl describe pods -l "experiment=$EXP" | grep -A 5 Events
kubectl delete -f experiments/$EXP/
```

各Podに `experiment: exp01` のようなラベルを付けておくと、まとめて操作できる。この種の自動化スクリプトを書くこと自体が、面接で「再現性を意識している」評価につながる。

---

## 時間配分の目安

| タスク | 時間 |
|---|---|
| 事前準備（クラスタ再構築・ラベル/taint付け） | 30分 |
| 実験1〜2（nodeSelector / nodeAffinity required） | 30分 |
| 実験3（preferred の挙動を観察） | 30分 |
| 実験4（taint/toleration、罠を体感） | 40分 |
| 実験5（podAntiAffinity の罠を体感） | 40分 |
| 実験6（topologySpread、Day 3の山場） | 50分 |
| 実験7（複合シナリオ） | 30分 |
| cheatsheet と README 執筆 | 40分 |
| **合計** | **約4時間40分** |