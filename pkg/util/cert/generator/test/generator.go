/*
Copyright 2026 The CDI Authors.

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

// Package generatortest provides a recording test double for generator.CertGenerator.
package generatortest

import "time"

// CertGenerator is a recording test double for generator.CertGenerator.
// When Cert and Key are both unset it returns []byte("foo") and []byte("bar").
type CertGenerator struct {
	// Cert and Key are the bytes returned by both methods.
	Cert, Key []byte
	// Err is the error returned by both methods.
	Err error

	// ClientCalls counts the MakeClientCert invocations and records the
	// arguments of the most recent one.
	ClientCalls    int
	ClientName     string
	ClientGroups   []string
	ClientDuration time.Duration

	// ServerCalls counts the MakeServerCert invocations and records the
	// arguments of the most recent one.
	ServerCalls     int
	ServerNamespace string
	ServerService   string
	ServerDuration  time.Duration
}

func (cg *CertGenerator) certKey() ([]byte, []byte) {
	if cg.Err == nil && cg.Cert == nil && cg.Key == nil {
		return []byte("foo"), []byte("bar")
	}
	return cg.Cert, cg.Key
}

// MakeClientCert records the call and returns the configured cert, key and error.
func (cg *CertGenerator) MakeClientCert(name string, groups []string, duration time.Duration) ([]byte, []byte, error) {
	cg.ClientCalls++
	cg.ClientName = name
	cg.ClientGroups = groups
	cg.ClientDuration = duration
	cert, key := cg.certKey()
	return cert, key, cg.Err
}

// MakeServerCert records the call and returns the configured cert, key and error.
func (cg *CertGenerator) MakeServerCert(namespace, service string, duration time.Duration) ([]byte, []byte, error) {
	cg.ServerCalls++
	cg.ServerNamespace = namespace
	cg.ServerService = service
	cg.ServerDuration = duration
	cert, key := cg.certKey()
	return cert, key, cg.Err
}
