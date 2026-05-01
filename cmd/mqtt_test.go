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
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMqttTopicMatch(t *testing.T) {
	tests := []struct {
		name   string
		filter string
		topic  string
		want   bool
	}{
		// 完全一致
		{name: "exact single level", filter: "foo", topic: "foo", want: true},
		{name: "exact multi level", filter: "a/b/c", topic: "a/b/c", want: true},
		{name: "case sensitive mismatch", filter: "Foo", topic: "foo", want: false},

		// レベル数の不一致
		{name: "filter has fewer levels", filter: "a/b", topic: "a/b/c", want: false},
		{name: "filter has more levels", filter: "a/b/c", topic: "a/b", want: false},
		{name: "first segment differs", filter: "a/b", topic: "x/b", want: false},
		{name: "middle segment differs", filter: "a/b/c", topic: "a/x/c", want: false},

		// + (単一レベルワイルドカード)
		{name: "+ matches single level middle", filter: "a/+/c", topic: "a/b/c", want: true},
		{name: "+ matches single level head", filter: "+/b", topic: "a/b", want: true},
		{name: "+ matches single level tail", filter: "a/+", topic: "a/b", want: true},
		{name: "+ does not match multiple levels", filter: "a/+", topic: "a/b/c", want: false},
		{name: "+ does not match missing level", filter: "a/+", topic: "a", want: false},
		{name: "multiple + match", filter: "+/+/+", topic: "x/y/z", want: true},
		{name: "multiple + with literal", filter: "+/b/+", topic: "a/b/c", want: true},
		{name: "+ does not match literal mismatch", filter: "+/b", topic: "a/c", want: false},

		// # (マルチレベルワイルドカード)
		{name: "# alone matches single", filter: "#", topic: "a", want: true},
		{name: "# alone matches multi", filter: "#", topic: "a/b/c", want: true},
		{name: "trailing # matches deeper", filter: "a/#", topic: "a/b/c", want: true},
		{name: "trailing # matches one level", filter: "a/#", topic: "a/b", want: true},
		{name: "trailing # matches parent itself", filter: "a/#", topic: "a", want: true},
		{name: "trailing # mismatch on prefix", filter: "a/#", topic: "x/b", want: false},

		// + と # の混在
		{name: "+ and # combined", filter: "a/+/#", topic: "a/b/c/d", want: true},
		{name: "+ and # combined parent only", filter: "a/+/#", topic: "a/b", want: true},
		{name: "+ and # combined missing wildcard level", filter: "a/+/#", topic: "a", want: false},

		// エッジケース
		{name: "empty filter and empty topic", filter: "", topic: "", want: true},
		{name: "empty filter vs non-empty", filter: "", topic: "a", want: false},
		{name: "non-empty filter vs empty topic", filter: "a", topic: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mqttTopicMatch(tt.filter, tt.topic)
			assert.Equalf(t, tt.want, got, "mqttTopicMatch(%q, %q)", tt.filter, tt.topic)
		})
	}
}

func TestMatchPasswordCredential(t *testing.T) {
	users := []mqttUser{
		{Username: "alice", Password: "alice-pw"},
		{Username: "bob", Password: "bob-pw"},
	}

	tests := []struct {
		name     string
		users    []mqttUser
		username string
		password string
		want     bool
	}{
		{name: "exact match first", users: users, username: "alice", password: "alice-pw", want: true},
		{name: "exact match second", users: users, username: "bob", password: "bob-pw", want: true},
		{name: "username matches but wrong password", users: users, username: "alice", password: "bob-pw", want: false},
		{name: "password matches but wrong username", users: users, username: "carol", password: "alice-pw", want: false},
		{name: "unknown user", users: users, username: "carol", password: "carol-pw", want: false},
		{name: "case sensitive username", users: users, username: "Alice", password: "alice-pw", want: false},
		{name: "case sensitive password", users: users, username: "alice", password: "Alice-PW", want: false},
		{name: "empty users list", users: nil, username: "alice", password: "alice-pw", want: false},
		{name: "empty credentials against populated users", users: users, username: "", password: "", want: false},
		{name: "empty credentials match empty entry", users: []mqttUser{{Username: "", Password: ""}}, username: "", password: "", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchPasswordCredential(tt.users, tt.username, tt.password)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestEncodeUdpMessage(t *testing.T) {
	t.Run("encode then parse round-trip", func(t *testing.T) {
		topic := "sensor/temp"
		payload := []byte("23.5")

		encoded := encodeUdpMessage(topic, payload)
		assert.Equal(t, []byte("sensor/temp\n23.5"), encoded)

		msg, ok := parseUdpMessage(encoded)
		require.True(t, ok)
		assert.Equal(t, topic, msg.Topic)
		assert.Equal(t, string(payload), msg.Payload)
	})

	t.Run("empty payload", func(t *testing.T) {
		encoded := encodeUdpMessage("topic", []byte{})
		assert.Equal(t, []byte("topic\n"), encoded)

		msg, ok := parseUdpMessage(encoded)
		require.True(t, ok)
		assert.Equal(t, "topic", msg.Topic)
		assert.Equal(t, "", msg.Payload)
	})

	t.Run("nil payload", func(t *testing.T) {
		encoded := encodeUdpMessage("topic", nil)
		assert.Equal(t, []byte("topic\n"), encoded)
	})

	t.Run("binary payload preserved", func(t *testing.T) {
		payload := []byte{0x00, 0x01, 0xff, 0xfe}
		encoded := encodeUdpMessage("topic", payload)
		assert.Equal(t, append([]byte("topic\n"), payload...), encoded)

		msg, ok := parseUdpMessage(encoded)
		require.True(t, ok)
		assert.Equal(t, "topic", msg.Topic)
		assert.Equal(t, string(payload), msg.Payload)
	})

	t.Run("payload containing newlines round-trips because parser splits only at first newline", func(t *testing.T) {
		payload := []byte("line1\nline2")
		encoded := encodeUdpMessage("topic", payload)

		msg, ok := parseUdpMessage(encoded)
		require.True(t, ok)
		assert.Equal(t, "topic", msg.Topic)
		assert.Equal(t, string(payload), msg.Payload)
	})

	t.Run("topic with newline breaks round-trip (documented quirk)", func(t *testing.T) {
		// topic 内に "\n" を含むと parseUdpMessage は最初の "\n" で切るため、
		// topic の後半は payload 側に取り込まれて復元できない。
		encoded := encodeUdpMessage("a\nb", []byte("c"))
		assert.Equal(t, []byte("a\nb\nc"), encoded)

		msg, ok := parseUdpMessage(encoded)
		require.True(t, ok)
		assert.Equal(t, "a", msg.Topic)
		assert.Equal(t, "b\nc", msg.Payload)
	})
}

func TestBuildTopicToAddressMap(t *testing.T) {
	t.Run("single address single topic", func(t *testing.T) {
		got := buildTopicToAddressMap(map[string][]string{
			"127.0.0.1:6000": {"sensor/temp"},
		})
		assert.Equal(t, map[string]string{"sensor/temp": "127.0.0.1:6000"}, got)
	})

	t.Run("single address multiple topics", func(t *testing.T) {
		got := buildTopicToAddressMap(map[string][]string{
			"127.0.0.1:6000": {"sensor/temp", "sensor/humidity"},
		})
		assert.Equal(t, map[string]string{
			"sensor/temp":     "127.0.0.1:6000",
			"sensor/humidity": "127.0.0.1:6000",
		}, got)
	})

	t.Run("multiple addresses different topics", func(t *testing.T) {
		got := buildTopicToAddressMap(map[string][]string{
			"127.0.0.1:6000": {"sensor/temp"},
			"127.0.0.1:6001": {"sensor/humidity"},
		})
		assert.Equal(t, map[string]string{
			"sensor/temp":     "127.0.0.1:6000",
			"sensor/humidity": "127.0.0.1:6001",
		}, got)
	})

	t.Run("empty input returns empty map (not nil)", func(t *testing.T) {
		got := buildTopicToAddressMap(map[string][]string{})
		require.NotNil(t, got)
		assert.Empty(t, got)
	})

	t.Run("nil input returns empty map", func(t *testing.T) {
		got := buildTopicToAddressMap(nil)
		require.NotNil(t, got)
		assert.Empty(t, got)
	})

	t.Run("empty topic list under address is skipped", func(t *testing.T) {
		got := buildTopicToAddressMap(map[string][]string{
			"127.0.0.1:6000": {},
		})
		assert.Empty(t, got)
	})
}

func TestValidateMQTTAuthConfig(t *testing.T) {
	tests := []struct {
		name      string
		mode      string
		tlsEnable bool
		caPath    string
		wantErr   bool
		errSubstr string
	}{
		{name: "none mode passes regardless", mode: "none", wantErr: false},
		{name: "password mode passes regardless", mode: "password", wantErr: false},
		{name: "empty mode passes (treated as none upstream)", mode: "", wantErr: false},
		{name: "unknown mode passes (only cert is validated)", mode: "future", wantErr: false},

		{name: "cert mode without TLS errors", mode: "cert", tlsEnable: false, caPath: "/some/ca.pem", wantErr: true, errSubstr: "MQTT.tls.enable"},
		{name: "cert mode without client_ca errors", mode: "cert", tlsEnable: true, caPath: "", wantErr: true, errSubstr: "MQTT.tls.client_ca"},
		{name: "cert mode without TLS or CA errors on TLS first", mode: "cert", tlsEnable: false, caPath: "", wantErr: true, errSubstr: "MQTT.tls.enable"},
		{name: "cert mode with TLS and CA passes", mode: "cert", tlsEnable: true, caPath: "/some/ca.pem", wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMQTTAuthConfig(tt.mode, tt.tlsEnable, tt.caPath)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errSubstr != "" {
					assert.Contains(t, err.Error(), tt.errSubstr)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// generateCertPEMForTest は単一の自己署名 X.509 証明書 PEM を返す。
// loadClientCAPool テスト用。integration ビルドタグで使う TLS ヘルパに頼らないため
// ここで直接生成する。
func generateCertPEMForTest(t *testing.T) []byte {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "unit-test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	require.NoError(t, err)
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func TestLoadClientCAPool(t *testing.T) {
	t.Run("nonexistent file returns error", func(t *testing.T) {
		_, err := loadClientCAPool(filepath.Join(t.TempDir(), "does-not-exist.pem"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "read client CA file")
	})

	t.Run("empty file returns error", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "empty.pem")
		require.NoError(t, os.WriteFile(path, []byte{}, 0o600))
		_, err := loadClientCAPool(path)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no valid certificates")
	})

	t.Run("garbage content returns error", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "garbage.pem")
		require.NoError(t, os.WriteFile(path, []byte("this is not a PEM"), 0o600))
		_, err := loadClientCAPool(path)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no valid certificates")
	})

	t.Run("valid CA PEM returns non-empty pool", func(t *testing.T) {
		pemBytes := generateCertPEMForTest(t)
		path := filepath.Join(t.TempDir(), "ca.pem")
		require.NoError(t, os.WriteFile(path, pemBytes, 0o600))

		pool, err := loadClientCAPool(path)
		require.NoError(t, err)
		require.NotNil(t, pool)
		// CertPool に内容が入っているか確認 (Equal で空 pool との非一致を見る)
		assert.False(t, pool.Equal(x509.NewCertPool()), "pool should not be empty")
	})

	t.Run("multiple CAs concatenated PEM are all loaded", func(t *testing.T) {
		pem1 := generateCertPEMForTest(t)
		pem2 := generateCertPEMForTest(t)
		combined := append(append([]byte{}, pem1...), pem2...)
		path := filepath.Join(t.TempDir(), "multi-ca.pem")
		require.NoError(t, os.WriteFile(path, combined, 0o600))

		pool, err := loadClientCAPool(path)
		require.NoError(t, err)
		require.NotNil(t, pool)
	})
}
