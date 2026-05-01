//go:build integration

/*
Copyright © 2026 ramsesyok

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
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// startBrokerForTest は MQTT broker を起動するヘルパ。
// 任意の追加 viper 設定を受け取り、起動完了を待ってから返る。
// 戻り値の cleanup を defer で呼ぶこと。
func startBrokerForTest(t *testing.T, mqttTCP string, configure func()) (errCh <-chan error, cleanup func()) {
	t.Helper()

	viper.Set("MQTT.tcp", mqttTCP)
	viper.Set("MQTT.websocket", localAddr(freeTCPPort(t)))
	if configure != nil {
		configure()
	}

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	errChBuf := make(chan error, 2)
	msgCh := make(chan UdpMessage, 1)

	wg.Add(1)
	go startMQTTBroker(ctx, &wg, errChBuf, msgCh)

	waitForTCP(t, mqttTCP, 2*time.Second)

	cleanup = func() {
		cancel()
		waitWG(t, &wg, 3*time.Second)
	}
	return errChBuf, cleanup
}

// pahoConnectExpectingSuccess は paho クライアントを接続し、成功することを assert する。
func pahoConnectExpectingSuccess(t *testing.T, brokerAddr, clientID, username, password string) paho.Client {
	t.Helper()
	opts := paho.NewClientOptions().
		AddBroker("tcp://" + brokerAddr).
		SetClientID(clientID).
		SetConnectTimeout(2 * time.Second).
		SetAutoReconnect(false)
	if username != "" || password != "" {
		opts.SetUsername(username).SetPassword(password)
	}
	c := paho.NewClient(opts)
	tok := c.Connect()
	require.True(t, tok.WaitTimeout(3*time.Second), "MQTT connect timeout")
	require.NoError(t, tok.Error(), "MQTT connect should succeed")
	return c
}

// pahoConnectExpectingRejection は paho クライアントを接続し、認証拒否されることを assert する。
func pahoConnectExpectingRejection(t *testing.T, brokerAddr, clientID, username, password string) {
	t.Helper()
	opts := paho.NewClientOptions().
		AddBroker("tcp://" + brokerAddr).
		SetClientID(clientID).
		SetConnectTimeout(2 * time.Second).
		SetAutoReconnect(false).
		SetUsername(username).
		SetPassword(password)
	c := paho.NewClient(opts)
	defer c.Disconnect(50)

	tok := c.Connect()
	require.True(t, tok.WaitTimeout(3*time.Second), "MQTT connect token did not complete")
	require.Error(t, tok.Error(), "MQTT broker should have rejected credentials %q/%q", username, password)
}

// E: MQTT 認証
//
// none モードと password モードそれぞれで、正規/不正なクレデンシャルが
// 期待どおり受理・拒否されることを確認する。
func TestIntegration_MQTTAuth(t *testing.T) {
	t.Run("none mode accepts no credentials", func(t *testing.T) {
		resetViper(t)
		mqttTCP := localAddr(freeTCPPort(t))
		_, cleanup := startBrokerForTest(t, mqttTCP, func() {
			viper.Set("MQTT.auth.mode", "none")
		})
		defer cleanup()

		c := pahoConnectExpectingSuccess(t, mqttTCP, "auth-none-empty", "", "")
		c.Disconnect(50)
	})

	t.Run("none mode accepts arbitrary credentials", func(t *testing.T) {
		resetViper(t)
		mqttTCP := localAddr(freeTCPPort(t))
		_, cleanup := startBrokerForTest(t, mqttTCP, func() {
			viper.Set("MQTT.auth.mode", "none")
		})
		defer cleanup()

		c := pahoConnectExpectingSuccess(t, mqttTCP, "auth-none-junk", "anyone", "anything")
		c.Disconnect(50)
	})

	t.Run("password mode accepts valid credentials", func(t *testing.T) {
		resetViper(t)
		mqttTCP := localAddr(freeTCPPort(t))
		_, cleanup := startBrokerForTest(t, mqttTCP, func() {
			viper.Set("MQTT.auth", map[string]interface{}{
				"mode": "password",
				"users": []map[string]string{
					{"username": "alice", "password": "alice-pw"},
					{"username": "bob", "password": "bob-pw"},
				},
			})
		})
		defer cleanup()

		c := pahoConnectExpectingSuccess(t, mqttTCP, "auth-pw-valid", "alice", "alice-pw")
		c.Disconnect(50)
	})

	t.Run("password mode rejects wrong password", func(t *testing.T) {
		resetViper(t)
		mqttTCP := localAddr(freeTCPPort(t))
		_, cleanup := startBrokerForTest(t, mqttTCP, func() {
			viper.Set("MQTT.auth", map[string]interface{}{
				"mode": "password",
				"users": []map[string]string{
					{"username": "alice", "password": "alice-pw"},
				},
			})
		})
		defer cleanup()

		pahoConnectExpectingRejection(t, mqttTCP, "auth-pw-wrong", "alice", "wrong-password")
	})

	t.Run("password mode rejects unknown user", func(t *testing.T) {
		resetViper(t)
		mqttTCP := localAddr(freeTCPPort(t))
		_, cleanup := startBrokerForTest(t, mqttTCP, func() {
			viper.Set("MQTT.auth", map[string]interface{}{
				"mode": "password",
				"users": []map[string]string{
					{"username": "alice", "password": "alice-pw"},
				},
			})
		})
		defer cleanup()

		pahoConnectExpectingRejection(t, mqttTCP, "auth-pw-unknown", "carol", "carol-pw")
	})
}

// F: forwarding 無効化
//
// auth.mode=none かつ allow_unauthenticated_forwards=false の組み合わせでは、
// MQTT publish が UDP 転送先に到達しないことを確認する。
// （未認証クライアントの publish が UDP ストリームへ任意注入されることを防ぐ仕様）
func TestIntegration_ForwardingDisabledWhenUnauthenticated(t *testing.T) {
	resetViper(t)

	mqttTCP := localAddr(freeTCPPort(t))
	mqttWS := localAddr(freeTCPPort(t))
	forwardPort := freeUDPPort(t)
	forwardAddr := localAddr(forwardPort)

	const topic = "test/integration/forwarding-disabled"

	viper.Set("UDP.forwards", map[string][]string{forwardAddr: {topic}})
	viper.Set("UDP.allow_unauthenticated_forwards", false)
	viper.Set("MQTT.tcp", mqttTCP)
	viper.Set("MQTT.websocket", mqttWS)
	viper.Set("MQTT.auth.mode", "none")

	udpRecv, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: forwardPort})
	require.NoError(t, err)
	defer udpRecv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var wg sync.WaitGroup
	errCh := make(chan error, 1)
	msgCh := make(chan UdpMessage, 1)

	wg.Add(1)
	go startMQTTBroker(ctx, &wg, errCh, msgCh)

	waitForTCP(t, mqttTCP, 2*time.Second)

	c := pahoConnectExpectingSuccess(t, mqttTCP, "fwd-disabled-pub", "", "")
	defer c.Disconnect(250)

	pubToken := c.Publish(topic, 1, false, "should-not-be-forwarded")
	require.True(t, pubToken.WaitTimeout(2*time.Second))
	require.NoError(t, pubToken.Error())

	// 転送先 UDP ソケットでパケットが来ないことを assert (短時間タイムアウト)
	require.NoError(t, udpRecv.SetReadDeadline(time.Now().Add(500*time.Millisecond)))
	buf := make([]byte, 1024)
	n, _, readErr := udpRecv.ReadFromUDP(buf)

	if readErr == nil {
		t.Fatalf("forwarding should be disabled but received UDP packet: %q", buf[:n])
	}
	var ne net.Error
	require.True(t, errors.As(readErr, &ne) && ne.Timeout(),
		"expected timeout (no packet), got: %v", readErr)
	assert.Equal(t, 0, n)

	cancel()
	waitWG(t, &wg, 3*time.Second)
	drainErr(t, errCh)
}
