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
	"software.sslmate.com/src/go-pkcs12"
)

// setupTestCA creates a self-signed CA cert and key in dir, returns their paths.
func setupTestCA(t *testing.T, dir string) (certPath, keyPath string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-ca"},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)

	certPath = filepath.Join(dir, "ca.crt")
	keyPath = filepath.Join(dir, "ca.key")

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	require.NoError(t, os.WriteFile(certPath, certPEM, 0o600))

	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	require.NoError(t, os.WriteFile(keyPath, keyPEM, 0o600))

	return certPath, keyPath
}

func TestGencertClientPEM(t *testing.T) {
	dir := t.TempDir()
	caCert, caKey := setupTestCA(t, dir)

	outCert := filepath.Join(dir, "client.crt")
	outKey := filepath.Join(dir, "client.key")

	clientCACert = caCert
	clientCAKey = caKey
	clientOutCert = outCert
	clientOutKey = outKey
	clientOutP12 = filepath.Join(dir, "client.p12")
	clientDays = 1
	clientCN = "test-client"
	clientFormat = "pem"
	clientPKCS12Password = ""

	err := gencertClientCmd.RunE(gencertClientCmd, nil)
	require.NoError(t, err)

	certData, err := os.ReadFile(outCert)
	require.NoError(t, err)
	block, _ := pem.Decode(certData)
	require.NotNil(t, block)
	cert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)
	assert.Equal(t, "test-client", cert.Subject.CommonName)

	keyData, err := os.ReadFile(outKey)
	require.NoError(t, err)
	keyBlock, _ := pem.Decode(keyData)
	require.NotNil(t, keyBlock)
	_, err = x509.ParseECPrivateKey(keyBlock.Bytes)
	require.NoError(t, err)
}

func TestGencertClientPKCS12(t *testing.T) {
	dir := t.TempDir()
	caCert, caKey := setupTestCA(t, dir)

	outP12 := filepath.Join(dir, "client.p12")
	password := "testpassword"

	clientCACert = caCert
	clientCAKey = caKey
	clientOutCert = filepath.Join(dir, "client.crt")
	clientOutKey = filepath.Join(dir, "client.key")
	clientOutP12 = outP12
	clientDays = 1
	clientCN = "test-client-p12"
	clientFormat = "pkcs12"
	clientPKCS12Password = password

	err := gencertClientCmd.RunE(gencertClientCmd, nil)
	require.NoError(t, err)

	p12Data, err := os.ReadFile(outP12)
	require.NoError(t, err)

	key, cert, _, err := pkcs12.DecodeChain(p12Data, password)
	require.NoError(t, err)
	assert.Equal(t, "test-client-p12", cert.Subject.CommonName)
	_, ok := key.(*ecdsa.PrivateKey)
	assert.True(t, ok)
}

func TestGencertClientPKCS12EmptyPassword(t *testing.T) {
	dir := t.TempDir()
	caCert, caKey := setupTestCA(t, dir)

	outP12 := filepath.Join(dir, "client.p12")

	clientCACert = caCert
	clientCAKey = caKey
	clientOutCert = filepath.Join(dir, "client.crt")
	clientOutKey = filepath.Join(dir, "client.key")
	clientOutP12 = outP12
	clientDays = 1
	clientCN = "test-client-nopass"
	clientFormat = "pkcs12"
	clientPKCS12Password = ""

	err := gencertClientCmd.RunE(gencertClientCmd, nil)
	require.NoError(t, err)

	p12Data, err := os.ReadFile(outP12)
	require.NoError(t, err)

	_, cert, _, err := pkcs12.DecodeChain(p12Data, "")
	require.NoError(t, err)
	assert.Equal(t, "test-client-nopass", cert.Subject.CommonName)
}

func TestGencertClientInvalidFormat(t *testing.T) {
	dir := t.TempDir()
	caCert, caKey := setupTestCA(t, dir)

	clientCACert = caCert
	clientCAKey = caKey
	clientFormat = "invalid"

	err := gencertClientCmd.RunE(gencertClientCmd, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "不正なフォーマット")
}
