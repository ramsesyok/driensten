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
	"crypto/ecdsa"
	"crypto/tls"
	"crypto/x509"
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

// pahoTLSConnect は paho クライアントを TLS で接続する。
// expectError=true の場合、token に Error が乗ることを assert する。
func pahoTLSConnect(t *testing.T, brokerAddr, clientID string, tlsConf *tls.Config, expectError bool) paho.Client {
	t.Helper()
	opts := paho.NewClientOptions().
		AddBroker("tls://" + brokerAddr).
		SetClientID(clientID).
		SetTLSConfig(tlsConf).
		SetConnectTimeout(3 * time.Second).
		SetAutoReconnect(false)

	c := paho.NewClient(opts)
	tok := c.Connect()
	require.True(t, tok.WaitTimeout(3*time.Second), "connect token did not complete")
	if expectError {
		require.Error(t, tok.Error(), "connect should have failed")
	} else {
		require.NoError(t, tok.Error(), "connect should have succeeded")
	}
	return c
}

// buildClientTLSConfig は CA を信頼し、必要なら client cert を提示する設定を組み立てる。
// clientCert が nil の場合はクライアント証明書を提示しない。
func buildClientTLSConfig(rootPool *x509.CertPool, clientCert *x509.Certificate, clientKey *ecdsa.PrivateKey, serverName string) *tls.Config {
	conf := &tls.Config{
		RootCAs:    rootPool,
		ServerName: serverName,
	}
	if clientCert != nil {
		conf.Certificates = []tls.Certificate{
			{
				Certificate: [][]byte{clientCert.Raw},
				PrivateKey:  clientKey,
				Leaf:        clientCert,
			},
		}
	}
	return conf
}

// I-b: MQTT cert 認証
//
// CA → server cert + 正規 client cert + 別 CA 由来の不正 client cert を生成し、
// auth.mode=cert で起動した broker への paho 接続が以下のように振る舞うことを確認する:
//   - 正規クライアント証明書: 接続成功 + publish/subscribe 往復
//   - クライアント証明書なし: TLS ハンドシェイク失敗
//   - 別 CA で署名された証明書: TLS ハンドシェイク失敗
func TestIntegration_MQTTCertAuth(t *testing.T) {
	resetViper(t)

	// 信頼する CA とそれに紐づく server / client cert
	ca, caKey := generateTestCA(t)
	serverCert, serverKey := signLeafCert(t, ca, caKey, []string{"127.0.0.1"}, false)
	validClientCert, validClientKey := signLeafCert(t, ca, caKey, nil, true)

	// 信頼しない別 CA とその配下の client cert (拒否されるべき)
	rogueCA, rogueCAKey := generateTestCA(t)
	rogueClientCert, rogueClientKey := signLeafCert(t, rogueCA, rogueCAKey, nil, true)

	// ファイル配置
	dir := t.TempDir()
	serverCertPath := filepath.Join(dir, "server.crt")
	serverKeyPath := filepath.Join(dir, "server.key")
	caPath := filepath.Join(dir, "ca.pem")
	require.NoError(t, os.WriteFile(serverCertPath, certPEM(serverCert), 0o600))
	require.NoError(t, os.WriteFile(serverKeyPath, keyPEM(t, serverKey), 0o600))
	require.NoError(t, os.WriteFile(caPath, certPEM(ca), 0o600))

	mqttTCP := localAddr(freeTCPPort(t))
	mqttWS := localAddr(freeTCPPort(t))
	forwardAddr := localAddr(freeUDPPort(t))

	const topic = "test/integration/cert-auth"

	viper.Set("UDP.forwards", map[string][]string{forwardAddr: {topic}})
	viper.Set("UDP.allow_unauthenticated_forwards", false)
	viper.Set("MQTT.tcp", mqttTCP)
	viper.Set("MQTT.websocket", mqttWS)
	viper.Set("MQTT.tls.enable", true)
	viper.Set("MQTT.tls.cert", serverCertPath)
	viper.Set("MQTT.tls.key", serverKeyPath)
	viper.Set("MQTT.tls.client_ca", caPath)
	viper.Set("MQTT.auth.mode", "cert")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var wg sync.WaitGroup
	errCh := make(chan error, 1)
	msgCh := make(chan UdpMessage, 1)

	wg.Add(1)
	go startMQTTBroker(ctx, &wg, errCh, msgCh)
	waitForTCP(t, mqttTCP, 2*time.Second)

	rootPool := x509.NewCertPool()
	rootPool.AddCert(ca)

	t.Run("valid client cert is accepted with publish round-trip", func(t *testing.T) {
		conf := buildClientTLSConfig(rootPool, validClientCert, validClientKey, "127.0.0.1")
		c := pahoTLSConnect(t, mqttTCP, "cert-valid", conf, false)
		defer c.Disconnect(50)

		received := make(chan paho.Message, 1)
		subTok := c.Subscribe(topic, 1, func(_ paho.Client, m paho.Message) {
			received <- m
		})
		require.True(t, subTok.WaitTimeout(2*time.Second))
		require.NoError(t, subTok.Error())

		const payload = "cert-payload"
		pubTok := c.Publish(topic, 1, false, payload)
		require.True(t, pubTok.WaitTimeout(2*time.Second))
		require.NoError(t, pubTok.Error())

		select {
		case m := <-received:
			assert.Equal(t, topic, m.Topic())
			assert.Equal(t, payload, string(m.Payload()))
		case <-time.After(3 * time.Second):
			t.Fatal("did not receive published message over cert-authenticated TLS")
		}
	})

	t.Run("client without cert is rejected", func(t *testing.T) {
		conf := buildClientTLSConfig(rootPool, nil, nil, "127.0.0.1")
		_ = pahoTLSConnect(t, mqttTCP, "cert-none", conf, true)
	})

	t.Run("client cert from different CA is rejected", func(t *testing.T) {
		conf := buildClientTLSConfig(rootPool, rogueClientCert, rogueClientKey, "127.0.0.1")
		_ = pahoTLSConnect(t, mqttTCP, "cert-rogue", conf, true)
	})

	cancel()
	waitWG(t, &wg, 3*time.Second)
	drainErr(t, errCh)
}

// I-b 補完: cert 認証の設定不備で startMQTTBroker が起動時にエラーで終了する。
func TestIntegration_MQTTCertAuthMisconfigured(t *testing.T) {
	type runResult struct {
		err error
	}

	startAndCollectErr := func(t *testing.T, configure func()) runResult {
		t.Helper()
		resetViper(t)
		configure()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		var wg sync.WaitGroup
		errCh := make(chan error, 1)
		msgCh := make(chan UdpMessage, 1)

		wg.Add(1)
		go startMQTTBroker(ctx, &wg, errCh, msgCh)

		// broker は起動失敗で wg.Done() するはず
		waitWG(t, &wg, 2*time.Second)

		select {
		case err := <-errCh:
			return runResult{err: err}
		default:
			return runResult{err: nil}
		}
	}

	t.Run("cert mode without TLS errors at startup", func(t *testing.T) {
		mqttTCP := localAddr(freeTCPPort(t))
		mqttWS := localAddr(freeTCPPort(t))
		res := startAndCollectErr(t, func() {
			viper.Set("MQTT.tcp", mqttTCP)
			viper.Set("MQTT.websocket", mqttWS)
			viper.Set("MQTT.tls.enable", false)
			viper.Set("MQTT.auth.mode", "cert")
		})
		require.Error(t, res.err)
		assert.Contains(t, res.err.Error(), "MQTT.tls.enable")
	})

	t.Run("cert mode without client_ca errors at startup", func(t *testing.T) {
		ca, caKey := generateTestCA(t)
		serverCert, serverKey := signLeafCert(t, ca, caKey, []string{"127.0.0.1"}, false)
		certPath, keyPath := writeCertFiles(t, certPEM(serverCert), keyPEM(t, serverKey))

		mqttTCP := localAddr(freeTCPPort(t))
		mqttWS := localAddr(freeTCPPort(t))
		res := startAndCollectErr(t, func() {
			viper.Set("MQTT.tcp", mqttTCP)
			viper.Set("MQTT.websocket", mqttWS)
			viper.Set("MQTT.tls.enable", true)
			viper.Set("MQTT.tls.cert", certPath)
			viper.Set("MQTT.tls.key", keyPath)
			viper.Set("MQTT.auth.mode", "cert")
			// client_ca 未設定
		})
		require.Error(t, res.err)
		assert.Contains(t, res.err.Error(), "MQTT.tls.client_ca")
	})

	t.Run("cert mode with nonexistent client_ca errors at startup", func(t *testing.T) {
		ca, caKey := generateTestCA(t)
		serverCert, serverKey := signLeafCert(t, ca, caKey, []string{"127.0.0.1"}, false)
		certPath, keyPath := writeCertFiles(t, certPEM(serverCert), keyPEM(t, serverKey))

		mqttTCP := localAddr(freeTCPPort(t))
		mqttWS := localAddr(freeTCPPort(t))
		res := startAndCollectErr(t, func() {
			viper.Set("MQTT.tcp", mqttTCP)
			viper.Set("MQTT.websocket", mqttWS)
			viper.Set("MQTT.tls.enable", true)
			viper.Set("MQTT.tls.cert", certPath)
			viper.Set("MQTT.tls.key", keyPath)
			viper.Set("MQTT.tls.client_ca", filepath.Join(t.TempDir(), "nonexistent-ca.pem"))
			viper.Set("MQTT.auth.mode", "cert")
		})
		require.Error(t, res.err)
		assert.Contains(t, res.err.Error(), "client CA")
	})
}
