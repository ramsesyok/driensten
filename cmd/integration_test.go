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
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A: HTTP 静的配信
//
// 一時ディレクトリに配置したファイルを、startWebServer 経由で取得できることを確認する。
// 終了時は ctx キャンセルで goroutine が wg.Done() するまで待機する。
func TestIntegration_HTTPStaticServing(t *testing.T) {
	resetViper(t)

	distDir := t.TempDir()
	contents := []byte("hello driensten")
	require.NoError(t, os.WriteFile(filepath.Join(distDir, "hello.txt"), contents, 0o644))

	addr := localAddr(freeTCPPort(t))
	viper.Set("HTTP.listen", addr)
	viper.Set("HTTP.root", distDir)
	viper.Set("HTTP.tls.enable", false)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var wg sync.WaitGroup
	errCh := make(chan error, 1)

	wg.Add(1)
	go startWebServer(ctx, &wg, errCh)

	waitForTCP(t, addr, 2*time.Second)

	resp, err := http.Get(fmt.Sprintf("http://%s/hello.txt", addr))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, contents, body)

	cancel()
	waitWG(t, &wg, 3*time.Second)
	drainErr(t, errCh)
}

// B: UDP→MQTT 転送
//
// UDP リスナーへ "<topic>\n<payload>" を送信し、MQTT subscriber が同じトピック・
// 同じペイロードで受信することを確認する。
func TestIntegration_UDPtoMQTTRelay(t *testing.T) {
	resetViper(t)

	udpListenAddr := localAddr(freeUDPPort(t))
	mqttTCP := localAddr(freeTCPPort(t))
	mqttWS := localAddr(freeTCPPort(t))
	// 転送先 UDP は本テストでは使わないが、forwards に登録しないと
	// MQTT 側のトピック許可リストに入らないため設定する。
	forwardAddr := localAddr(freeUDPPort(t))

	const topic = "test/integration/udp-to-mqtt"
	const payload = "udp-payload-body"

	viper.Set("UDP.listen", udpListenAddr)
	viper.Set("UDP.forwards", map[string][]string{forwardAddr: {topic}})
	viper.Set("UDP.allow_unauthenticated_forwards", true)
	viper.Set("MQTT.tcp", mqttTCP)
	viper.Set("MQTT.websocket", mqttWS)
	viper.Set("MQTT.auth.mode", "none")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var wg sync.WaitGroup
	errCh := make(chan error, 2)
	msgCh := make(chan UdpMessage, 1)

	wg.Add(2)
	go startMQTTBroker(ctx, &wg, errCh, msgCh)
	go startUDPListener(ctx, &wg, errCh, msgCh)

	waitForTCP(t, mqttTCP, 2*time.Second)

	opts := paho.NewClientOptions().
		AddBroker("tcp://" + mqttTCP).
		SetClientID("test-sub-udp2mqtt").
		SetConnectTimeout(2 * time.Second).
		SetAutoReconnect(false)
	client := paho.NewClient(opts)
	connectToken := client.Connect()
	require.True(t, connectToken.WaitTimeout(2*time.Second), "MQTT connect timeout")
	require.NoError(t, connectToken.Error())
	defer client.Disconnect(250)

	received := make(chan paho.Message, 1)
	subToken := client.Subscribe(topic, 1, func(_ paho.Client, m paho.Message) {
		received <- m
	})
	require.True(t, subToken.WaitTimeout(2*time.Second), "MQTT subscribe timeout")
	require.NoError(t, subToken.Error())

	// UDP 送信
	udpAddr, err := net.ResolveUDPAddr("udp", udpListenAddr)
	require.NoError(t, err)
	conn, err := net.DialUDP("udp", nil, udpAddr)
	require.NoError(t, err)
	defer conn.Close()
	_, err = conn.Write([]byte(topic + "\n" + payload))
	require.NoError(t, err)

	select {
	case m := <-received:
		assert.Equal(t, topic, m.Topic())
		assert.Equal(t, payload, string(m.Payload()))
	case <-time.After(3 * time.Second):
		t.Fatal("MQTT subscriber did not receive forwarded message")
	}

	cancel()
	waitWG(t, &wg, 3*time.Second)
	drainErr(t, errCh)
}

// C: MQTT→UDP 転送
//
// MQTT publisher がトピックに publish したペイロードが、
// 設定した UDP 転送先へ "<topic>\n<payload>" 形式で配送されることを確認する。
func TestIntegration_MQTTtoUDPRelay(t *testing.T) {
	resetViper(t)

	mqttTCP := localAddr(freeTCPPort(t))
	mqttWS := localAddr(freeTCPPort(t))
	forwardPort := freeUDPPort(t)
	forwardAddr := localAddr(forwardPort)

	const topic = "test/integration/mqtt-to-udp"
	const payload = "mqtt-payload-body"

	viper.Set("UDP.forwards", map[string][]string{forwardAddr: {topic}})
	viper.Set("UDP.allow_unauthenticated_forwards", true)
	viper.Set("MQTT.tcp", mqttTCP)
	viper.Set("MQTT.websocket", mqttWS)
	viper.Set("MQTT.auth.mode", "none")

	// 転送 UDP の受信ソケットを先に開く
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

	opts := paho.NewClientOptions().
		AddBroker("tcp://" + mqttTCP).
		SetClientID("test-pub-mqtt2udp").
		SetConnectTimeout(2 * time.Second).
		SetAutoReconnect(false)
	client := paho.NewClient(opts)
	connectToken := client.Connect()
	require.True(t, connectToken.WaitTimeout(2*time.Second), "MQTT connect timeout")
	require.NoError(t, connectToken.Error())
	defer client.Disconnect(250)

	pubToken := client.Publish(topic, 1, false, payload)
	require.True(t, pubToken.WaitTimeout(2*time.Second), "MQTT publish timeout")
	require.NoError(t, pubToken.Error())

	require.NoError(t, udpRecv.SetReadDeadline(time.Now().Add(3*time.Second)))
	buf := make([]byte, 2048)
	n, _, err := udpRecv.ReadFromUDP(buf)
	require.NoError(t, err, "did not receive forwarded UDP packet")

	expected := []byte(topic + "\n" + payload)
	assert.Equal(t, expected, buf[:n])

	cancel()
	waitWG(t, &wg, 3*time.Second)
	drainErr(t, errCh)
}

// D: グレースフルシャットダウン
//
// HTTP / MQTT / UDP の 3 サービスを並行起動し、ctx キャンセルで
// 全 goroutine が期間内に終了することを確認する。
// 後続の取り直しでポートがリリースされていることも合わせて確認する。
func TestIntegration_GracefulShutdown(t *testing.T) {
	resetViper(t)

	distDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(distDir, "ok.txt"), []byte("ok"), 0o644))

	httpPort := freeTCPPort(t)
	udpPort := freeUDPPort(t)
	mqttTCPPort := freeTCPPort(t)
	mqttWSPort := freeTCPPort(t)

	httpAddr := localAddr(httpPort)
	udpListen := localAddr(udpPort)
	mqttTCP := localAddr(mqttTCPPort)
	mqttWS := localAddr(mqttWSPort)

	viper.Set("HTTP.listen", httpAddr)
	viper.Set("HTTP.root", distDir)
	viper.Set("HTTP.tls.enable", false)
	viper.Set("UDP.listen", udpListen)
	viper.Set("MQTT.tcp", mqttTCP)
	viper.Set("MQTT.websocket", mqttWS)
	viper.Set("MQTT.auth.mode", "none")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var wg sync.WaitGroup
	errCh := make(chan error, 3)
	msgCh := make(chan UdpMessage, 1)

	wg.Add(3)
	go startWebServer(ctx, &wg, errCh)
	go startMQTTBroker(ctx, &wg, errCh, msgCh)
	go startUDPListener(ctx, &wg, errCh, msgCh)

	waitForTCP(t, httpAddr, 2*time.Second)
	waitForTCP(t, mqttTCP, 2*time.Second)

	cancel()
	waitWG(t, &wg, 5*time.Second)

	// 全サービスが終了していれば、各ポートを再バインドできる
	tcpHTTP, err := net.Listen("tcp", httpAddr)
	require.NoError(t, err, "HTTP port not released after shutdown")
	require.NoError(t, tcpHTTP.Close())

	tcpMQTT, err := net.Listen("tcp", mqttTCP)
	require.NoError(t, err, "MQTT TCP port not released after shutdown")
	require.NoError(t, tcpMQTT.Close())

	udpReBind, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: udpPort})
	require.NoError(t, err, "UDP port not released after shutdown")
	require.NoError(t, udpReBind.Close())

	drainErr(t, errCh)
}
