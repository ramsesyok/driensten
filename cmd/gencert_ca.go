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
	caOutCert string
	caOutKey  string
	caDays    int
	caCN      string
)

var gencertCACmd = &cobra.Command{
	Use:   "ca",
	Short: "自己署名のCA証明書と秘密鍵を生成します",
	Long: `自己署名のルートCA証明書と秘密鍵を生成します。
生成したCA証明書はサーバ証明書・クライアント証明書の署名に使用します。

使用例:
  driensten gencert ca
  driensten gencert ca --out-cert my-ca.crt --out-key my-ca.key --days 730`,
	RunE: func(cmd *cobra.Command, args []string) error {
		key, err := generateECKey()
		if err != nil {
			return fmt.Errorf("鍵の生成に失敗: %w", err)
		}

		template := &x509.Certificate{
			SerialNumber:          big.NewInt(1),
			Subject:               pkix.Name{CommonName: caCN},
			NotBefore:             time.Now(),
			NotAfter:              time.Now().AddDate(0, 0, caDays),
			IsCA:                  true,
			KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
			BasicConstraintsValid: true,
		}

		der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
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
		if err := writePEMFile(caOutCert, marshalCertPEM(cert)); err != nil {
			return fmt.Errorf("%s の書き込みに失敗: %w", caOutCert, err)
		}
		if err := writePEMFile(caOutKey, keyPEM); err != nil {
			return fmt.Errorf("%s の書き込みに失敗: %w", caOutKey, err)
		}

		fmt.Printf("CA証明書: %s\n", caOutCert)
		fmt.Printf("CA秘密鍵: %s\n", caOutKey)
		return nil
	},
}

func init() {
	gencertCmd.AddCommand(gencertCACmd)
	gencertCACmd.Flags().StringVar(&caOutCert, "out-cert", "ca.crt", "CA証明書の出力パス")
	gencertCACmd.Flags().StringVar(&caOutKey, "out-key", "ca.key", "CA秘密鍵の出力パス")
	gencertCACmd.Flags().IntVar(&caDays, "days", 365, "有効期間（日数）")
	gencertCACmd.Flags().StringVar(&caCN, "cn", "driensten-ca", "Common Name")
}
