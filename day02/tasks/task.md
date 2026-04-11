# Day 02: Deployment / Service / ConfigMap / Secret / Resource Packing

## 重心
本丸は **タスク6（リソース詰め込み計算と実証）**。Day3 以降のスケジューラ理解の土台。他はその足場。Sidecar / Headless / NodePort はスケジューラ研究に効かないので Day2 では扱わない。

## 学習ゴール
- Pod / Deployment / Service / ConfigMap / Secret の関係を自分の言葉で説明
- **requests/limits を設定した Pod の配置を計算で予測し実測と突き合わせる**
- `maxSurge` / `maxUnavailable` の意味を実演
- Liveness / Readiness の違いを実演
- ConfigMap を env と file mount の両方式で使う

## 前提
kind 3ノード（control-plane 1 + worker 2）、`kubectl` / `docker` / `kind` 稼働、`day02/` 作成済み。

## ディレクトリ構成
```
day02/
├── README.md
├── app/            main.go, Dockerfile, go.mod   # Go 推奨
├── manifests/      01-ns / 02-cm / 03-secret / 04-deploy-v1 / 05-deploy-v2 / 06-deploy-small / 07-svc-clusterip
├── scripts/        build-and-load.sh / apply-all.sh / cleanup.sh
└── experiments/    01-rolling-update.md / 02-probes.md / 03-resource-packing.md
```

## タスク

### 1. 自作アプリ（Go 推奨）
エンドポイント: `/`(version+host), `/env`(CM env), `/config`(CM file), `/secret`(secret file), `/healthz`(常200), `/ready`(起動後10秒は503), `/crash`(os.Exit(1))。`APP_VERSION` 環境変数でバージョン切替。

### 2. ConfigMap 2方式
1つの CM に `APP_GREETING`(env 注入) と `app.conf`(file mount to `/etc/app/app.conf`)。Deployment で `env.valueFrom.configMapKeyRef` と `volumes`+`volumeMounts` の両方を書く。

### 3. Secret（file mount のみ）
`api-token` を `/etc/secrets/api-token` にマウント。`kubectl create secret generic ... --dry-run=client -o yaml` で生成して保存。env 注入は CM で経験済みなので不要。

### 4. Rolling Update（1パターンのみ）
v1(replicas=6, maxSurge=2, maxUnavailable=1) → v2 を apply。別端末で `while true; do curl ... ; sleep 0.2; done` を回し、v1/v2 混在の推移を記録。`kubectl rollout status/history/undo` を実行。
→ `experiments/01-rolling-update.md`: レスポンス推移ログ、maxSurge/maxUnavailable の意味を自分の言葉で。**3パターン実測は不要**。

### 5. Probes
Liveness `/healthz`(initialDelay=5, period=5), Readiness `/ready`(initialDelay=2, period=2, failureThreshold=3)。
→ `experiments/02-probes.md`: 起動直後 READY 0/1→1/1 の推移、その間 curl が通らないこと、`/crash` で RESTARTS が増えること、**Readiness/Liveness が無いと何が壊れるか考察**。

### 6. リソース詰め込み（Day2 の目玉、必ず完了）
1. `kubectl describe node` で各 worker の `Allocatable` を記録
2. v1 に `requests: cpu=300m, mem=256Mi / limits: 500m, 512Mi` 設定
3. 理論上の詰め込み数を式で書く（kube-system の消費も考慮）
4. replicas を 3→6→10→20 と増やし Pending 出現点を観察
5. Pending Pod を `kubectl describe` して `FailedScheduling` / `Insufficient cpu` を記録
6. `deployment-small.yaml`(requests 半分=150m/128Mi) を作って比較
7. `experiments/03-resource-packing.md` に下記表を作成:

```
| replicas | deployment | Running | Pending | 失敗理由 |
```

**考察（必須、500字以上）**: 計算値と実測値のズレの理由（kube-system、DaemonSet、OS予約）/ requests と limits の違い、スケジューラはどちらを見るか / Day3 以降の `nodeAffinity` や `taints` はこの requests/limits の**上に乗る制約**であるという接続を自分の言葉で。

### 7. Service（最小限）
ClusterIP を1つ、curl で疎通確認。それだけ。

## README 必須セクション
概要 / 環境 / 再現手順 / 各タスク実施ログ / **リソース計算の詳細** / つまずき / 学んだこと7項目以上 / Day3 引き継ぎ

## 完了基準
必須: 再現スクリプト動作 / 7エンドポイント動作 / CM 両方式 / Secret file mount / Rolling update 1パターン記録 / Probes ログ / **タスク6完了** / README 全埋め。
**スキップしてよい**: Sidecar / Headless / NodePort / Rolling update 複数パターン / Secret env 注入。

## 時間配分（合計 4〜5h）
1:50m / 2+3:30m / 4:40m / 5:40m / **6:80m超過可** / 7:15m / README:40m

## 迷ったときの判断基準
> このタスクは Day7〜9（自作プラグイン、gang scheduling）に直接効くか？

効くならやる、効かないなら捨てる。Day2 に溶かして Day5〜7 に届かないのが最悪のシナリオ。
