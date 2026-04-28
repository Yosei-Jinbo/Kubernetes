package noop //package名はすべて小文字

import (
	"context"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	fwk "k8s.io/kube-scheduler/framework"
)

// これでわかるのはそのインターフェースを実装しているかどうか?
var _ fwk.ScorePlugin = &Noop{} //score0を返してログに呼ばれたことだけを出す
const Name = "Noop"

type Noop struct{}

// Scoreプラグインを実装するときに必須
func (pl *Noop) ScoreExtensions() fwk.ScoreExtensions {
	return nil //NormalizeScoreを持っているなら自分自身を返して、そうでないならnilを返すのが正解
}

// Score は呼ばれた事実を log に残して 0 を返すだけ。
func (pl *Noop) Score(ctx context.Context, state fwk.CycleState, pod *v1.Pod, nodeInfo fwk.NodeInfo) (int64, *fwk.Status) {
	node := nodeInfo.Node()
	if node == nil {
		return 0, fwk.NewStatus(fwk.Error, "node not found")
	}
	nodeName := node.Name //GetName()と同じGetterメソッド, Goでは先頭を大文字にするフィールドがGetterと同じ役割
	klog.FromContext(ctx).V(4).Info("Noop.Score called", "pod", klog.KObj(pod), "node", nodeName)
	return 0, nil
}

func (pl *Noop) Name() string {
	return Name
}

func New(_ context.Context, _ runtime.Object, _ fwk.Handle) (fwk.Plugin, error) {
	return &Noop{}, nil
}
