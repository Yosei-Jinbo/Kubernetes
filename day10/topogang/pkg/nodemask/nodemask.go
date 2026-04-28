package nodemask

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fwk "k8s.io/kube-scheduler/framework"
)

// これでわかるのはそのインターフェースを実装しているかどうか?
var _ fwk.FilterPlugin = &Nodemask{} //設定でexcludeLabelを持つnodeをFilter拡張点でreject
const Name = "Nodemask"

/*
Argsのためにruntime.Objectのインターフェースを実装する
 1. k8s.io/apimachinery/pkg/runtimeにアクセスしてGoの公式ドキュメントを見る
 2. type Object interfaceをドキュメント内で検索する
    type Object interface {
    GetObjectKind() schema.ObjectKind GetObjectKind()  │ 自分で書かない。metav1.TypeMeta という既製の struct を埋め込むだけで自動的に手に入る
    DeepCopyObject() Object							   │ 自分で書く。自分自身のコピーを作って runtime.Object として返すだけ
    }
    こんなのが出てくるからこれらを実装すればArgsを自分で実装できるね!
*/
//これで自動的にruntime.Object interfaceも実装していることになる
type Args struct {
	metav1.TypeMeta `json:",inline"`
	ExcludeLabel    string `json:"excludeLabel"`
}

/*
返り値型は runtime.Object なのに、なぜruntime.Object は*Args (アドレス) を返す?
interface (箱) で、*Args を中身として詰められる。Goが自動で詰めてくれる
*/
func (in *Args) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}
	out := *in
	return &out //アドレスを返す
}

type Nodemask struct {
	args Args
}

/*
KubeSchedulerConfiguration の YAML はこんな感じになります:

	pluginConfig:
	  - name: Nodemask
	    args:
	      excludeLabel: topogang.io/exclude    # ← この値を Go コードに持ち込みたい

このargsをFilter()まで持ち込むためにはNew()からArg structを自分で定義してNew()でruntime.Objectからdecodeする
*/
func (pl *Nodemask) Filter(ctx context.Context, state fwk.CycleState, pod *v1.Pod, nodeInfo fwk.NodeInfo) *fwk.Status {
	//nodeを取得してそこからさらにラベル情報を抽出 ➡ labelのうち、excludeLabelを持つnodeをFilter(UnScheduled)
	node := nodeInfo.Node()
	if node == nil {
		return fwk.NewStatus(fwk.Error, "node not found")
	}
	if _, ok := node.Labels[pl.args.ExcludeLabel]; ok {
		//UnschedulableAndUnresolvable: 未来永劫これはスケジュール不可
		//Unschedulable: 今は無理だが状況が変わればいけるかも
		return fwk.NewStatus(fwk.UnschedulableAndUnresolvable, fmt.Sprintf("node has excludeLabel %q", pl.args.ExcludeLabel)) //Filterではじく
	}
	return fwk.NewStatus(fwk.Success, "")
}

func (pl *Nodemask) Name() string {
	return Name
}

func New(_ context.Context, obj runtime.Object, _ fwk.Handle) (fwk.Plugin, error) {
	args, ok := obj.(*Args) //このargsはポインタになっている
	if !ok {
		return nil, fmt.Errorf("want *nodemask.Args, got %T", obj)
	}
	if args.ExcludeLabel == "" {
		return nil, fmt.Errorf("want not nil excludelabel")
	}
	return &Nodemask{args: *args}, nil //ここでstructどおりに値型のargsに戻す
}
