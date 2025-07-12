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
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

// rootCmdは、サブコマンドを指定せずに呼び出されたときの基本コマンドを表します
var rootCmd = &cobra.Command{
	Use:   "driensten",
	Short: "HTTP & `MQTT relayed over UDP` service",
	Long: `本サービスは、HTTPサーバとMQTTブローカー機能を提供します。
MQTTは、UDP通信を中継することができ、指定ポートにUDPでメッセージを送付すると、MQTTによってPublishされます。
一方、設定ファイルで指定したトピック情報は、UDPとして送信されます。`,
	Run: func(cmd *cobra.Command, args []string) {
		execute()
	},
}

// Execute はすべての子コマンドを root コマンドに追加し、フラグを適切に設定します。
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	cobra.MousetrapHelpText = ""
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (デフォルトは、実行ファイル直下の driensten.yaml)")
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
}

// initConfig は、設定ファイルを読み込み、設定されていれば環境変数も読み込みます。
func initConfig() {
	viper.SetDefault("HTTP.listen", "127.0.0.1:8080")
	viper.SetDefault("HTTP.root", "dist")
	viper.SetDefault("MQTT.tcp", "127.0.0.1:1883")
	viper.SetDefault("MQTT.websocket", "127.0.0.1:6565")
	viper.SetDefault("UDP.listen", "127.0.0.1:5656")
	if cfgFile != "" {
		// フラグで指定された設定ファイルを使用します。
		viper.SetConfigFile(cfgFile)
	} else {
		// 実行ファイルのパスを取得します。
		exePath, err := os.Executable()
		cobra.CheckErr(err)
		// 実行ファイルのディレクトリを取得します。
		exeDir := filepath.Dir(exePath)

		// 実行ファイルのディレクトリ内で、名前が "driensten"（拡張子なし）の設定ファイルを検索します。
		viper.AddConfigPath(exeDir)
		viper.SetConfigType("yaml")
		viper.SetConfigName("driensten")
		cfgFile = filepath.Join(exeDir, "driensten.yaml")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "設定ファイルを利用します:", viper.ConfigFileUsed())
	} else {
		fmt.Fprintln(os.Stderr, "設定ファイルが見つかりません:", cfgFile)
		os.Exit(-1)
	}
}
