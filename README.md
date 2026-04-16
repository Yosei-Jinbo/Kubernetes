# Kubernetes
### Kubernetesはコンテナを中心とした管理基盤(オーケストレーション役)

- kindコマンド(Kubernetes IN Docker)によりDockerコンテナをノードとしてKubernetesクラスタをローカルに構築するためのコマンド
```bash
kind create cluster <--name kind-name> #kindの作成
kind get clusters #kindのリスト取得

kubectl cluster-info --context kind-kind #kind-<kind名>の情報を取得(クラスタの情報)
Kubernetes control plane is running at https://127.0.0.1:33729 #Kubernetesのコントロールプレーンがこのアドレスで稼働中
CoreDNS is running at https://127.0.0.1:33729/api/v1/namespaces/kube-system/services/kube-dns:dns/proxy #CoreDNS(Kubernetesクラスタ内のDNSサーバ)がこのアドレスで動作している

To further debug and diagnose cluster problems, use 'kubectl cluster-info dump'.

kubectl get nodes (--show-labels) #クラスタを構成するノードの情報

kubectl label nodes kind-control-plane env=production # ラベルを付ける
kubectl label nodes kind-control-plane env # ラベルを外す

kubectl get pods -A -o wide #すべての名前空間のPod一覧

kubectl describe pod <-n kube-system> <Pod名> #特定のPod内のコンテナ情報, -nはnamespaceを指定するオプションでnamespaceの指定がないならいらない

kind delete cluster #clusterの削除

kind export logs #clusterのログ
```

- 自作のDockerイメージをkindクラスタ上で動作させるワークフローについて
```bash
kind create cluster <--name kind-name> #kindの作成

docker build -t my-custom-image:unique-tag ./my-image-dir #Dockerイメージの作成, my-image-dirはDockerfileのある場所と同じでいいかも
kind load docker-image my-custom-image:unique-tag #ノードの中にイメージをコピー(イメージをkindに読み込む)
kubectl apply -f my-custom-image.yaml #読み込まれたイメージからPodを作り、Podとして動かす
#※ kubectl apply  -f pod.yaml としてPodの設定ファイルを直接読み込んでもPodとして動かすことが可能
#※ kubectl delete -f *.yaml  で削除

docker exec -it <node名> crictl images #ノード内のイメージ一覧
```

- 複数Nodeを持ったkindクラスタを作るには
```bash
kind クラスタを作り直す際に config で指定します 
# kind-config.yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
- role: worker
- role: worker

kind delete cluster
kind create cluster --config kind-config.yaml #新しく--configでconfig設定を読み込んだクラスター
```

## 重要な流れ
[ソースコード]
     ↓ docker build
[Docker イメージ]
     ↓ kind load docker-image
[kind ノード内にイメージ配置]
     ↓ kubectl apply -f xxx.yaml
[Pod として起動] ← ここまでやると kubectl get pods に現れる

- kubectl port-forwardコマンドについて
```bash
kubectl port-forward pod/my-app 8080:8080
```
    - kubectl port-forward: ポートフォワード(ポート転送)を行うコマンド
    - pod/my-app: 転送先のPodを指定
    - 8080:8080: 8080(自分のPC側のポート)⇔8080(Pod側のポート)を接続する
    - これが必要な理由: Podはクラスタ内部でのみ利用できるIPアドレスを持っているが、これはクラスタの外(自分のPCブラウザやcurl)からはアクセス不可
      そこで、このコマンドによりkubectlにより自分のPCとPod内のポートをそれぞれ接続する
```
┌─────────────────────────────────┐
│ 自分の PC                       │
│                                 │
│  localhost:8080 ─┐              │
│                  │              │
│  ┌── kind ───────┼──────────┐  │
│  │               ▼          │  │
│  │  ┌──────────────────┐    │  │
│  │  │ Pod (my-app)     │    │  │
│  │  │ Port 8080        │    │  │
│  │  └──────────────────┘    │  │
│  └──────────────────────────┘  │
└─────────────────────────────────┘
```

## トラブルシュート
- 次の順番で見ていくといいらしい
1. kubectl get pod         → 状態 (Pending / Running / CrashLoopBackOff / Evicted)
2. kubectl describe pod <pod名>    → Events とコンテナの Last State (原因の決定打)
3. kubectl logs <pod名>       → アプリ側のエラーメッセージ
4. kubectl top pod / node  → 実使用量と request/limit の対比
5. Prometheus など          → 長期トレンド、throttle の有無

## 高度な設定
これらはクラスタ作成時に次のようにして設定.yamlファイルを読み込んで特殊な環境にできる
```bash
kind create cluster --config kind-example-config.yaml
```
1. マルチノードクラスタ
    - コントロールプレーン以外にもワーカーノードを追加できる(multinode.yaml)
2. コントロールプレーンのHA(High Avaliability: 高可用性)
    - 複数コントロールプレーンにして冗長化できる(ha.yaml)
3. ホストマシンへのポートマッピング
    - ノード内のポートを自分のPCのポートに直接マッピング可能になる(mapping.yaml)