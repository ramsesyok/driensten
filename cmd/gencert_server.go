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
	"net"
	"time"

	"github.com/spf13/cobra"
)

var (
	serverCACert  string
	serverCAKey   string
	serverOutCert string
	serverOutKey  string
	serverDays    int
	serverCN      string
	serverHosts   []string
)

var gencertServerCmd = &cobra.Command{
	Use:   "server",
	Short: "CAで署名したサーバ証明書と秘密鍵を生成します",
	Long: `指定したCAで署名したサーバ証明書と秘密鍵を生成します。
driensten.yaml の HTTP.tls または MQTT.tls に設定して使用します。

SAN（Subject Alternative Name）にはデフォルトで localhost と 127.0.0.1 が含まれます。
--hosts フラグで追加のホスト名や IP アドレスを指定できます。

使用例:
  driensten gencert server
  driensten gencert server --hosts localhost,127.0.0.1,192.168.1.10`,
	RunE: func(cmd *cobra.Command, args []string) error {
		caCert, caKey, err := loadCACert(serverCACert, serverCAKey)
		if err != nil {
			return err
		}

		key, err := generateECKey()
		if err != nil {
			return fmt.Errorf("鍵の生成に失敗: %w", err)
		}

		template := &x509.Certificate{
			SerialNumber: big.NewInt(time.Now().UnixNano()),
			Subject:      pkix.Name{CommonName: serverCN},
			NotBefore:    time.Now(),
			NotAfter:     time.Now().AddDate(0, 0, serverDays),
			KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
			ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		}
		for _, h := range serverHosts {
			if ip := net.ParseIP(h); ip != nil {
				template.IPAddresses = append(template.IPAddresses, ip)
			} else {
				template.DNSNames = append(template.DNSNames, h)
			}
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
		if err := writePEMFile(serverOutCert, marshalCertPEM(cert)); err != nil {
			return fmt.Errorf("%s の書き込みに失敗: %w", serverOutCert, err)
		}
		if err := writePEMFile(serverOutKey, keyPEM); err != nil {
			return fmt.Errorf("%s の書き込みに失敗: %w", serverOutKey, err)
		}

		fmt.Printf("サーバ証明書: %s\n", serverOutCert)
		fmt.Printf("サーバ秘密鍵: %s\n", serverOutKey)
		fmt.Printf("SAN: %v\n", serverHosts)
		return nil
	},
}

func init() {
	gencertCmd.AddCommand(gencertServerCmd)
	gencertServerCmd.Flags().StringVar(&serverCACert, "ca-cert", "ca.crt", "CA証明書のパス")
	gencertServerCmd.Flags().StringVar(&serverCAKey, "ca-key", "ca.key", "CA秘密鍵のパス")
	gencertServerCmd.Flags().StringVar(&serverOutCert, "out-cert", "server.crt", "サーバ証明書の出力パス")
	gencertServerCmd.Flags().StringVar(&serverOutKey, "out-key", "server.key", "サーバ秘密鍵の出力パス")
	gencertServerCmd.Flags().IntVar(&serverDays, "days", 365, "有効期間（日数）")
	gencertServerCmd.Flags().StringVar(&serverCN, "cn", "driensten-server", "Common Name")
	gencertServerCmd.Flags().StringSliceVar(&serverHosts, "hosts", []string{"localhost", "127.0.0.1"}, "SANに含めるホスト名またはIPアドレス（カンマ区切り）")
}
