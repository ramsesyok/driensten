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
	"encoding/pem"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var gencertCmd = &cobra.Command{
	Use:   "gencert",
	Short: "TLS動作確認用の自己署名証明書を生成します",
	Long: `CA証明書、サーバ証明書、クライアント証明書を生成します。
生成した証明書は driensten.yaml の TLS 設定に利用できます。

使用例:
  driensten gencert ca
  driensten gencert server --ca-cert ca.crt --ca-key ca.key
  driensten gencert client --ca-cert ca.crt --ca-key ca.key`,
}

func init() {
	rootCmd.AddCommand(gencertCmd)
}

func generateECKey() (*ecdsa.PrivateKey, error) {
	return ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
}

func marshalKeyPEM(key *ecdsa.PrivateKey) ([]byte, error) {
	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der}), nil
}

func marshalCertPEM(cert *x509.Certificate) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
}

func writePEMFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0o600)
}

func loadCACert(certPath, keyPath string) (*x509.Certificate, *ecdsa.PrivateKey, error) {
	certPEMData, err := os.ReadFile(certPath)
	if err != nil {
		return nil, nil, fmt.Errorf("CA証明書の読み込みに失敗: %w", err)
	}
	keyPEMData, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, nil, fmt.Errorf("CA秘密鍵の読み込みに失敗: %w", err)
	}

	block, _ := pem.Decode(certPEMData)
	if block == nil {
		return nil, nil, fmt.Errorf("CA証明書のPEMデコードに失敗")
	}
	caCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("CA証明書のパースに失敗: %w", err)
	}

	block, _ = pem.Decode(keyPEMData)
	if block == nil {
		return nil, nil, fmt.Errorf("CA秘密鍵のPEMデコードに失敗")
	}
	caKey, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("CA秘密鍵のパースに失敗: %w", err)
	}

	return caCert, caKey, nil
}
