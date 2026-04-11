package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

func main() {
	// .healthzにアクセスが来たら、この関数を実行する
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		//200を返す, w.Writeは[]byteを引数としていいるのでw.Write()は利用不可
		fmt.Fprintln(w, "ok") //ボディに"ok"とかく。ステータスは自動的に200
	})

	// /(version+host)の出力
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		version := os.Getenv("APP_VERSION")
		fmt.Fprintln(w, "version = ", version)

		hostname, err := os.Hostname()
		if err != nil {
			http.Error(w, "hostname取得エラー: "+err.Error(), http.StatusInternalServerError)
			return
		}
		fmt.Fprintln(w, "hostname = ", hostname)
	})

	// /envにアクセスが来たら、この関数を実行する
	http.HandleFunc("/env", func(w http.ResponseWriter, r *http.Request) {
		for _, e := range os.Environ() { //環境変数を読み込んで一覧を出力
			fmt.Fprintln(w, e)
		}
	})

	// /config
	http.HandleFunc("/config", func(w http.ResponseWriter, r *http.Request) {
		data, err := os.ReadFile("/etc/config/app.config")
		if err != nil {
			http.Error(w, "ファイル読み込みエラーr: "+err.Error(), 500)
			return
		}
		fmt.Println(w, data)
	})

	// /secret
	http.HandleFunc("/secret", func(w http.ResponseWriter, r *http.Request) {
		dir := "/etc/secret"
		entries, err := os.ReadDir(dir)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			path := filepath.Join(dir, e.Name())
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			fmt.Println(w, "%s: %s\n", e.Name(), data)
		}
	})

	start_time := time.Now()
	// /ready
	http.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		if time.Since(start_time) < 10*time.Second {
			w.WriteHeader(http.StatusServiceUnavailable) // 503
			fmt.Fprintln(w, "not ready")
			return
		}
		fmt.Fprintln(w, "ready") //勝手にデフォルトとして200が入る
	})

	// /crash
	http.HandleFunc("/crash", func(w http.ResponseWriter, r *http.Request) {
		os.Exit(1)
	})

	http.ListenAndServe(":8090", nil)
}
