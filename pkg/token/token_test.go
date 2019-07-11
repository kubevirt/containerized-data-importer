/*
 * This file is part of the CDI project
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * Copyright 2019 Red Hat, Inc.
 *
 */
package token

import (
	"crypto/rand"
	"crypto/rsa"
	"reflect"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func generateTestKey() (*rsa.PrivateKey, error) {
	apiKeyPair, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	return apiKeyPair, nil
}

func TestToken(t *testing.T) {
	issuer := "issuer"

	key, err := generateTestKey()
	if err != nil {
		t.Errorf("error generating keys: %v", err)
	}

	tokenData := &Payload{
		Operation: OperationUpload,
		Name:      "fakepvc",
		Namespace: "fakenamespace",
		Resource: metav1.GroupVersionResource{
			Group:    "",
			Version:  "v1",
			Resource: "persistentvolumeclaims",
		},
	}

	g := NewGenerator(issuer, key, 5*time.Minute)

	signedToken, err := g.Generate(tokenData)

	if err != nil {
		t.Errorf("unable to generate token: %v", err)
	}

	validator := NewValidator(issuer, &key.PublicKey, 0)

	payload, err := validator.Validate(signedToken)

	if err != nil {
		t.Errorf("unable to verify token: %v", err)
	}

	if !reflect.DeepEqual(tokenData, payload) {
		t.Errorf("invalid token payload")
	}
}

func TestTokenTimeout(t *testing.T) {
	issuer := "issuer"

	key, err := generateTestKey()
	if err != nil {
		t.Errorf("error generating keys: %v", err)
	}

	tokenData := &Payload{
		Operation: OperationUpload,
		Name:      "fakepvc",
		Namespace: "fakenamespace",
		Resource: metav1.GroupVersionResource{
			Group:    "",
			Version:  "v1",
			Resource: "persistentvolumeclaims",
		},
	}

	g := NewGenerator(issuer, key, 200*time.Millisecond)

	signedToken, err := g.Generate(tokenData)

	if err != nil {
		t.Errorf("unable to generate token: %v", err)
	}

	validator := NewValidator(issuer, &key.PublicKey, 0)

	time.Sleep(time.Second)

	_, err = validator.Validate(signedToken)

	if err == nil {
		t.Errorf("token did not time out: %v", err)
	}
}

func TestWrongIssuer(t *testing.T) {
	key, err := generateTestKey()
	if err != nil {
		t.Errorf("error generating keys: %v", err)
	}

	tokenData := &Payload{
		Operation: OperationUpload,
		Name:      "fakepvc",
		Namespace: "fakenamespace",
		Resource: metav1.GroupVersionResource{
			Group:    "",
			Version:  "v1",
			Resource: "persistentvolumeclaims",
		},
	}

	g := NewGenerator("foo", key, 5*time.Minute)

	signedToken, err := g.Generate(tokenData)

	if err != nil {
		t.Errorf("unable to generate token: %v", err)
	}

	validator := NewValidator("bar", &key.PublicKey, 0)

	_, err = validator.Validate(signedToken)

	if err == nil {
		t.Errorf("bad issuer: %v", err)
	}
}

func TestBadKey(t *testing.T) {
	issuer := "issuer"

	key, err := generateTestKey()
	if err != nil {
		t.Errorf("error generating keys: %v", err)
	}

	key2, err := generateTestKey()
	if err != nil {
		t.Errorf("error generating keys: %v", err)
	}

	tokenData := &Payload{
		Operation: OperationUpload,
		Name:      "fakepvc",
		Namespace: "fakenamespace",
		Resource: metav1.GroupVersionResource{
			Group:    "",
			Version:  "v1",
			Resource: "persistentvolumeclaims",
		},
	}

	g := NewGenerator(issuer, key, 5*time.Minute)

	signedToken, err := g.Generate(tokenData)

	if err != nil {
		t.Errorf("unable to generate token: %v", err)
	}

	validator := NewValidator(issuer, &key2.PublicKey, 0)

	time.Sleep(time.Second)

	_, err = validator.Validate(signedToken)

	if err == nil {
		t.Errorf("validated with bad key: %v", err)
	}
}
