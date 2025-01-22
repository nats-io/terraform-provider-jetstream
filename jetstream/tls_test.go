// Copyright 2025 The NATS Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package jetstream

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"strings"
	"time"
)

func newTLSConfig(caPEM, certPEM, keyPEM string) (*tls.Config, error) {
	tlsClientConfig := &tls.Config{}

	if len(certPEM) != 0 && len(keyPEM) != 0 {
		tlsCert, err := tls.X509KeyPair([]byte(certPEM), []byte(keyPEM))
		if err != nil {
			return nil, err
		}
		tlsClientConfig.Certificates = []tls.Certificate{tlsCert}
	} else if len(certPEM) != 0 || len(keyPEM) != 0 {
		return nil, fmt.Errorf("both client cert and client key must be provided")
	}

	if len(caPEM) != 0 {
		pool := x509.NewCertPool()

		ok := pool.AppendCertsFromPEM([]byte(caPEM))
		if !ok {
			return nil, errors.New("error appending CA: Couldn't parse PEM")
		}

		tlsClientConfig.RootCAs = pool
	}

	return tlsClientConfig, nil
}

func generateCerts() (caPEM, certPEM, keyPEM string, err error) {
	host := "localhost"
	validFrom := time.Now()
	validFor := time.Duration(365 * 24 * time.Hour)

	caKey, err := newPrivateKey()

	if err != nil {
		return
	}

	root, err := newCert(host, validFrom, validFor, true)
	if err != nil {
		return
	}

	caPEM, _, err = generatePEM(root, root, caKey)

	if err != nil {
		return
	}

	cert, err := newCert(host, validFrom, validFor, false)
	if err != nil {
		return
	}

	certPEM, keyPEM, err = generatePEM(root, cert, caKey)
	return
}

func generatePEM(root, cert x509.Certificate, key *ecdsa.PrivateKey) (certPEM, keyPEM string, err error) {

	derBytes, err := x509.CreateCertificate(rand.Reader, &cert, &root, &key.PublicKey, key)
	if err != nil {
		err = fmt.Errorf("failed to create certificate: %v", err)
		return
	}

	var certOut bytes.Buffer

	if err = pem.Encode(&certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		err = fmt.Errorf("failed to write data to cert.pem: %v", err)
		return
	}

	certPEM = certOut.String()

	privBytes, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		err = fmt.Errorf("unable to marshal private key: %v", err)
		return
	}

	var keyOut bytes.Buffer
	if err = pem.Encode(&keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}); err != nil {
		err = fmt.Errorf("failed to write data to key.pem: %v", err)
		return
	}

	keyPEM = keyOut.String()
	return
}

func newCert(host string, validFrom time.Time, validFor time.Duration, isCA bool) (template x509.Certificate, err error) {

	var notBefore = validFrom

	notAfter := notBefore.Add(validFor)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		err = fmt.Errorf("failed to generate serial number: %v", err)
		return
	}

	var ips []net.IP

	if !isCA {
		ips, err = getLocalIPs()

		if err != nil {
			err = fmt.Errorf("failed to get local IPs: %v", err)
			return
		}
	}

	org := "ACME Co"
	name := "localhost"

	if isCA {
		org += " Root Authority"
		name = "nats-ca"
	}

	template = x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   name,
			Organization: []string{org},
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		IPAddresses:           ips,
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}

	hosts := strings.Split(host, ",")
	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, h)
		}
	}

	if isCA {
		template.IsCA = true
		template.KeyUsage |= x509.KeyUsageCertSign
	}

	return
}

func newPrivateKey() (key *ecdsa.PrivateKey, err error) {
	key, err = ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		err = fmt.Errorf("failed to generate private key: %v", err)
		return
	}

	return
}

func getLocalIPs() (addresses []net.IP, err error) {

	ifaces, err := net.Interfaces()
	if err != nil {
		return
	}

	for _, i := range ifaces {
		addrs, e := i.Addrs()
		if e != nil {
			err = e
			return
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			addresses = append(addresses, ip)
		}
	}
	return
}
