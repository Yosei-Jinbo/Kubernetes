package myscheduler

import (
	"context"
	"fmt"
	"strings"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fwk "k8s.io/kube-scheduler/framework"
	"k8s.io/kubernetes/pkg/scheduler/framework"
)

const (
	AnnotationRequiredGPUs = "my-scheduler/require-label"
)

/*
/home/y-jinbo/go/pkg/mod/k8s.io/kubernetes@v1.34.4/pkg/scheduler/framework/interface.goに登録されている

//プラグインのためのインターフェース
type Plugin interface {
	Name() string
}

Filterプラグインのためのインターフェース
type FilterPlugin interface {
	Plugin // Name() stringで自分の名前を返す必要がある
	Filter(ctx context.Context, state fwk.CycleState, pod *v1.Pod, nodeInfo fwk.NodeInfo) *fwk.Status //Filterの中身を実装(ダメなやつをはじく)
}
*/

const Name = "my-scheduler" //KubeSchedulerConfigurationのyamlファイルに対してplugins: name: <ここでのName>で登録する

type Filter struct{}                     //状態は必要ないけどFilterプラグイン実装のために型だけは定義
var _ framework.FilterPlugin = &Filter{} //コンパイル時の実装インターフェースチェック(変数登録だけしてそのまま捨てる)

// Name() string
func (pl *Filter) Name() string {
	return Name
}

// Filterメソッドでインターフェース実装
// Podのアノテーションについて、metadata: annotations:として.yamlで定義する
func (pl *Filter) Filter(ctx context.Context, _ fwk.CycleState, pod *v1.Pod, nodeInfo fwk.NodeInfo) *fwk.Status {
	//Podのアノテーションmy-scheduler/require-label: <key>=<value> があれば、そのラベルを持たないnodeをFilterではじく
	annotations := pod.GetAnnotations() // Pod のアノテーションを取得

	// キーが存在するか確認しつつ値を取る
	podKeyValue, ok := annotations[AnnotationRequiredGPUs]
	if !ok {
		// アノテーションが無い Pod は自動で通過する
		return fwk.NewStatus(fwk.Success, "")
	}
	//"<key>=<vlaue>"となっているためパースする
	podLabelKey, podLabelValue, podLabelFound := parseKeyValue(podKeyValue)
	if !podLabelFound {
		//アノテーションがあるのにラベルを持たないようなnodeをFilterではじく
		// Unschedulable: このノードでは動かせないが、他のノードなら動ける可能性あり
		return fwk.NewStatus(fwk.Unschedulable, "label format error")
	}

	//Nodeのリストを取得する
	node := nodeInfo.Node()
	if node == nil {
		return fwk.NewStatus(fwk.Error, "node is nil")
	}
	nodeLabels := node.Labels //Goの慣習 Getterではなく、先頭大文字で取得
	//特定のラベルを取得
	nodeLabelValue, ok := nodeLabels[podLabelKey]
	if !ok {
		//nodeにそのラベルがない➡はじく
		return fwk.NewStatus(fwk.Unschedulable, fmt.Sprintf("this Node has no label %s", podLabelKey))
	}
	if nodeLabelValue != podLabelValue {
		//PodとNodeのラベルが一致していない➡はじく
		return fwk.NewStatus(fwk.Unschedulable, fmt.Sprintf("this Node is not equal value nodeLabelValue: %s but podLabelValue: %s", nodeLabelValue, podLabelValue))
	}

	return fwk.NewStatus(fwk.Success, "") //ちゃんとpodのラベル = nodeのラベルでkey_valueが正しかったならOK
}

func parseKeyValue(s string) (string, string, bool) {
	key, value, found := strings.Cut(s, "=") //"="をもとに分割
	return key, value, found
}

// Newによりプラグインを初期化してそのまま返す
func New(_ context.Context, _ runtime.Object, _ framework.Handle) (framework.Plugin, error) {
	return &Filter{}, nil
}
