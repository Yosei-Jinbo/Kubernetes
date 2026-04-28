# 自作Scheduler-Pluginのインストール(実際に使う方法)

## scheduler-pluginsの導入方法
1. セカンドスケジュール方式: 複数スケジューラで競合するので非推奨らしい
2. *単一スケジューラとして入れる方式*: デフォルトスケジューラを置き換える方式(本番環境では利用する)

## 単一スケジューラ方式での自作shceduler-pluginの利用方法
1. kindの作成
    ```bash
    kind create cluster
    ```
2. コントロールプレーンノードに入る + バックアップ
    - kindのコントロールプレーンコンテナに`docker exec`により入る
        ```bash
        sudo docker exec -it $(sudo docker ps | grep control-plane | awk '{print $1}') bash
        ```
    - 元の設定(Static Podの定義ファイル)を退避させる
        ```bash
        cp /etc/kubernetes/manifests/kube-scheduler.yaml /etc/kubernetes/kube-scheduler.yaml
        ```
3. スケジューラ設定ファイルを作成する
    - *コンテナ内で*スケジューラ本体に読ませるための`KubeSchedulerConfiguration`を作成
    - `coscheduling-test/configs/kube-scheduler.yaml`に例がある
4. CRD 用のコントローラと RBAC をデプロイ
    - CoschedulingなどのプラグインのためにCRD(カスタムリソース定義)を利用するように設定する
    - *system:kube-scheduler ユーザに CRD オブジェクトを操作できるよう 追加の RBAC 権限 を付与する + CRD オブジェクトを管理するための コントローラバイナリ をインストールする* ために*ホスト内で*次を実行する
        ```bash
        kubectl apply -f ~/scheduler-plugins/manifests/install/all-in-one.yaml
        ```
    - *カスタムリソースをAPIサーバに登録するために次を実行*
        ```bash
        kubectl apply -f ~/scheduler-plugins/manifests/crds/
        ```
5. `kube-scheduler.yaml`を書き換えてscheduler-plugins版を起動する
    - *自作したカスタムスケジューラの設定*`sched-cc.yaml`をhostPathボリュームとしてマウントして、スケジューラから読めるようにする
    - fileは`/etc/kubernetes/manifests/kubescheduler.yaml`に戻せばkubeleteが*自動的に*Static Podとして立ち上げ
    - `kubectl get pod -n kube-system | grep kube-scheduler`によりPodが新しいイメージで動作しているかがわかる

- Coschedulingの動作テスト
    - Coschedulingプラグインにより「指定した数のPodが全部そろわないとスケジュールしない」というギャングスケジューリングを実現する
    - *PodGroup(GangSchedulingを適用するポッド群)*を`pg1.yaml`に定義
    - *PodGroupに属するDeployment*を`pause-deploy.yaml`に作成、`scheduling.x-k8s.io/pod-group: pg1`としてPodGroup pg1に属するPod群に対してこのルールを適用