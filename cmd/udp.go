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
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/spf13/viper"
)

type UdpMessage struct {
	Topic   string
	Payload string
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
			if len(body) < 2 || body[0] == "" {
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
