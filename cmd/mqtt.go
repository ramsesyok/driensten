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
	"crypto/tls"
	"log/slog"
	"net"
	"strings"
	"sync"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/listeners"
	"github.com/mochi-mqtt/server/v2/packets"
	"github.com/spf13/viper"
)

// mqttTopicMatch は MQTT のトピックフィルタ（ワイルドカード + / # を含む）と
// 具体的なトピック名が一致するか判定する。
// MQTT 仕様に従い、+ は単一レベル、# は末尾で残り全レベルにマッチする。
func mqttTopicMatch(filter, topic string) bool {
	filterParts := strings.Split(filter, "/")
	topicParts := strings.Split(topic, "/")
	fi, ti := 0, 0
	for fi < len(filterParts) {
		if filterParts[fi] == "#" {
			return true
		}
		if ti >= len(topicParts) {
			return false
		}
		if filterParts[fi] != "+" && filterParts[fi] != topicParts[ti] {
			return false
		}
		fi++
		ti++
	}
	return fi == len(filterParts) && ti == len(topicParts)
}

type mqttUser struct {
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

type mqttAuthConfig struct {
	Mode  string     `mapstructure:"mode"`
	Users []mqttUser `mapstructure:"users"`
}

type mqttAuthHook struct {
	mqtt.HookBase
	cfg mqttAuthConfig
}

func (h *mqttAuthHook) ID() string { return "driensten-auth" }

func (h *mqttAuthHook) Provides(b byte) bool {
	return bytes.Contains([]byte{
		mqtt.OnConnectAuthenticate,
		mqtt.OnACLCheck,
	}, []byte{b})
}

func (h *mqttAuthHook) OnConnectAuthenticate(cl *mqtt.Client, pk packets.Packet) bool {
	switch h.cfg.Mode {
	case "password":
		for _, u := range h.cfg.Users {
			if u.Username == string(pk.Connect.Username) &&
				u.Password == string(pk.Connect.Password) {
				return true
			}
		}
		slog.Warn("mqtt auth rejected", slog.String("username", string(pk.Connect.Username)))
		return false
	case "cert":
		tlsConn, ok := cl.Net.Conn.(*tls.Conn)
		if !ok {
			slog.Warn("mqtt cert auth rejected: connection is not TLS")
			return false
		}
		if len(tlsConn.ConnectionState().PeerCertificates) == 0 {
			slog.Warn("mqtt cert auth rejected: no client certificate presented")
			return false
		}
		return true
	default: // "none"
		return true
	}
}

func (h *mqttAuthHook) OnACLCheck(cl *mqtt.Client, topic string, write bool) bool {
	return true
}

// MQTTブローカーを起動し、エラー発生時に errCh へ通知、コンテキストキャンセルで停止
func startMQTTBroker(ctx context.Context, wg *sync.WaitGroup, errCh chan<- error, msgCh <-chan UdpMessage) {
	defer wg.Done()
	tcpAddress := viper.GetString("MQTT.tcp")
	webAddress := viper.GetString("MQTT.websocket")
	tlsEnable := viper.GetBool("MQTT.tls.enable")
	tlsCert := viper.GetString("MQTT.tls.cert")
	tlsKey := viper.GetString("MQTT.tls.key")
	slog.Info("load configuration", slog.String("MQTT.tcp", tcpAddress))
	slog.Info("load configuration", slog.String("MQTT.websocket", webAddress))
	slog.Info("load configuration", slog.Bool("MQTT.tls.enable", tlsEnable))

	var authCfg mqttAuthConfig
	if err := viper.UnmarshalKey("MQTT.auth", &authCfg); err != nil {
		slog.Warn("MQTT.auth config parse error, falling back to none", slog.String("error", err.Error()))
		authCfg.Mode = "none"
	}
	if authCfg.Mode == "" {
		authCfg.Mode = "none"
	}
	slog.Info("load configuration", slog.String("MQTT.auth.mode", authCfg.Mode))
	if authCfg.Mode == "none" {
		slog.Warn("MQTT authentication is DISABLED (MQTT.auth.mode=none) - do not use in production")
	}

	broker := mqtt.New(&mqtt.Options{InlineClient: true, Logger: slog.Default()})
	_ = broker.AddHook(&mqttAuthHook{cfg: authCfg}, nil)
	var tlsConfig *tls.Config
	if tlsEnable {
		cert, err := tls.LoadX509KeyPair(tlsCert, tlsKey)
		if err != nil {
			slog.Error("mqtt failed to load TLS certificate", slog.String("error", err.Error()))
			errCh <- err
			return
		}
		tlsConfig = &tls.Config{Certificates: []tls.Certificate{cert}}
	}

	// TCP待ち受け
	tcp := listeners.NewTCP(listeners.Config{ID: "mqtt.tcp", Address: tcpAddress, TLSConfig: tlsConfig})
	if err := broker.AddListener(tcp); err != nil {
		slog.Error("mqtt failed to listen for TCP connections.", slog.String("error", err.Error()))
		errCh <- err
		return
	}

	// WebSocket待ち受け
	web := listeners.NewWebsocket(listeners.Config{ID: "mqtt.websocket", Address: webAddress, TLSConfig: tlsConfig})
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
	allowUnauthForwards := viper.GetBool("UDP.allow_unauthenticated_forwards")
	slog.Info("load configuration", slog.Bool("UDP.allow_unauthenticated_forwards", allowUnauthForwards))

	forwardingEnabled := authCfg.Mode != "none" || allowUnauthForwards
	if !forwardingEnabled {
		slog.Warn("MQTT→UDP forwarding is DISABLED: set UDP.allow_unauthenticated_forwards=true to enable forwarding without authentication")
	} else {
		if authCfg.Mode == "none" && allowUnauthForwards {
			slog.Warn("MQTT→UDP forwarding enabled without authentication (UDP.allow_unauthenticated_forwards=true) - unauthenticated clients can inject data into UDP streams")
		}
		for topic := range topics {
			err := broker.Subscribe(topic, 1, callbackFn)
			if err != nil {
				slog.Warn("udp forwarder failed to subscribe", slog.String("error", err.Error()))
			} else {
				slog.Info("udp forwarder regist subscribe", slog.String("topic", topic))
			}
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
				matched := false
				for filter := range topics {
					if mqttTopicMatch(filter, msg.Topic) {
						matched = true
						break
					}
				}
				if !matched {
					slog.Warn("udp listener topic not allowed", slog.String("topic", msg.Topic))
					continue
				}
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
