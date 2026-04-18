package myscheduler

import (
	"context"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fwk "k8s.io/kube-scheduler/framework"
	"k8s.io/kubernetes/pkg/scheduler/framework"
)

var _ framework.ScorePlugin = &GPUGenerationScore{}
var _ framework.ScoreExtensions = &GPUGenerationScore{}

const Name = "GPUGenerationScore"

type GPUGenerationScore struct{}

// ScoreExtensoins(): Scoreプラグインのオプション拡張点
// 全ノードのスコアリング後に呼ばれるフック(NormalizeScoreを実装するために利用する)
func (pl *GPUGenerationScore) ScoreExtensions() framework.ScoreExtensions {
	return pl //自分自身がNormalizeScoreを実装する
}

// MyPlugin構造体自体がNormalizeScore()を実装する
func (pl *GPUGenerationScore) NormalizeScore(ctx context.Context, state fwk.CycleState, pod *v1.Pod, scores framework.NodeScoreList) *fwk.Status {
	//TODO: NormalizeScoreの実装
	maxNodeScore := framework.MaxNodeScore //100 (常に定数100)

	maxGPUScore := int64(0) //ここで実際のNode ListのNodeのうち優先度が最大のものを取得する
	for i := range scores {
		if scores[i].Score > maxGPUScore {
			maxGPUScore = scores[i].Score
		}
	}

	if maxGPUScore == 0 {
		return nil
	}

	for i, _ := range scores {
		score := scores[i].Score
		score = maxNodeScore * score / maxGPUScore //[0,100]に正規化
		scores[i].Score = score
	}
	return nil
}

// Score拡張点
// Podを作成する際にmetadata: label: gpu-generation: "a100"
// Nodeにbashで`kubectl label node worker-1 gpu-generation=h100`などで作る
// 今回のScore()はNodeに対するスコア付のため、Nodeについたラベルを見てそのノードの点数をつける
func (pl *GPUGenerationScore) Score(ctx context.Context, state fwk.CycleState, pod *v1.Pod, nodeInfo fwk.NodeInfo) (int64, *fwk.Status) {
	node := nodeInfo.Node()
	switch node.Labels["gpu-generation"] {
	case "v100":
		return 100, nil
	case "a100":
		return 200, nil
	case "h100":
		return 300, nil
	default:
		return 0, nil
	}
}

func (pl *GPUGenerationScore) Name() string {
	return Name
}

func New(_ context.Context, _ runtime.Object, _ framework.Handle) (framework.Plugin, error) {
	return &GPUGenerationScore{}, nil
}
