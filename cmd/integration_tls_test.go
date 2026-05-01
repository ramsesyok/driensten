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
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"math/big"
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

// generateSelfSignedCert は ECDSA P-256 鍵で自己署名 X.509 証明書を生成する。
// hosts は SubjectAlternativeName として埋め込まれる
// (IP リテラルなら IPAddresses、それ以外は DNSNames として登録)。
func generateSelfSignedCert(t *testing.T, hosts []string) (certPEM, keyPEM []byte, parsed *x509.Certificate) {
	t.Helper()

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: "driensten-integration-test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, h)
		}
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	require.NoError(t, err)

	parsed, err = x509.ParseCertificate(der)
	require.NoError(t, err)

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})

	keyDER, err := x509.MarshalECPrivateKey(priv)
	require.NoError(t, err)
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	return
}

// writeCertFiles は cert/key の PEM をテンポラリディレクトリに書き出してパスを返す。
func writeCertFiles(t *testing.T, certPEM, keyPEM []byte) (certPath, keyPath string) {
	t.Helper()
	dir := t.TempDir()
	certPath = filepath.Join(dir, "cert.pem")
	keyPath = filepath.Join(dir, "key.pem")
	require.NoError(t, os.WriteFile(certPath, certPEM, 0o600))
	require.NoError(t, os.WriteFile(keyPath, keyPEM, 0o600))
	return
}

// H: HTTP TLS 配信
//
// 自己署名 cert で startWebServer を TLS 起動し、
// その cert を root CA として信頼するクライアントから https GET できることを確認する。
func TestIntegration_HTTPTLSStaticServing(t *testing.T) {
	resetViper(t)

	distDir := t.TempDir()
	contents := []byte("tls hello")
	require.NoError(t, os.WriteFile(filepath.Join(distDir, "tls.txt"), contents, 0o644))

	certPEM, keyPEM, parsedCert := generateSelfSignedCert(t, []string{"127.0.0.1"})
	certPath, keyPath := writeCertFiles(t, certPEM, keyPEM)

	addr := localAddr(freeTCPPort(t))
	viper.Set("HTTP.listen", addr)
	viper.Set("HTTP.root", distDir)
	viper.Set("HTTP.tls.enable", true)
	viper.Set("HTTP.tls.cert", certPath)
	viper.Set("HTTP.tls.key", keyPath)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var wg sync.WaitGroup
	errCh := make(chan error, 1)

	wg.Add(1)
	go startWebServer(ctx, &wg, errCh)

	waitForTCP(t, addr, 2*time.Second)

	pool := x509.NewCertPool()
	pool.AddCert(parsedCert)
	httpClient := &http.Client{
		Timeout: 3 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{RootCAs: pool},
		},
	}

	resp, err := httpClient.Get("https://" + addr + "/tls.txt")
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

// I-a: MQTT TLS トランスポート
//
// 自己署名 cert で MQTT broker を TLS 起動し、paho クライアントが
// tls:// で接続して publish/subscribe の往復ができることを確認する。
//
// 注: 本テストは TLS による通信路の暗号化のみを検証する。
// auth.mode="cert" のクライアント証明書認証は現状の startMQTTBroker が
// tls.Config に ClientAuth/ClientCAs を設定しないため動作しない。
// 当該検証は本 PR の対象外とし、別 PR でコード修正と合わせて対応する。
func TestIntegration_MQTTTLSTransport(t *testing.T) {
	resetViper(t)

	certPEM, keyPEM, parsedCert := generateSelfSignedCert(t, []string{"127.0.0.1"})
	certPath, keyPath := writeCertFiles(t, certPEM, keyPEM)

	mqttTCP := localAddr(freeTCPPort(t))
	mqttWS := localAddr(freeTCPPort(t))
	forwardAddr := localAddr(freeUDPPort(t))

	const topic = "test/integration/mqtt-tls"
	const payload = "tls-payload"

	viper.Set("UDP.forwards", map[string][]string{forwardAddr: {topic}})
	viper.Set("UDP.allow_unauthenticated_forwards", true)
	viper.Set("MQTT.tcp", mqttTCP)
	viper.Set("MQTT.websocket", mqttWS)
	viper.Set("MQTT.tls.enable", true)
	viper.Set("MQTT.tls.cert", certPath)
	viper.Set("MQTT.tls.key", keyPath)
	viper.Set("MQTT.auth.mode", "none")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var wg sync.WaitGroup
	errCh := make(chan error, 1)
	msgCh := make(chan UdpMessage, 1)

	wg.Add(1)
	go startMQTTBroker(ctx, &wg, errCh, msgCh)

	waitForTCP(t, mqttTCP, 2*time.Second)

	pool := x509.NewCertPool()
	pool.AddCert(parsedCert)

	opts := paho.NewClientOptions().
		AddBroker("tls://" + mqttTCP).
		SetClientID("test-mqtt-tls").
		SetTLSConfig(&tls.Config{RootCAs: pool, ServerName: "127.0.0.1"}).
		SetConnectTimeout(3 * time.Second).
		SetAutoReconnect(false)

	client := paho.NewClient(opts)
	connectToken := client.Connect()
	require.True(t, connectToken.WaitTimeout(3*time.Second), "MQTT TLS connect timeout")
	require.NoError(t, connectToken.Error())
	defer client.Disconnect(250)

	received := make(chan paho.Message, 1)
	subToken := client.Subscribe(topic, 1, func(_ paho.Client, m paho.Message) {
		received <- m
	})
	require.True(t, subToken.WaitTimeout(2*time.Second), "MQTT TLS subscribe timeout")
	require.NoError(t, subToken.Error())

	pubToken := client.Publish(topic, 1, false, payload)
	require.True(t, pubToken.WaitTimeout(2*time.Second), "MQTT TLS publish timeout")
	require.NoError(t, pubToken.Error())

	select {
	case m := <-received:
		assert.Equal(t, topic, m.Topic())
		assert.Equal(t, payload, string(m.Payload()))
	case <-time.After(3 * time.Second):
		t.Fatal("did not receive message over TLS")
	}

	cancel()
	waitWG(t, &wg, 3*time.Second)
	drainErr(t, errCh)
}
