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
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"time"

	"github.com/spf13/cobra"
)

var (
	clientCACert  string
	clientCAKey   string
	clientOutCert string
	clientOutKey  string
	clientDays    int
	clientCN      string
)

var gencertClientCmd = &cobra.Command{
	Use:   "client",
	Short: "CAで署名したクライアント証明書と秘密鍵を生成します",
	Long: `指定したCAで署名したクライアント証明書と秘密鍵を生成します。
MQTT のクライアント証明書認証（auth.mode=cert）で使用します。

driensten.yaml の MQTT.tls.client_ca に CA 証明書のパスを設定し、
生成したクライアント証明書を MQTT クライアント（例: mosquitto_pub）に渡してください。

使用例:
  driensten gencert client
  driensten gencert client --cn my-device --out-cert device.crt --out-key device.key`,
	RunE: func(cmd *cobra.Command, args []string) error {
		caCert, caKey, err := loadCACert(clientCACert, clientCAKey)
		if err != nil {
			return err
		}

		key, err := generateECKey()
		if err != nil {
			return fmt.Errorf("鍵の生成に失敗: %w", err)
		}

		template := &x509.Certificate{
			SerialNumber: big.NewInt(time.Now().UnixNano()),
			Subject:      pkix.Name{CommonName: clientCN},
			NotBefore:    time.Now(),
			NotAfter:     time.Now().AddDate(0, 0, clientDays),
			KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
			ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		}

		der, err := x509.CreateCertificate(rand.Reader, template, caCert, &key.PublicKey, caKey)
		if err != nil {
			return fmt.Errorf("証明書の生成に失敗: %w", err)
		}
		cert, err := x509.ParseCertificate(der)
		if err != nil {
			return fmt.Errorf("証明書のパースに失敗: %w", err)
		}

		keyPEM, err := marshalKeyPEM(key)
		if err != nil {
			return fmt.Errorf("秘密鍵のエンコードに失敗: %w", err)
		}
		if err := writePEMFile(clientOutCert, marshalCertPEM(cert)); err != nil {
			return fmt.Errorf("%s の書き込みに失敗: %w", clientOutCert, err)
		}
		if err := writePEMFile(clientOutKey, keyPEM); err != nil {
			return fmt.Errorf("%s の書き込みに失敗: %w", clientOutKey, err)
		}

		fmt.Printf("クライアント証明書: %s\n", clientOutCert)
		fmt.Printf("クライアント秘密鍵: %s\n", clientOutKey)
		return nil
	},
}

func init() {
	gencertCmd.AddCommand(gencertClientCmd)
	gencertClientCmd.Flags().StringVar(&clientCACert, "ca-cert", "ca.crt", "CA証明書のパス")
	gencertClientCmd.Flags().StringVar(&clientCAKey, "ca-key", "ca.key", "CA秘密鍵のパス")
	gencertClientCmd.Flags().StringVar(&clientOutCert, "out-cert", "client.crt", "クライアント証明書の出力パス")
	gencertClientCmd.Flags().StringVar(&clientOutKey, "out-key", "client.key", "クライアント秘密鍵の出力パス")
	gencertClientCmd.Flags().IntVar(&clientDays, "days", 365, "有効期間（日数）")
	gencertClientCmd.Flags().StringVar(&clientCN, "cn", "driensten-client", "Common Name")
}
