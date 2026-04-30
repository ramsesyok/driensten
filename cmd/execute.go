/*
Copyright © 2025 ramsesyok

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

func execute() {
	// グレースフルシャットダウン用のコンテキスト
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// シグナル監視 (CTRL+C)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		slog.Info("shutdown signal received, stopping services...")
		cancel()
	}()

	// サービスエラー通知チャネル
	errCh := make(chan error, 1)
	// publishメッセージ通知チャンネル
	msgCh := make(chan UdpMessage, 1)

	var wg sync.WaitGroup
	wg.Add(3)

	// 各サービスを起動
	go startWebServer(ctx, &wg, errCh)
	go startMQTTBroker(ctx, &wg, errCh, msgCh)
	go startUDPListener(ctx, &wg, errCh, msgCh)

	// いずれかのサービスがエラーを返した場合、シャットダウン
	go func() {
		if err := <-errCh; err != nil {
			slog.Error("Service error, shutting down all services...", slog.String("error", err.Error()))
			cancel()
		}
	}()

	// 全ゴルーチンの終了を待機
	wg.Wait()
	slog.Info("all services stopped gracefully")
}
