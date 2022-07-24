package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"io/ioutil"
	"log"
	"math/big"
	"time"
)

const path string = "./certs/"

type CommonNames struct {
	CommonNames []string `json:"commonnames"`
}

func makeCA(subject *pkix.Name) (*x509.Certificate, *rsa.PrivateKey, error) {
	// creating a CA which will be used to sign all of our certificates using the x509 package from the Go Standard Library
	caCert := &x509.Certificate{
		SerialNumber:          big.NewInt(2022), // TODO: make it random
		Subject:               *subject,
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(1, 0, 0),
		IsCA:                  true, // <- indicating this certificate is a CA certificate.
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	// generate a private key for the CA
	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		log.Printf("Generate the CA Private Key error: %v\n", err)
		return nil, nil, err
	}

	// create the CA certificate
	caBytes, err := x509.CreateCertificate(rand.Reader, caCert, caCert, &caKey.PublicKey, caKey)
	if err != nil {
		log.Println("Create the CA Certificate error: ", err)
		return nil, nil, err
	}

	// Create the CA PEM files
	caPEM := new(bytes.Buffer)
	pem.Encode(caPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caBytes,
	})

	if err := ioutil.WriteFile(path+"ca.crt", caPEM.Bytes(), 0644); err != nil {
		log.Println("Write the CA certificate file error: ", err)
		return nil, nil, err
	}

	caPrivKeyPEM := new(bytes.Buffer)
	pem.Encode(caPrivKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(caKey),
	})
	if err := ioutil.WriteFile(path+"ca.key", caPEM.Bytes(), 0644); err != nil {
		log.Println("Write the CA certificate file error: ", err)
		return nil, nil, err
	}
	return caCert, caKey, nil
}

func makeCert(caCert *x509.Certificate, caKey *rsa.PrivateKey, subject *pkix.Name, name string) error {

	cert := &x509.Certificate{
		SerialNumber: big.NewInt(2001), // TODO: make it random
		Subject:      *subject,
		DNSNames:     []string{"localhost"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(0, 0, 100),
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}

	certKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		log.Println("Generate the Key error: ", err)
		return err
	}
	certBytes, err := x509.CreateCertificate(rand.Reader, cert, caCert, &certKey.PublicKey, caKey)
	if err != nil {
		log.Println("Generate the certificate error ", err)
		return err
	}

	certPEM := new(bytes.Buffer)
	pem.Encode(certPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	})
	if err := ioutil.WriteFile(path+name+".crt", certPEM.Bytes(), 0644); err != nil {
		log.Println("Write the CA certificate file error: ", err)
		return err
	}

	certKeyPEM := new(bytes.Buffer)
	pem.Encode(certKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(certKey),
	})
	if err := ioutil.WriteFile(path+name+".key", certKeyPEM.Bytes(), 0644); err != nil {
		log.Println("Write the CA certificate file error: ", err)
		return err
	}
	return nil
}

func main() {
	subject := pkix.Name{
		CommonName: "CA",
	}
	caCert, caKey, err := makeCA(&subject)
	if err != nil {
		log.Fatalf("make CA Certificate error!")
	}

	subject.CommonName = "server"
	if err := makeCert(caCert, caKey, &subject, "server"); err != nil {
		log.Fatal("make server certificate error!")
	}
	log.Println("Create and Sign the certificate successfully for server.")

	file, err := ioutil.ReadFile(path + "client_certs.json")
	if err != nil {
		log.Fatalf("error reading client_certs.json! %v", err)
	}

	data := CommonNames{}
	_ = json.Unmarshal([]byte(file), &data)
	for _, cn := range data.CommonNames {
		subject.CommonName = cn
		if err := makeCert(caCert, caKey, &subject, cn); err != nil {
			log.Fatal("make Certificate error!", cn)
		}
		log.Println("Create and Sign the certificate successfully for ", cn)
	}
}
