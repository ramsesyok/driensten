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
	"bytes"
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/spf13/viper"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/auth"
	"github.com/mochi-mqtt/server/v2/listeners"
	"github.com/mochi-mqtt/server/v2/packets"
)

type UdpMessage struct {
	Topic   string
	Payload string
}

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

// HTTPサーバを起動し、エラー発生時に errCh へ通知、コンテキストキャンセルでシャットダウン
func startWebServer(ctx context.Context, wg *sync.WaitGroup, errCh chan<- error) {
	defer wg.Done()
	addr := viper.GetString("HTTP.listen")
	fsRoot := viper.GetString("HTTP.root")
	slog.Info("load configuration", slog.String("HTTP.listen", addr))
	slog.Info("load configuration", slog.String("HTTP.root", fsRoot))
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
	if err := e.Start(addr); err != nil && err != http.ErrServerClosed {
		errCh <- err
	}
}

// MQTTブローカーを起動し、エラー発生時に errCh へ通知、コンテキストキャンセルで停止
func startMQTTBroker(ctx context.Context, wg *sync.WaitGroup, errCh chan<- error, msgCh <-chan UdpMessage) {
	defer wg.Done()
	tcpAddress := viper.GetString("MQTT.tcp")
	webAddress := viper.GetString("MQTT.websocket")
	slog.Info("load configuration", slog.String("MQTT.tcp", tcpAddress))
	slog.Info("load configuration", slog.String("MQTT.websocket", webAddress))

	broker := mqtt.New(&mqtt.Options{InlineClient: true, Logger: slog.Default()})
	_ = broker.AddHook(new(auth.AllowHook), nil)
	// TCP待ち受け
	tcp := listeners.NewTCP(listeners.Config{ID: "mqtt.tcp", Address: tcpAddress})
	if err := broker.AddListener(tcp); err != nil {
		slog.Error("mqtt failed to listen for TCP connections.", slog.String("error", err.Error()))
		errCh <- err
		return
	}

	// WebSocket待ち受け
	web := listeners.NewWebsocket(listeners.Config{ID: "mqtt.websocket", Address: webAddress})
	if err := broker.AddListener(web); err != nil {
		slog.Error("mqtt failed to listen for WebSocket connections.", slog.String("error", err.Error()))
		errCh <- err
		return
	}

	// 転送用UDPの設定を取得
	forwards := viper.GetStringMapStringSlice("UDP.forwards")
	topics := map[string]string{}
	for addr, tpcs := range forwards {
		slog.Info("load configuration", slog.String("UDP.forwards.topic", addr), slog.String("UDP.forwards.address", strings.Join(tpcs, ",")))
		for _, topic := range tpcs {
			topics[topic] = addr
		}
	}
	hasTopic := len(forwards) > 0 // トピック設定があるかの判定用

	callbackFn := func(cl *mqtt.Client, sub packets.Subscription, pk packets.Packet) {
		// 転送用UDPソケットを作成（nil指定で、エフェメラルポートをオープン）
		conn, err := net.ListenUDP("udp", nil)
		if err != nil && hasTopic { // UDPエラーが発生した上に、topicが存在していた場合はエラー
			slog.Error("udp forwarder failed to create socket", slog.String("error", err.Error()))
			errCh <- err
			return

		}
		defer conn.Close()
		// トピックに対応した転送先があれば、転送する
		if writeTo, ok := topics[pk.TopicName]; ok {
			if udpAddr, err := net.ResolveUDPAddr("udp", writeTo); err != nil {
				slog.Error("udp forwarder failed to resolve addres", slog.String("error", err.Error()), slog.String("writeTo", writeTo))
			} else {
				// 送信用メッセージを作成
				messages := [][]byte{}
				messages = append(messages, []byte(pk.TopicName))
				messages = append(messages, pk.Payload)
				body := bytes.Join(messages, []byte("\n"))

				// アドレスに以上がなければ、送信時のエラーチェックしない
				if n, err := conn.WriteToUDP(body, udpAddr); err != nil {
					slog.Info("udp forwarder failed to write data", slog.String("error", err.Error()))
				} else {
					slog.Info("udp forwarder wrote data", slog.Int("size", n))
				}
			}
		} else {
			slog.Warn("udp forwarder unknown topic", slog.String("topic", pk.TopicName))
		}
	}
	for topic := range topics {
		err := broker.Subscribe(topic, 1, callbackFn)
		if err != nil {
			slog.Warn("udp forwarder failed to subscribe", slog.String("error", err.Error()))
		} else {
			slog.Info("udp forwarder regist subscribe", slog.String("topic", topic))
		}
	}

	// コンテキストキャンセルで停止
	go func() {
		for {
			select {
			case <-ctx.Done():
				slog.Info("mochi mqtt receive shutdown request")
				broker.Close()
			case msg := <-msgCh:
				slog.Debug("udp forwarder received message", slog.String("topic", msg.Topic), slog.String("payload", string(msg.Payload)))
				broker.Publish(msg.Topic, []byte(msg.Payload), false, 1)
			}
		}
	}()

	// ブロックしてブローカー実行
	if err := broker.Serve(); err != nil {
		errCh <- err
	}
}

// UDPリスナーを起動し、エラー発生時に errCh へ通知、コンテキストキャンセルで停止
func startUDPListener(ctx context.Context, wg *sync.WaitGroup, errCh chan<- error, msgCh chan<- UdpMessage) {
	defer wg.Done()
	updListen := viper.GetString("UDP.listen")
	slog.Info("load configuration", slog.String("UDP.listen", updListen))
	udpAddr, udpPort, err := net.SplitHostPort(updListen)
	if err != nil {
		errCh <- err
		return
	}
	port, err := strconv.Atoi(udpPort)
	if err != nil {
		errCh <- err
		return
	}
	addr := net.UDPAddr{IP: net.ParseIP(udpAddr), Port: port}
	conn, err := net.ListenUDP("udp", &addr)
	if err != nil {
		errCh <- err
		return
	}
	defer conn.Close()

	buf := make([]byte, 1024)
	for {
		select {
		case <-ctx.Done():
			slog.Info("udp listener receive shutdown request")
			return
		default:
			conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
			n, _, err := conn.ReadFrom(buf)
			if err != nil {
				if ne, ok := err.(net.Error); ok && ne.Timeout() {
					continue
				}
				// それ以外のエラーはサービス停止トリガー
				errCh <- err
				return
			}
			payload := string(buf[:n])
			body := strings.SplitN(payload, "\n", 2)
			if len(body) < 2 {
				slog.Warn("udp listener invalid payload", slog.String("payload", payload))
				continue
			}
			msg := UdpMessage{
				Topic:   body[0],
				Payload: body[1],
			}
			msgCh <- msg
		}
	}
}
