## コンテナ内のリソースを指定するような.yamlファイルの書き方
- container_resources_sample.yaml
- なお、コンテナのリソースは主にcpu,memory使用率をモニタリングしておき、p95使用率を割り当てるのがよくあるプラクティス

## Pod単位でもリソースを指定できる
- pod_resources_sample.yaml

## cpuやメモリ資源を実際にモニタリングする
- 以下のようなコンテナ構造になっている
```
┌─────────── アプリ (コンテナ内) ───────────┐
│                                            │
│  プロセスの CPU / Memory 使用量            │
│           │                                │
│           ▼                                │
│  cgroups (Linux カーネル)                  │
│           │                                │
│           ▼                                │
│  cAdvisor (kubelet に組み込み)             │
│           │                                │
│           ▼                                │
│  metrics-server (クラスタ全体で集約)        │
│           │                                │
│           ▼                                │
│  kubectl top / HPA / ダッシュボード        │
│                                            │
└────────────────────────────────────────────┘

┌─ Node 1 ─┐  ┌─ Node 2 ─┐  ┌─ Node 3 ─┐
│ kubelet  │  │ kubelet  │  │ kubelet  │
│ +cAdvisor│  │ +cAdvisor│  │ +cAdvisor│
└────┬─────┘  └────┬─────┘  └────┬─────┘
     │             │             │
     │ 15秒ごとに pull            │
     └─────────┬───┴─────────────┘
               ▼
      ┌─────────────────┐
      │ metrics-server  │ ← 集約役
      └────────┬────────┘
               │
               ▼
      kubectl top / HPA

┌─── モニタリングで何を見て、何を決めるか ───┐
│                                            │
│  短期 (リアルタイム): metrics-server        │
│    → kubectl top / HPA                     │
│                                            │
│  長期 (履歴):         Prometheus 等         │
│    → right-sizing の根拠データ              │
│                                            │
│  right-sizing の定石:                      │
│    request = P95 程度                      │
│    limit   = 最大値 + 余裕 (or 無制限)      │
│                                            │
└────────────────────────────────────────────┘
```

## GPUなどの外部デバイスのリソース割り当て
```yaml
# Pod 側の書き方
resources:
  requests:
    nvidia.com/gpu: 2     # GPU 2 枚欲しい
  limits:
    nvidia.com/gpu: 2
```
- モニタリングは次の流れで行われる
```
Device Plugin (実務ではこちら)
────────────────
Node に Device Plugin という常駐プロセスを動かす
  ↓
Plugin が GPU などを検出して kubelet に報告
  ↓
kubelet が Node capacity に自動登録

NVIDIA GPU なら NVIDIA が提供する k8s-device-plugin を使う
```
- 制約
    1. GPUなどの外部デバイスは1,2,...という整数値でrequest,limitを宣言する必要がある
    2. resources == limitとしなければいけない

## Pod templates
- PodはPodTemplateをもとに実際のPodを作成する(宣言的設定)。つまり、Podのひな型をworkload resourceに埋め込むのが運用上の標準
- ユーザーは「どういう Pod をどういう方針で運用したいか」を宣言し、細かい実体管理は controller に任せる
- controller は「何をあるべき状態にするか」を担当、kubelet は「割り当てられた Pod を実行すること」を担当
- pod_template_sample.yamlに例

## Security Context
- security_context_sample.yamlに例

## Service
- Serviceは「Pod群を安定した一つのネットワーク入り口として抽象化する仕組み」
- service_example.yaml, service_port_example.yaml(ポートに名称をつける)に例
- Serviceを定義しないときは自分でEndpointSliceというエンドポイントを自前定義(endpointslice_example.yaml)
- Service typeは次の通り
```
ClusterIP
   └─ クラスター内だけで到達可能(クラスター内部専用の仮想IP)

NodePort
   └─ ClusterIP + 各Nodeの固定ポートで到達可能(NodeIP:NodePort でクラスター外から届かせる)

LoadBalancer
   └─ NodePort(+通常はClusterIP) + 外部LBで公開

ExternalName
   └─ 転送ではなく DNS 名の別名化(Service 名を別の DNS 名へ CNAME 的にマップする)
```

- Headless Service は .spec.clusterIP: None を明示し、仮想IPを割り当てない Serviceのこと
    - クライアントは DNS で個々の Pod IP を受け取り、どの Pod に直接つなぐかを自分で選べる
    ```
    通常Service:
    client -> Service VIP -> kube-proxy -> backend Pod

    Headless Service:
    client -> DNSでPod一覧取得 -> backend Podへ直接接続
    ```

- 「Pod から Service をどう見つけるか」は2通りの方法がある
    1. 環境変数
    2. DNS: KubernetesがCoreDNSなどのcluster-aware DNSを入れ、ServiceごとにDNSレコードを作成する

## Probes
- Kubernetesが「このPodのコンテナは生きている?/使える状態?」かどうかを定期的にチェックする仕組み
1. *Liveness Probe*: コンテナが生きているかを調べて、失敗するとコンテナを再起動する
2. *Readiness Probe*: コンテナがリクエストを受けれる状態かを調べて、失敗するとServiceのendpointsから外す
3. *Startup Probe*: 初期化が終了したかを調べて、失敗するとLiveness, Readinessを開始しない

## Deployment
- Podを直接バラバラに管理するのではなく、Deploymentで"望ましい状態"を宣言し、Kubernetesにその状態を維持・更新させる
- deployment_example.yaml
```
[Deployment]
    │ 望ましい状態を宣言
    ▼
[ReplicaSet]
    │ 必要な数を維持
    ▼
[Pods]
    │ 実際にコンテナが動く
    ▼
[Containers]
```
- Rolling Update: 動いているサービスを止めずに、Podを少しずつ新しいバージョンに入れ替えていく更新方式(type: RollingUPdateで指定)
    - strategy: という項目で指定するパラメータは以下の通り
        1. MaxUnavailable: 更新中に利用不可になっていいPodの最大数
        2. maxSurge: 更新中にreplicasを超えて一時的に増やしていいPodの最大数

- 重要な点
    1. Deployment は Pod を直接管理するというより ReplicaSet を管理する。
    2. 更新の本体は `.spec.template` の変更であり、新しい ReplicaSet が作られる。
    3. スケール変更は revision を増やさない。
    4. `selector` は必須で immutable なので、最初に慎重に設計する必要がある。
    5. `pod-template-hash` は自動管理ラベルなので手動で触らない。
    6. status 条件を見ると、進行中・完了・失敗を判別できる。
    7. `revisionHistoryLimit` はロールバック可能性と履歴肥大化のバランスを取る設定である。

## ConfigMap
- ConfigMapは、機密でない設定情報をkey-valueで保存するKubernetesのAPIオブジェクトのこと
- "*アプリ本体のイメージ*"と"*環境ごとの差分設定*"を分離することが目標(設定をコードから分離すると、環境が変化してもイメージやコードを作り直す必要がなくなる)
- ConfigMapの例
    1. 環境変数
        - 値が短く、アプリが環境変数を直接読む設計の時
        - 例としてenv_configmap_single.yaml、env_configmap_all.yaml
    2. volumeとして利用
        - 複数行の設定があり、設定ファイル形式で渡したい時
        - 例としてconfig_volume.yaml
- ConfigMapを環境変数として利用すると、Podが起動した時点での値が入り、ConfigMapを更新しても既存Podの環境変数はそのまま。一方、volumeの場合はConfigMap更新後に/config/ファイルとしてfile内容が更新される。

## Secret
- Secretは"*機密情報をPodやコンテナイメージに直接書かず、Kubernetesの
Secretオブジェクトとして分離して扱う*"ためにある(秘密とコードは分離する)
- ConfigMap(非機密な設定情報) ⇔ Secret(機密データ)
- secret.yamlに例がある