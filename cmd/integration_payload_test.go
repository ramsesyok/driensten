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
	"net"
	"sync"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// G: 不正な UDP ペイロード
//
// 改行なし / 空トピックを含む不正パケットを送信しても UDP リスナーが
// 落ちず、後続の正常パケットは引き続き msgCh に流れることを確認する。
func TestIntegration_UDPInvalidPayloadDoesNotCrash(t *testing.T) {
	resetViper(t)

	udpListenAddr := localAddr(freeUDPPort(t))
	viper.Set("UDP.listen", udpListenAddr)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var wg sync.WaitGroup
	errCh := make(chan error, 1)
	msgCh := make(chan UdpMessage, 4)

	wg.Add(1)
	go startUDPListener(ctx, &wg, errCh, msgCh)

	// リスナーが listen を開始するまで少し待つ。
	// UDP は connectionless だが、ListenUDP の bind 完了を待たないと
	// 送信パケットが破棄される可能性がある。
	require.Eventually(t, func() bool {
		c, err := net.DialTimeout("udp", udpListenAddr, 100*time.Millisecond)
		if err != nil {
			return false
		}
		_ = c.Close()
		return true
	}, 2*time.Second, 50*time.Millisecond)

	udpAddr, err := net.ResolveUDPAddr("udp", udpListenAddr)
	require.NoError(t, err)
	conn, err := net.DialUDP("udp", nil, udpAddr)
	require.NoError(t, err)
	defer conn.Close()

	// 不正パケットを送る
	invalidPayloads := [][]byte{
		[]byte("no-newline-here"),     // 改行なし
		[]byte("\nempty-topic"),       // 空トピック
		[]byte(""),                    // 完全に空
		[]byte("\n"),                  // 改行のみ
	}
	for _, p := range invalidPayloads {
		_, err := conn.Write(p)
		require.NoError(t, err)
	}

	// 不正パケット送信後に msgCh に何も流れていないこと
	select {
	case msg := <-msgCh:
		t.Fatalf("invalid payload should not produce message, got: %#v", msg)
	case <-time.After(300 * time.Millisecond):
		// expected: nothing arrives
	}

	// 続けて正常パケットを送り、リスナーがまだ生きていることを確認
	const validTopic = "test/integration/valid-after-invalid"
	const validPayload = "still-alive"
	_, err = conn.Write([]byte(validTopic + "\n" + validPayload))
	require.NoError(t, err)

	select {
	case msg := <-msgCh:
		assert.Equal(t, validTopic, msg.Topic)
		assert.Equal(t, validPayload, msg.Payload)
	case <-time.After(2 * time.Second):
		t.Fatal("listener appears to have died: valid packet not received after invalid ones")
	}

	// 念のためもう一度正常パケット (連続稼働の確認)
	const validTopic2 = "test/integration/second-valid"
	const validPayload2 = "second"
	_, err = conn.Write([]byte(validTopic2 + "\n" + validPayload2))
	require.NoError(t, err)

	select {
	case msg := <-msgCh:
		assert.Equal(t, validTopic2, msg.Topic)
		assert.Equal(t, validPayload2, msg.Payload)
	case <-time.After(2 * time.Second):
		t.Fatal("listener died after second valid packet")
	}

	cancel()
	waitWG(t, &wg, 3*time.Second)
	drainErr(t, errCh)
}
