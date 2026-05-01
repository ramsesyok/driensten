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
	"github.com/spf13/cobra"
)

// runCmdは、HTTP/MQTT/UDP の各サービスを起動するコマンドを表します。
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "HTTP/MQTT/UDP の各サービスを起動します",
	Long: `本サービスは、HTTPサーバとMQTTブローカー機能を提供します。
MQTTは、UDP通信を中継することができ、指定ポートにUDPでメッセージを送付すると、MQTTによってPublishされます。
一方、設定ファイルで指定したトピック情報は、UDPとして送信されます。`,
	Run: func(cmd *cobra.Command, args []string) {
		execute()
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
}
