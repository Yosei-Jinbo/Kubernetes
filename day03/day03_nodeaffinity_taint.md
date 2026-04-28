## Podをノードに割り当てる方法
1. *nodeSelector*
    - nodeSelectorで指定したラベルを持つノードにPodを配置
2. *nodeAffinity・Anti-Affinity*
    - 優先度制御 + 演算子の利用 + ほかのPodラベルを利用して配置制御ができるより柔軟な制御ルール
    - *nodeAffinity*: ノードのラベルで制御(required = 必須 / preferred = 優先)
    - *InterPodAffinity/Anti-Affinity*: 他Podのラベルで制御(topologyKeyによりPod同士の場所を管理する)
    - *KubeSchedulerConfigurationにより、プロファイルごとに追加で制御できるaffinityを設定可能
    - *affinity.yamlというファイルに例が書かれている
3. *nodeName*
    - Pod specにノード名を直接指定してスケジューラをバイパスして kubelet が直接起動を試みる
4. *Pod topology spread constraints*
    - Podをゾーン・リージョン・ノードなどのトポロジー単位で均等に分散させる仕組み

## Taints/Tolerations(ノードが特定のPodをはじく仕組み)
- *Taints*
    - ノードに設定して不適切なPodをはじくための仕組み、以下のようにkubectlで付与・削除
    ```bash
    kubectl taint nodes node1 key1=value1:NoSchedule  #Taints付与
    kubectl taint nodes node1 key1=value1:NoSchedule- #末尾に-をつけて削除
    ```
- *Tolerations*
    - Podに設定してTaintsを許容する(TolerationsのついたPodはTaintsがついたノードに配置していい)
    - Tolerationsのeffectフィールドのそれぞれの動作(あくまでTaintsははじくためのもの)
        1. NoSchedule: tolerationがないなら新規Podはスケジュールされない(すでに稼働中のPodはそのまま)、tolerationがあるなら配置されるかもしれない(候補に入るだけで優先的にここに来るわけではない)
        2. PreferNoSchedule: 新規Podをスケジュールすることをなるべく避ける, 
        3. NoExecute: Tolerationなし➡そのノード内にいても強制的にEvict, Tolerationあり➡永久に残ることが可能 or tolerationSecondsだけ待ってEvict
    - 特にNoExecuteを利用して障害などが起こったタイミングでそのPodを残す・Evict・数秒待ってEvictと選べるようになる。
      (これはノードコントローラが特定の条件下でノードに対してnode.kubernetes.io/memory-pressureなどのTaintを自動で付与するため)

## Pod topology spread constraints
- topologySpreadConstraintsによりリージョン、ゾーン、ノードなどの障害ドメイン間でPodを分散配置し、高可用性とリソースの効率的な管理を達成する
- `kind: KubeSchedulerConfiguration`にデフォルトの制約を設定してクラスタ内のすべてのPodに分散スケジュールを適用することができる。
- 例として、opology-spread-constraints-one(two)-constraint.yaml