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
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseUdpMessage(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		wantMsg UdpMessage
		wantOk  bool
	}{
		{
			name:    "valid topic and payload",
			input:   []byte("sensor/temp\n23.5"),
			wantMsg: UdpMessage{Topic: "sensor/temp", Payload: "23.5"},
			wantOk:  true,
		},
		{
			name:    "payload contains newlines preserved as-is",
			input:   []byte("topic\nline1\nline2"),
			wantMsg: UdpMessage{Topic: "topic", Payload: "line1\nline2"},
			wantOk:  true,
		},
		{
			name:    "empty payload after newline",
			input:   []byte("topic\n"),
			wantMsg: UdpMessage{Topic: "topic", Payload: ""},
			wantOk:  true,
		},
		{
			name:    "binary payload",
			input:   append([]byte("topic\n"), 0x00, 0x01, 0xff, 0xfe),
			wantMsg: UdpMessage{Topic: "topic", Payload: string([]byte{0x00, 0x01, 0xff, 0xfe})},
			wantOk:  true,
		},
		{
			name:    "single level topic",
			input:   []byte("a\nb"),
			wantMsg: UdpMessage{Topic: "a", Payload: "b"},
			wantOk:  true,
		},
		{
			name:    "multi-level topic",
			input:   []byte("a/b/c/d\nx"),
			wantMsg: UdpMessage{Topic: "a/b/c/d", Payload: "x"},
			wantOk:  true,
		},
		{
			name:    "no newline rejects",
			input:   []byte("topiconly"),
			wantMsg: UdpMessage{},
			wantOk:  false,
		},
		{
			name:    "empty topic rejects",
			input:   []byte("\npayload"),
			wantMsg: UdpMessage{},
			wantOk:  false,
		},
		{
			name:    "newline only rejects",
			input:   []byte("\n"),
			wantMsg: UdpMessage{},
			wantOk:  false,
		},
		{
			name:    "empty input rejects",
			input:   []byte(""),
			wantMsg: UdpMessage{},
			wantOk:  false,
		},
		{
			name:    "nil input rejects",
			input:   nil,
			wantMsg: UdpMessage{},
			wantOk:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMsg, gotOk := parseUdpMessage(tt.input)
			require.Equal(t, tt.wantOk, gotOk, "ok flag mismatch")
			assert.Equal(t, tt.wantMsg, gotMsg)
		})
	}
}

func TestResolveUdpListenAddr(t *testing.T) {
	t.Run("valid IPv4 address", func(t *testing.T) {
		got, err := resolveUdpListenAddr("127.0.0.1:6565")
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.True(t, got.IP.Equal(net.ParseIP("127.0.0.1")))
		assert.Equal(t, 6565, got.Port)
	})

	t.Run("valid IPv6 address", func(t *testing.T) {
		got, err := resolveUdpListenAddr("[::1]:6565")
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.True(t, got.IP.Equal(net.ParseIP("::1")))
		assert.Equal(t, 6565, got.Port)
	})

	t.Run("zero IP wildcard", func(t *testing.T) {
		got, err := resolveUdpListenAddr("0.0.0.0:6565")
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.True(t, got.IP.Equal(net.IPv4zero))
		assert.Equal(t, 6565, got.Port)
	})

	t.Run("port zero is allowed (ephemeral)", func(t *testing.T) {
		got, err := resolveUdpListenAddr("127.0.0.1:0")
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, 0, got.Port)
	})

	t.Run("missing port returns error", func(t *testing.T) {
		_, err := resolveUdpListenAddr("127.0.0.1")
		require.Error(t, err)
	})

	t.Run("non-numeric port returns error", func(t *testing.T) {
		_, err := resolveUdpListenAddr("127.0.0.1:abc")
		require.Error(t, err)
	})

	t.Run("empty string returns error", func(t *testing.T) {
		_, err := resolveUdpListenAddr("")
		require.Error(t, err)
	})

	t.Run("hostname yields nil IP (documented quirk: listens on all interfaces)", func(t *testing.T) {
		// net.ParseIP は IP リテラル以外を受け付けないため "localhost" は nil となる。
		// その結果 net.ListenUDP は全インタフェースで待ち受けに格上げされる点に注意。
		got, err := resolveUdpListenAddr("localhost:6565")
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Nil(t, got.IP, "hostnames silently lose IP — surfaced for visibility")
		assert.Equal(t, 6565, got.Port)
	})
}
