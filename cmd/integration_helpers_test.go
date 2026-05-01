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
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

// freeTCPPort は OS が割り当てた利用可能な TCP ポート番号を返す。
// 取得後すぐに Close するため、再利用までに微小な競合の余地はあるが
// テスト用途では十分。
func freeTCPPort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := l.Addr().(*net.TCPAddr).Port
	require.NoError(t, l.Close())
	return port
}

// freeUDPPort は OS が割り当てた利用可能な UDP ポート番号を返す。
func freeUDPPort(t *testing.T) int {
	t.Helper()
	c, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	require.NoError(t, err)
	port := c.LocalAddr().(*net.UDPAddr).Port
	require.NoError(t, c.Close())
	return port
}

// resetViper はテスト間で viper のグローバル状態を初期化する。
// 統合テストは並列実行不可。
func resetViper(t *testing.T) {
	t.Helper()
	viper.Reset()
	t.Cleanup(viper.Reset)
}

// localAddr は 127.0.0.1:<port> 形式の文字列を返す。
func localAddr(port int) string {
	return fmt.Sprintf("127.0.0.1:%d", port)
}

// waitForTCP は指定アドレスへ TCP 接続が成功するまでポーリングする。
// timeout を超えるとテストを失敗させる。
func waitForTCP(t *testing.T, addr string, timeout time.Duration) {
	t.Helper()
	require.Eventually(t, func() bool {
		c, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err != nil {
			return false
		}
		_ = c.Close()
		return true
	}, timeout, 50*time.Millisecond, "no TCP listener at %s", addr)
}

// waitWG は wg.Wait() の完了をタイムアウト付きで待つ。
// 期間内に完了しなければテストを失敗させる。
func waitWG(t *testing.T, wg *sync.WaitGroup, timeout time.Duration) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(timeout):
		t.Fatalf("WaitGroup did not complete within %s", timeout)
	}
}

// drainErr は errCh が空であることを確認する。
// ノンブロッキングで一度だけ読み出し、エラーが入っていればテストを失敗させる。
func drainErr(t *testing.T, errCh <-chan error) {
	t.Helper()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("unexpected service error: %v", err)
		}
	default:
	}
}
