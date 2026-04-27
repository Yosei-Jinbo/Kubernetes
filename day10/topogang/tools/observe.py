import subprocess, json, time, sys
from collections import defaultdict
from datetime import datetime, timezone

def get_pods(selector, namespace="default"):
    # subprocess で kubectl get pods -l <selector> -o json
    # <selector>はラベルセレクターで、Podに付与されているmetadata.labelsを条件に絞り込む
    # 例1: kubectl get pods -l app=nginx #等価チェック、　例2: kubectl get pods -l 'env in (prod,staging)' #集合ベースのチェック
    # labelはapp=topogang
    result = subprocess.run(
        ["kubectl", "get", "pods",
           "-l", selector,
           "-n", namespace,
           "-o", "json"],
        capture_output=True, #stdout/stderrを捕まえる
        text=True, #bytesではなく、strで受け取る
        check=True, #exit code!=0で例外を投げる
    )
    # JSON parse して items を返す
    '''これがjsonとして返す出力
    (.venv) y-jinbo@y-jinbo:~/Kubernetes/day10/topogang/manifests$ kubectl get pods -o json
    {
        "apiVersion": "v1",
        "items": [],
        "kind": "List",
        "metadata": {
            "resourceVersion": ""
        }
    }
    
    observe.py で必要になりそうな field                                                                                                     
    ┌─────────────────────┬────────────────────────────────────────────────────────────────┐                                                
    │     取り出す値      │                          アクセス方法                          │                                              
    ├─────────────────────┼────────────────────────────────────────────────────────────────┤                                                
    │ Pod 名              │ pod["metadata"]["name"]                                        │                                              
    ├─────────────────────┼────────────────────────────────────────────────────────────────┤
    │ ラベル              │ pod["metadata"]["labels"]                                      │                                                
    ├─────────────────────┼────────────────────────────────────────────────────────────────┤                                                
    │ 作成時刻            │ pod["metadata"]["creationTimestamp"]                           │                                                
    ├─────────────────────┼────────────────────────────────────────────────────────────────┤                                                
    │ 配置先 node         │ pod["spec"].get("nodeName") ← Pending 中は キーが存在しない    │                                              
    ├─────────────────────┼────────────────────────────────────────────────────────────────┤                                                
    │ phase               │ pod["status"]["phase"]                                         │
    ├─────────────────────┼────────────────────────────────────────────────────────────────┤                                                
    │ PodScheduled の時刻 │ pod["status"]["conditions"] を type=="PodScheduled" でフィルタ │                                              
    └─────────────────────┴────────────────────────────────────────────────────────────────┘  
    '''
    data = json.loads(result.stdout) #str -> dictionaryにパース
    return data["items"] #このitemsにはmetadata, spec, statusなどが入る

def get_node_rack(node_name):
    # node の topology.example.io/rack label を返す
    # これは kubectl get node <node_name> -o jsonでjson形式で次のkeysは手に入る
    '''
    (.venv) y-jinbo@y-jinbo:~/Kubernetes/day10/topogang/manifests$ kubectl get node topogang-cluster-worker -o json | jq '.metadata | keys'
    [
    "annotations",
    "creationTimestamp",
    "labels",
    "name",
    "resourceVersion",
    "uid"
    ]
    '''
    #そのため、 | jq '.metadata.labels'により、jqueryとしてlabelsの値を手に入れる(次のようになる)
    '''
    {
    "beta.kubernetes.io/arch": "amd64",
    "beta.kubernetes.io/os": "linux",
    "kubernetes.io/arch": "amd64",
    "kubernetes.io/hostname": "topogang-cluster-worker",
    "kubernetes.io/os": "linux",
    "topology.example.io/block": "1",
    "topology.example.io/rack": "A"
    }
    '''
    #これを利用して、["topology.example.io/rack"]としてdictionaryから引けばいい
    result = subprocess.run(
        ["kubectl", "get", "node", node_name, "-o", "json"],
        capture_output=True,
        text=True,
        check=True
    )
    data = json.loads(result.stdout)
    labels = data["metadata"]["labels"]
    return labels.get("topology.example.io/rack")

#「Podの状態」「rack 分布」を JSON で出す
def main(selector, max_seconds=300, min_seconds=10):
    # 1 秒間隔で Pod 状態を観測
    #pod名 ➡ (状態、初めて Pending/Running/Succeeded になったタイムスタンプ、それが配置されているnode名、そのnodeの所属しているrack(=A or B))となる辞書を宣言
    dic = defaultdict(lambda: {
        "phase":        None,   # 状態 (最新の phase)
        "pending_at":   None,   # 初めて Pending を観測した時刻
        "running_at":   None,   # 初めて Running を観測した時刻
        "succeeded_at": None,   # 初めて Succeeded を観測した時刻 (Job 用)
        "node":         None,   # 配置先 node 名
        "rack":         None,   # その node の rack (A or B)
    })

    start = time.time()

    while time.time() - start < max_seconds:
        # selector で指定された Pod の一覧を get_pods で取得(関数を再利用)
        for p in get_pods(selector):
            pod_name = p["metadata"]["name"]
            phase    = p["status"]["phase"]    # Running / Pending / Succeeded / Failed
            node     = p["spec"].get("nodeName")  # 未 schedule なら None
            now      = datetime.now(timezone.utc).isoformat()

            slot = dic[pod_name]
            slot["phase"] = phase  # 最新 phase で常に上書き

            # 初めてその phase を観測した時刻だけ記録
            if phase == "Pending" and slot["pending_at"] is None:
                slot["pending_at"] = now
            if phase == "Running" and slot["running_at"] is None:
                slot["running_at"] = now
            if phase == "Succeeded" and slot["succeeded_at"] is None:
                slot["succeeded_at"] = now

            # node が確定している Pod のみ rack を解決(Pending 中は None なのでスキップ)
            if node is not None:
                slot["node"] = node
                if slot["rack"] is None:  # rack は 1 度引いたら使い回し
                    slot["rack"] = get_node_rack(node)

        # 終了条件: 最低 min_seconds は観測 (起動直後の「全部 Pending」での早期終了を防ぐ)。
        # その後、全 Pod が「決着」phase に達していれば抜ける。
        elapsed = time.time() - start
        settled = ("Running", "Pending", "Succeeded", "Failed", "Complete")
        if dic and elapsed >= min_seconds and all(s["phase"] in settled for s in dic.values()):
            break

        time.sleep(1)

    # rack 分布を集計 (未 schedule の Pending Pod は "unscheduled" としてカウント)
    rack_distribution = defaultdict(int)
    for slot in dic.values():
        key = slot["rack"] if slot["rack"] else "unscheduled"
        rack_distribution[key] += 1

    output = {
        "pods": dict(dic),
        "rack_distribution": dict(rack_distribution),
    }
    # 結果を JSON で出力
    print(json.dumps(output, indent=2, ensure_ascii=False))

if __name__ == "__main__":
    main(sys.argv[1])