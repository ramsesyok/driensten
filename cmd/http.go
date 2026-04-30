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
	"net/http"
	"sync"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/spf13/viper"
)

// HTTPサーバを起動し、エラー発生時に errCh へ通知、コンテキストキャンセルでシャットダウン
func startWebServer(ctx context.Context, wg *sync.WaitGroup, errCh chan<- error) {
	defer wg.Done()
	addr := viper.GetString("HTTP.listen")
	fsRoot := viper.GetString("HTTP.root")
	tlsEnable := viper.GetBool("HTTP.tls.enable")
	tlsCert := viper.GetString("HTTP.tls.cert")
	tlsKey := viper.GetString("HTTP.tls.key")
	slog.Info("load configuration", slog.String("HTTP.listen", addr))
	slog.Info("load configuration", slog.String("HTTP.root", fsRoot))
	slog.Info("load configuration", slog.Bool("HTTP.tls.enable", tlsEnable))
	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Static("/", fsRoot) // ./static ディレクトリを配信

	// コンテキストキャンセルで停止
	go func() {
		<-ctx.Done()
		slog.Info("echo web server receive shutdown request")
		if err := e.Shutdown(context.Background()); err != nil {
			errCh <- err
		}
	}()

	// ブロックしてサーバ実行
	var err error
	if tlsEnable {
		err = e.StartTLS(addr, tlsCert, tlsKey)
	} else {
		err = e.Start(addr)
	}
	if err != nil && err != http.ErrServerClosed {
		errCh <- err
	}
}
